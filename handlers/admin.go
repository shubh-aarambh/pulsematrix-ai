package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"

	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	DB              *db.DB
	Templates       *template.Template
	CurrencyService *services.CurrencyService
}

func NewAdminHandler(database *db.DB, tmpl *template.Template, cs *services.CurrencyService) *AdminHandler {
	return &AdminHandler{DB: database, Templates: tmpl, CurrencyService: cs}
}

func (h *AdminHandler) Show(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value(middleware.UserIDKey).(int64)
	adminName, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, adminID, adminName)

	var stats models.PlatformStats
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE status = 'active'`).Scan(&stats.ActiveUsers)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE status != 'active'`).Scan(&stats.SuspendedUsers)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&stats.AdminCount)
	_ = h.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM transactions`).Scan(&stats.TotalTransactions, &stats.TotalVolumeUSD)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM transactions WHERE is_anomaly = 1`).Scan(&stats.TotalAnomalies)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE status = 'active'`).Scan(&stats.ActiveSubs)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM savings_goals`).Scan(&stats.TotalGoals)

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	roleFilter := strings.TrimSpace(r.URL.Query().Get("role"))

	userQuery := `
		SELECT u.id, u.username, u.email, u.role, u.status, u.preferred_currency, u.monthly_budget_goal,
		       COALESCE(u.last_login_at, ''), strftime('%Y-%m-%d', u.created_at),
		       (SELECT COUNT(*) FROM transactions t WHERE t.user_id = u.id) as tx_count,
		       (SELECT COALESCE(SUM(t.amount), 0) FROM transactions t LEFT JOIN categories c ON t.category_id = c.id WHERE t.user_id = u.id AND COALESCE(c.type, 'expense') = 'expense') as total_spend,
		       (SELECT COUNT(*) FROM subscriptions s WHERE s.user_id = u.id AND s.status = 'active') as sub_count
		FROM users u
		WHERE 1=1`

	args := []interface{}{}
	if q != "" {
		userQuery += ` AND (u.username LIKE ? OR u.email LIKE ?)`
		pat := "%" + q + "%"
		args = append(args, pat, pat)
	}
	if roleFilter == "admin" || roleFilter == "user" {
		userQuery += ` AND u.role = ?`
		args = append(args, roleFilter)
	}
	userQuery += ` ORDER BY CASE WHEN u.role = 'admin' THEN 0 ELSE 1 END, u.id DESC`

	var users []models.AdminUserSummary
	if rows, err := h.DB.Query(userQuery, args...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var u models.AdminUserSummary
			if err := rows.Scan(
				&u.ID, &u.Username, &u.Email, &u.Role, &u.Status, &u.PreferredCurrency, &u.MonthlyBudgetGoal,
				&u.LastLoginAt, &u.CreatedAt, &u.TxCount, &u.TotalSpend, &u.SubCount,
			); err == nil {
				users = append(users, u)
			}
		}
	}

	type PlatformAnomaly struct {
		ID       int64
		Username string
		Vendor   string
		Amount   float64
		Date     string
		Reason   string
		Source   string
	}
	var anomalies []PlatformAnomaly
	if aRows, err := h.DB.Query(`
		SELECT t.id, u.username, t.vendor, t.amount, t.date, t.anomaly_reason, t.source
		FROM transactions t
		JOIN users u ON t.user_id = u.id
		WHERE t.is_anomaly = 1
		ORDER BY t.date DESC, t.id DESC LIMIT 10`); err == nil {
		defer aRows.Close()
		for aRows.Next() {
			var a PlatformAnomaly
			if err := aRows.Scan(&a.ID, &a.Username, &a.Vendor, &a.Amount, &a.Date, &a.Reason, &a.Source); err == nil {
				anomalies = append(anomalies, a)
			}
		}
	}

	type AdminAuditLog struct {
		ID        int64
		Username  string
		Action    string
		Details   string
		CreatedAt string
	}
	var auditLogs []AdminAuditLog
	if lRows, err := h.DB.Query(`
		SELECT a.id, COALESCE(u.username, 'system'), a.action, a.details, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON a.user_id = u.id
		ORDER BY a.created_at DESC, a.id DESC LIMIT 20`); err == nil {
		defer lRows.Close()
		for lRows.Next() {
			var l AdminAuditLog
			if err := lRows.Scan(&l.ID, &l.Username, &l.Action, &l.Details, &l.CreatedAt); err == nil {
				auditLogs = append(auditLogs, l)
			}
		}
	}

	viewCtx["Title"] = "Admin Control Plane"
	viewCtx["ActivePage"] = "admin"
	viewCtx["AdminID"] = adminID
	viewCtx["Stats"] = stats
	viewCtx["Users"] = users
	viewCtx["Search"] = q
	viewCtx["RoleFilter"] = roleFilter
	viewCtx["Anomalies"] = anomalies
	viewCtx["AuditLogs"] = auditLogs
	viewCtx["Settings"] = h.DB.GetSystemSettings()
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *AdminHandler) ToggleRole(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value(middleware.UserIDKey).(int64)
	targetID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil || targetID <= 0 {
		http.Redirect(w, r, "/admin?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}
	if targetID == adminID {
		http.Redirect(w, r, "/admin?error=You+cannot+change+your+own+admin+role", http.StatusSeeOther)
		return
	}

	var curRole, uname string
	if err := h.DB.QueryRow(`SELECT role, username FROM users WHERE id = ?`, targetID).Scan(&curRole, &uname); err != nil {
		http.Redirect(w, r, "/admin?error=User+not+found", http.StatusSeeOther)
		return
	}

	newRole := "admin"
	if curRole == "admin" {
		newRole = "user"
	}

	_, _ = h.DB.Exec(`UPDATE users SET role = ? WHERE id = ?`, newRole, targetID)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'admin_role_change', ?)`,
		adminID, fmt.Sprintf("Changed role of %s (ID %d) from %s to %s", uname, targetID, curRole, newRole),
	)

	http.Redirect(w, r, fmt.Sprintf("/admin?success=Updated+%s+role+to+%s", uname, newRole), http.StatusSeeOther)
}

func (h *AdminHandler) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value(middleware.UserIDKey).(int64)
	targetID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil || targetID <= 0 {
		http.Redirect(w, r, "/admin?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}
	if targetID == adminID {
		http.Redirect(w, r, "/admin?error=You+cannot+suspend+your+own+account", http.StatusSeeOther)
		return
	}

	var curStatus, uname string
	if err := h.DB.QueryRow(`SELECT status, username FROM users WHERE id = ?`, targetID).Scan(&curStatus, &uname); err != nil {
		http.Redirect(w, r, "/admin?error=User+not+found", http.StatusSeeOther)
		return
	}

	newStatus := "suspended"
	if curStatus != "active" {
		newStatus = "active"
	}

	_, _ = h.DB.Exec(`UPDATE users SET status = ? WHERE id = ?`, newStatus, targetID)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'admin_status_change', ?)`,
		adminID, fmt.Sprintf("Changed account status of %s (ID %d) to %s", uname, targetID, newStatus),
	)

	http.Redirect(w, r, fmt.Sprintf("/admin?success=User+%s+marked+%s", uname, newStatus), http.StatusSeeOther)
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value(middleware.UserIDKey).(int64)
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	// Owner-only admin policy: provisioned accounts are always standard users
	role := "user"
	if username == "" || email == "" || len(password) < 6 {
		http.Redirect(w, r, "/admin?error=Provide+username,+email,+and+a+password+of+at+least+6+characters", http.StatusSeeOther)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/admin?error=Password+hashing+failed", http.StatusSeeOther)
		return
	}

	settings := h.DB.GetSystemSettings()
	res, err := h.DB.Exec(
		`INSERT INTO users (username, email, password, role, status, preferred_currency, monthly_budget_goal) VALUES (?, ?, ?, ?, 'active', 'INR', ?)`,
		username, email, string(hashed), role, settings.DefaultBudgetGoal,
	)
	if err != nil {
		http.Redirect(w, r, "/admin?error=Username+or+email+already+exists", http.StatusSeeOther)
		return
	}

	newUID, _ := res.LastInsertId()
	_ = h.DB.SeedDefaults(newUID)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'admin_create_user', ?)`,
		adminID, fmt.Sprintf("Provisioned %s account: %s (%s)", role, username, email),
	)

	http.Redirect(w, r, fmt.Sprintf("/admin?success=Created+%s+account+%s", role, username), http.StatusSeeOther)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value(middleware.UserIDKey).(int64)
	targetID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil || targetID <= 0 {
		http.Redirect(w, r, "/admin?error=Invalid+user+ID", http.StatusSeeOther)
		return
	}
	if targetID == adminID {
		http.Redirect(w, r, "/admin?error=You+cannot+delete+your+own+account", http.StatusSeeOther)
		return
	}

	var uname string
	_ = h.DB.QueryRow(`SELECT username FROM users WHERE id = ?`, targetID).Scan(&uname)

	_, _ = h.DB.Exec(`DELETE FROM users WHERE id = ?`, targetID)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'admin_delete_user', ?)`,
		adminID, fmt.Sprintf("Deleted user account %s (ID %d)", uname, targetID),
	)

	http.Redirect(w, r, "/admin?success=User+account+deleted", http.StatusSeeOther)
}

func (h *AdminHandler) UpdatePlatformSettings(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value(middleware.UserIDKey).(int64)

	allowReg := "false"
	if r.FormValue("allow_registration") == "true" || r.FormValue("allow_registration") == "on" {
		allowReg = "true"
	}
	banner := strings.TrimSpace(r.FormValue("announcement_banner"))
	thresholdStr := strings.TrimSpace(r.FormValue("anomaly_threshold"))
	defaultGoalStr := strings.TrimSpace(r.FormValue("default_budget_goal"))

	_ = h.DB.SetSystemSetting("allow_registration", allowReg)
	_ = h.DB.SetSystemSetting("announcement_banner", banner)

	if f, err := strconv.ParseFloat(thresholdStr, 64); err == nil && f >= 50 {
		_ = h.DB.SetSystemSetting("anomaly_threshold", fmt.Sprintf("%.2f", f))
	}
	if f, err := strconv.ParseFloat(defaultGoalStr, 64); err == nil && f >= 100 {
		_ = h.DB.SetSystemSetting("default_budget_goal", fmt.Sprintf("%.2f", f))
	}

	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'admin_settings_update', ?)`,
		adminID, fmt.Sprintf("Updated platform controls (registration=%s, anomalyThreshold=%s)", allowReg, thresholdStr),
	)

	http.Redirect(w, r, "/admin?success=Platform+settings+saved", http.StatusSeeOther)
}
