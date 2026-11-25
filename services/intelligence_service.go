package services

import (
	"fmt"
	"math"
	"time"

	"expense-matrix/db"
	"expense-matrix/models"
)

type IntelligenceService struct {
	db *db.DB
}

func NewIntelligenceService(database *db.DB) *IntelligenceService {
	return &IntelligenceService{db: database}
}

// ComputeFinancialHealth calculates score (0-100), rating, savings rate, burn-rate forecast, and actionable advice.
func (s *IntelligenceService) ComputeFinancialHealth(userID int64) (*models.FinancialHealth, error) {
	today := time.Now()
	startOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	daysInMonth := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	dayOfMonth := today.Day()
	if dayOfMonth < 1 {
		dayOfMonth = 1
	}
	daysRemaining := daysInMonth - dayOfMonth + 1
	if daysRemaining < 1 {
		daysRemaining = 1
	}

	var monthlyIncome, monthlySpend float64

	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(t.amount), 0)
		FROM transactions t
		JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ? AND c.type = 'income' AND t.date >= ?`, userID, startOfMonth).Scan(&monthlyIncome)

	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(t.amount), 0)
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ? AND COALESCE(c.type, 'expense') = 'expense' AND t.date >= ?`, userID, startOfMonth).Scan(&monthlySpend)

	var budgetGoal float64
	err := s.db.QueryRow(`SELECT monthly_budget_goal FROM users WHERE id = ?`, userID).Scan(&budgetGoal)
	if err != nil || budgetGoal <= 0 {
		budgetGoal = 75000.0
	}

	var subSpend float64
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(
			CASE
				WHEN billing_cycle = 'yearly' THEN amount / 12.0
				WHEN billing_cycle = 'weekly' THEN amount * 4.333
				ELSE amount
			END
		), 0)
		FROM subscriptions
		WHERE user_id = ? AND status = 'active'`, userID).Scan(&subSpend)

	var anomalyCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE user_id = ? AND is_anomaly = 1 AND date >= ?`, userID, startOfMonth).Scan(&anomalyCount)

	netSavings := monthlyIncome - monthlySpend
	savingsRate := 0.0
	if monthlyIncome > 0 {
		savingsRate = (netSavings / monthlyIncome) * 100.0
	}

	subWeight := 0.0
	if monthlySpend > 0 {
		subWeight = (subSpend / monthlySpend) * 100.0
	}

	budgetUtil := (monthlySpend / budgetGoal) * 100.0
	dailyBurnRate := monthlySpend / float64(dayOfMonth)
	projectedSpend := dailyBurnRate * float64(daysInMonth)
	safeDailyAllowance := math.Max(0, (budgetGoal-monthlySpend)/float64(daysRemaining))

	score := 100
	if savingsRate < 10 {
		score -= 25
	} else if savingsRate < 20 {
		score -= 15
	} else if savingsRate < 30 {
		score -= 5
	}

	if budgetUtil > 100 {
		score -= 20
	} else if budgetUtil > 85 {
		score -= 10
	}

	score -= (anomalyCount * 5)

	if subWeight > 15 {
		score -= 10
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	rating := "Optimal"
	if score < 60 {
		rating = "Critical"
	} else if score < 75 {
		rating = "Caution"
	} else if score < 90 {
		rating = "Healthy"
	}

	var topCategory string
	_ = s.db.QueryRow(`
		SELECT COALESCE(c.name, 'Miscellaneous')
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ? AND COALESCE(c.type, 'expense') = 'expense' AND t.date >= ?
		GROUP BY COALESCE(c.name, 'Miscellaneous')
		ORDER BY SUM(t.amount) DESC LIMIT 1`, userID, startOfMonth).Scan(&topCategory)

	if topCategory == "" {
		topCategory = "None yet"
	}

	recommendations := []string{}
	if savingsRate < 20 {
		recommendations = append(recommendations, fmt.Sprintf("Boost net savings rate (currently %.1f%%) with an automated monthly SIP sweep on payday.", savingsRate))
	} else {
		recommendations = append(recommendations, fmt.Sprintf("Strong savings rate of %.1f%%! Channel surplus capital into automated index fund SIPs or savings goals.", savingsRate))
	}

	if anomalyCount > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d flagged anomaly alert(s) in Transactions to catch duplicate or surge charges.", anomalyCount))
	}

	if projectedSpend > budgetGoal {
		recommendations = append(recommendations, fmt.Sprintf("At current burn pace, projected month-end spend exceeds your target. Keep discretionary spend under %.0f/day.", safeDailyAllowance))
	} else {
		recommendations = append(recommendations, fmt.Sprintf("Your spend velocity is well-paced with %d day(s) remaining in this cycle.", daysRemaining))
	}

	return &models.FinancialHealth{
		Score:              score,
		Rating:             rating,
		MonthlyIncome:      monthlyIncome,
		MonthlySpend:       monthlySpend,
		NetSavings:         netSavings,
		SavingsRate:        math.Round(savingsRate*10) / 10,
		SubscriptionWeight: math.Round(subWeight*10) / 10,
		BudgetUtilization:  math.Round(budgetUtil*10) / 10,
		DailyBurnRate:      dailyBurnRate,
		ProjectedSpend:     projectedSpend,
		SafeDailyAllowance: safeDailyAllowance,
		DaysRemaining:      daysRemaining,
		TopCategory:        topCategory,
		AnomalyCount:       anomalyCount,
		Recommendations:    recommendations,
	}, nil
}

// FetchDailyHeatmap builds a 30-day GitHub-style daily expenditure intensity grid.
func (s *IntelligenceService) FetchDailyHeatmap(userID int64, days int) []models.DailyHeatmapCell {
	if days <= 0 {
		days = 30
	}
	today := time.Now()
	startDate := today.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	totals := make(map[string]float64)
	var maxVal float64

	if rows, err := s.db.Query(`
		SELECT t.date, COALESCE(SUM(t.amount), 0)
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ? AND COALESCE(c.type, 'expense') = 'expense' AND t.date >= ?
		GROUP BY t.date`, userID, startDate); err == nil {
		defer rows.Close()
		for rows.Next() {
			var dt string
			var amt float64
			if err := rows.Scan(&dt, &amt); err == nil {
				totals[dt] = amt
				if amt > maxVal {
					maxVal = amt
				}
			}
		}
	}

	cells := make([]models.DailyHeatmapCell, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		iso := d.Format("2006-01-02")
		amt := totals[iso]
		level := 0
		if amt > 0 && maxVal > 0 {
			ratio := amt / maxVal
			switch {
			case ratio >= 0.75:
				level = 4
			case ratio >= 0.45:
				level = 3
			case ratio >= 0.18:
				level = 2
			default:
				level = 1
			}
		}
		cells = append(cells, models.DailyHeatmapCell{
			Date:   iso,
			Label:  d.Format("02 Jan"),
			Amount: amt,
			Level:  level,
		})
	}
	return cells
}

// FetchCategoryBreakdown computes expense distributions per category (base INR).
func (s *IntelligenceService) FetchCategoryBreakdown(userID int64) ([]models.CategorySpend, error) {
	rows, err := s.db.Query(`
		SELECT COALESCE(c.name, 'Miscellaneous') as name,
		       COALESCE(c.icon, '📦') as icon,
		       COALESCE(c.color, '#64748b') as color,
		       COALESCE(SUM(t.amount), 0) as total
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = ? AND COALESCE(c.type, 'expense') = 'expense'
		GROUP BY COALESCE(c.name, 'Miscellaneous'), COALESCE(c.icon, '📦'), COALESCE(c.color, '#64748b')
		HAVING total > 0
		ORDER BY total DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]models.CategorySpend, 0)
	var grandTotal float64

	for rows.Next() {
		var item models.CategorySpend
		if err := rows.Scan(&item.Name, &item.Icon, &item.Color, &item.Amount); err == nil {
			grandTotal += item.Amount
			list = append(list, item)
		}
	}

	if grandTotal > 0 {
		for i := range list {
			list[i].Percentage = math.Round((list[i].Amount/grandTotal)*1000) / 10
		}
	}

	return list, nil
}

// FetchMonthlyTrends retrieves 6-month historical trajectory data (base INR).
func (s *IntelligenceService) FetchMonthlyTrends(userID int64) ([]models.MonthlyTrend, error) {
	trends := make([]models.MonthlyTrend, 0, 6)
	now := time.Now()
	firstOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var budgetGoal float64
	if err := s.db.QueryRow(`SELECT monthly_budget_goal FROM users WHERE id = ?`, userID).Scan(&budgetGoal); err != nil || budgetGoal <= 0 {
		budgetGoal = 75000.0
	}

	for i := 5; i >= 0; i-- {
		monthDate := firstOfCurrentMonth.AddDate(0, -i, 0)
		monthStr := monthDate.Format("2006-01")
		monthLabel := monthDate.Format("Jan 06")

		var inc, exp float64
		_ = s.db.QueryRow(`
			SELECT COALESCE(SUM(t.amount), 0)
			FROM transactions t
			JOIN categories c ON t.category_id = c.id
			WHERE t.user_id = ? AND c.type = 'income' AND strftime('%Y-%m', t.date) = ?`, userID, monthStr).Scan(&inc)

		_ = s.db.QueryRow(`
			SELECT COALESCE(SUM(t.amount), 0)
			FROM transactions t
			LEFT JOIN categories c ON t.category_id = c.id
			WHERE t.user_id = ? AND COALESCE(c.type, 'expense') = 'expense' AND strftime('%Y-%m', t.date) = ?`, userID, monthStr).Scan(&exp)

		trends = append(trends, models.MonthlyTrend{
			Month:      monthLabel,
			Income:     inc,
			Expense:    exp,
			Savings:    inc - exp,
			BudgetPace: budgetGoal,
		})
	}

	return trends, nil
}
