package handlers

import (
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
)

type GoalsHandler struct {
	DB        *db.DB
	Templates *template.Template
}

func NewGoalsHandler(database *db.DB, tmpl *template.Template) *GoalsHandler {
	return &GoalsHandler{DB: database, Templates: tmpl}
}

func (h *GoalsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, userID, username)

	today := time.Now()
	startOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	// Category Budget Envelopes (MTD spend vs monthly_limit)
	rows, err := h.DB.Query(`
		SELECT c.id, c.name, c.icon, c.color, c.type, c.monthly_limit,
		       COALESCE((SELECT SUM(t.amount) FROM transactions t WHERE t.category_id = c.id AND t.user_id = ? AND t.date >= ?), 0) as spent
		FROM categories c
		WHERE c.user_id = ? AND c.type = 'expense'
		ORDER BY spent DESC, c.name ASC`, userID, startOfMonth, userID)

	var budgets []models.CategoryBudgetStatus
	var totalBudgeted, totalBudgetSpent float64
	var overBudgetCount int

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var b models.CategoryBudgetStatus
			if err := rows.Scan(&b.ID, &b.Name, &b.Icon, &b.Color, &b.Type, &b.MonthlyLimit, &b.Spent); err == nil {
				totalBudgeted += b.MonthlyLimit
				totalBudgetSpent += b.Spent
				b.Remaining = b.MonthlyLimit - b.Spent
				if b.MonthlyLimit > 0 {
					b.Utilization = math.Round((b.Spent/b.MonthlyLimit)*1000) / 10
					if b.Utilization >= 100 {
						b.Status = "exceeded"
						overBudgetCount++
					} else if b.Utilization >= 75 {
						b.Status = "warning"
					} else {
						b.Status = "ok"
					}
				} else {
					b.Status = "unset"
				}
				budgets = append(budgets, b)
			}
		}
	}

	// Savings Goals
	gRows, err := h.DB.Query(`
		SELECT id, name, icon, target_amount, current_amount, target_date, category, status
		FROM savings_goals
		WHERE user_id = ?
		ORDER BY CASE WHEN status = 'active' THEN 0 ELSE 1 END, target_date ASC`, userID)

	var goals []models.SavingsGoal
	var totalGoalTarget, totalGoalSaved float64

	if err == nil {
		defer gRows.Close()
		for gRows.Next() {
			var g models.SavingsGoal
			if err := gRows.Scan(&g.ID, &g.Name, &g.Icon, &g.TargetAmount, &g.CurrentAmount, &g.TargetDate, &g.Category, &g.Status); err == nil {
				totalGoalTarget += g.TargetAmount
				totalGoalSaved += g.CurrentAmount
				if g.TargetAmount > 0 {
					g.ProgressPct = math.Round((g.CurrentAmount/g.TargetAmount)*1000) / 10
				}
				g.Remaining = math.Max(0, g.TargetAmount-g.CurrentAmount)
				goals = append(goals, g)
			}
		}
	}

	overallGoalPct := 0.0
	if totalGoalTarget > 0 {
		overallGoalPct = math.Round((totalGoalSaved/totalGoalTarget)*1000) / 10
	}

	viewCtx["Title"] = "Budgets & Savings Goals - PulseMatrix AI"
	viewCtx["ActivePage"] = "goals"
	viewCtx["Budgets"] = budgets
	viewCtx["TotalBudgeted"] = totalBudgeted
	viewCtx["TotalBudgetSpent"] = totalBudgetSpent
	viewCtx["OverBudgetCount"] = overBudgetCount
	viewCtx["Goals"] = goals
	viewCtx["TotalGoalTarget"] = totalGoalTarget
	viewCtx["TotalGoalSaved"] = totalGoalSaved
	viewCtx["OverallGoalPct"] = overallGoalPct
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *GoalsHandler) AddGoal(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	name := strings.TrimSpace(r.FormValue("name"))
	icon := strings.TrimSpace(r.FormValue("icon"))
	category := strings.TrimSpace(r.FormValue("category"))
	targetDate := strings.TrimSpace(r.FormValue("target_date"))
	targetAmt, err1 := strconv.ParseFloat(strings.TrimSpace(r.FormValue("target_amount")), 64)
	currentAmt, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("current_amount")), 64)

	if name == "" || err1 != nil || targetAmt <= 0 {
		http.Redirect(w, r, "/goals?error=Provide+a+valid+goal+name+and+positive+target+amount", http.StatusSeeOther)
		return
	}
	if currentAmt < 0 {
		currentAmt = 0
	}
	if icon == "" {
		icon = "🎯"
	}
	if category == "" {
		category = "Savings"
	}
	if targetDate == "" {
		targetDate = time.Now().AddDate(0, 6, 0).Format("2006-01-02")
	}

	status := "active"
	if currentAmt >= targetAmt {
		status = "completed"
	}

	_, err := h.DB.Exec(`
		INSERT INTO savings_goals (user_id, name, icon, target_amount, current_amount, target_date, category, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, name, icon, targetAmt, currentAmt, targetDate, category, status,
	)
	if err != nil {
		http.Redirect(w, r, "/goals?error=Failed+to+create+savings+goal", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/goals?success=Savings+goal+created", http.StatusSeeOther)
}

func (h *GoalsHandler) ContributeGoal(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	goalID, err1 := strconv.ParseInt(r.FormValue("id"), 10, 64)
	addAmt, err2 := strconv.ParseFloat(strings.TrimSpace(r.FormValue("amount")), 64)

	if err1 != nil || err2 != nil || goalID <= 0 || addAmt <= 0 {
		http.Redirect(w, r, "/goals?error=Enter+a+positive+contribution+amount", http.StatusSeeOther)
		return
	}

	_, _ = h.DB.Exec(`
		UPDATE savings_goals
		SET current_amount = current_amount + ?,
		    status = CASE WHEN (current_amount + ?) >= target_amount THEN 'completed' ELSE 'active' END
		WHERE id = ? AND user_id = ?`,
		addAmt, addAmt, goalID, userID,
	)

	http.Redirect(w, r, "/goals?success=Contribution+added+to+savings+goal", http.StatusSeeOther)
}

func (h *GoalsHandler) DeleteGoal(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	goalID := r.FormValue("id")
	_, _ = h.DB.Exec(`DELETE FROM savings_goals WHERE id = ? AND user_id = ?`, goalID, userID)
	http.Redirect(w, r, "/goals?success=Savings+goal+removed", http.StatusSeeOther)
}

func (h *GoalsHandler) UpdateCategoryBudget(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	catID, err1 := strconv.ParseInt(r.FormValue("id"), 10, 64)
	limitVal, err2 := strconv.ParseFloat(strings.TrimSpace(r.FormValue("monthly_limit")), 64)

	if err1 != nil || err2 != nil || catID <= 0 || limitVal < 0 {
		http.Redirect(w, r, "/goals?error=Invalid+budget+limit", http.StatusSeeOther)
		return
	}

	_, _ = h.DB.Exec(`UPDATE categories SET monthly_limit = ? WHERE id = ? AND user_id = ?`, limitVal, catID, userID)
	http.Redirect(w, r, "/goals?success=Category+monthly+budget+updated", http.StatusSeeOther)
}

func (h *GoalsHandler) AddCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	name := strings.TrimSpace(r.FormValue("name"))
	icon := strings.TrimSpace(r.FormValue("icon"))
	color := strings.TrimSpace(r.FormValue("color"))
	catType := strings.TrimSpace(r.FormValue("type"))
	limitVal, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("monthly_limit")), 64)

	if name == "" {
		http.Redirect(w, r, "/goals?error=Category+name+is+required", http.StatusSeeOther)
		return
	}
	if icon == "" {
		icon = "🏷️"
	}
	if color == "" || !strings.HasPrefix(color, "#") {
		color = "#6366f1"
	}
	if catType != "expense" && catType != "income" && catType != "investment" {
		catType = "expense"
	}
	if limitVal < 0 {
		limitVal = 0
	}

	_, err := h.DB.Exec(`
		INSERT INTO categories (user_id, name, icon, color, type, monthly_limit)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, name, icon, color, catType, limitVal,
	)
	if err != nil {
		http.Redirect(w, r, "/goals?error=Category+name+already+exists", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/goals?success=Custom+category+added", http.StatusSeeOther)
}
