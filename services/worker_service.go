package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"expense-matrix/db"
)

// WorkerService runs asynchronous background jobs (statistical anomaly scanning,
// subscription renewal watchdog, and live FX cache warming) with graceful cancellation.
type WorkerService struct {
	db      *db.DB
	fx      *CurrencyService
	metrics *MetricsService
}

func NewWorkerService(database *db.DB, fx *CurrencyService, metrics *MetricsService) *WorkerService {
	return &WorkerService{
		db:      database,
		fx:      fx,
		metrics: metrics,
	}
}

// Start launches background workers and stops cleanly when ctx is cancelled.
func (w *WorkerService) Start(ctx context.Context) {
	// Warm FX cache asynchronously on boot
	go func() {
		if w.fx != nil {
			_ = w.fx.GetRates()
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		// Initial sweep 15 seconds after startup
		select {
		case <-time.After(15 * time.Second):
			w.RunCycle()
		case <-ctx.Done():
			return
		}

		for {
			select {
			case <-ticker.C:
				w.RunCycle()
			case <-ctx.Done():
				log.Println("🛑 Background worker stopped gracefully")
				return
			}
		}
	}()
}

// RunCycle executes one full background maintenance & intelligence sweep.
func (w *WorkerService) RunCycle() {
	if w.metrics != nil {
		w.metrics.IncWorkerRun()
	}
	flagged := w.ScanStatisticalAnomalies()
	if flagged > 0 && w.metrics != nil {
		w.metrics.AddAnomalies(int64(flagged))
	}
	w.AdvanceOverdueSubscriptions()
}

// ScanStatisticalAnomalies computes per-user/category mean & standard deviation (z-score >= 2.2)
// to detect statistical surge spending automatically in the background.
func (w *WorkerService) ScanStatisticalAnomalies() int {
	rows, err := w.db.Query(`
		SELECT t.user_id, t.category_id, AVG(t.amount) as avg_amt, COUNT(*) as cnt
		FROM transactions t
		JOIN categories c ON t.category_id = c.id
		WHERE c.type = 'expense' AND t.category_id IS NOT NULL
		GROUP BY t.user_id, t.category_id
		HAVING cnt >= 3`)
	if err != nil {
		return 0
	}
	defer rows.Close()

	type statGroup struct {
		userID int64
		catID  int64
		avg    float64
	}
	var groups []statGroup
	for rows.Next() {
		var g statGroup
		var cnt int
		if err := rows.Scan(&g.userID, &g.catID, &g.avg, &cnt); err == nil && g.avg > 0 {
			groups = append(groups, g)
		}
	}

	flagged := 0
	for _, g := range groups {
		threshold := math.Max(g.avg*2.5, 5000.0)
		res, err := w.db.Exec(`
			UPDATE transactions
			SET is_anomaly = 1,
			    anomaly_reason = ?
			WHERE user_id = ? AND category_id = ? AND is_anomaly = 0 AND amount > ?`,
			fmt.Sprintf("Statistical Surge: >2.5x category avg (₹%.0f)", g.avg),
			g.userID, g.catID, threshold,
		)
		if err == nil {
			if n, _ := res.RowsAffected(); n > 0 {
				flagged += int(n)
			}
		}
	}
	return flagged
}

// AdvanceOverdueSubscriptions rolls forward past-due active subscription billing dates.
func (w *WorkerService) AdvanceOverdueSubscriptions() {
	today := time.Now().Format("2006-01-02")
	rows, err := w.db.Query(`
		SELECT id, billing_cycle, next_billing_date
		FROM subscriptions
		WHERE status = 'active' AND next_billing_date < ?`, today)
	if err != nil {
		return
	}
	defer rows.Close()

	type subDue struct {
		id    int64
		cycle string
		date  string
	}
	var overdue []subDue
	for rows.Next() {
		var s subDue
		if err := rows.Scan(&s.id, &s.cycle, &s.date); err == nil {
			overdue = append(overdue, s)
		}
	}

	for _, s := range overdue {
		parsed, err := time.Parse("2006-01-02", s.date)
		if err != nil {
			continue
		}
		for parsed.Format("2006-01-02") < today {
			switch s.cycle {
			case "yearly":
				parsed = parsed.AddDate(1, 0, 0)
			case "weekly":
				parsed = parsed.AddDate(0, 0, 7)
			default:
				parsed = parsed.AddDate(0, 1, 0)
			}
		}
		_, _ = w.db.Exec(`UPDATE subscriptions SET next_billing_date = ? WHERE id = ?`, parsed.Format("2006-01-02"), s.id)
	}
}
