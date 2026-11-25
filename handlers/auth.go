package handlers

import (
	"crypto/subtle"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB                 *db.DB
	LoginTemplate      *template.Template
	AdminLoginTemplate *template.Template
	RegisterTemplate   *template.Template
	JWTSecret          string
	AdminLoginPath     string
	AdminAccessKey     string
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *AuthHandler) issueSessionCookie(w http.ResponseWriter, r *http.Request, userID int64, username, role string) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt_token",
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	settings := h.DB.GetSystemSettings()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.LoginTemplate.Execute(w, map[string]interface{}{
		"Title":             "Sign In - PulseMatrix AI",
		"Error":             r.URL.Query().Get("error"),
		"Success":           r.URL.Query().Get("success"),
		"AllowRegistration": settings.AllowRegistration,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	identifier := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	var id int64
	var dbUsername, hash, role, status string

	err := h.DB.QueryRow(`SELECT id, username, password, role, status FROM users WHERE username = ? OR email = ?`, identifier, identifier).Scan(&id, &dbUsername, &hash, &role, &status)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = h.LoginTemplate.Execute(w, map[string]interface{}{
			"Title":             "Sign In - PulseMatrix AI",
			"Error":             "Invalid username/email or password.",
			"AllowRegistration": h.DB.GetSystemSettings().AllowRegistration,
		})
		return
	}

	// Owner admin must sign in via the private owner console URL (not the public user form)
	if role == "admin" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = h.LoginTemplate.Execute(w, map[string]interface{}{
			"Title":             "Sign In - PulseMatrix AI",
			"Error":             "Invalid username/email or password.",
			"AllowRegistration": h.DB.GetSystemSettings().AllowRegistration,
		})
		return
	}

	if status != "active" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = h.LoginTemplate.Execute(w, map[string]interface{}{
			"Title":             "Sign In - PulseMatrix AI",
			"Error":             "Your account has been suspended. Please contact support.",
			"AllowRegistration": h.DB.GetSystemSettings().AllowRegistration,
		})
		return
	}

	if err := h.issueSessionCookie(w, r, id, dbUsername, "user"); err != nil {
		http.Error(w, "Failed to generate session token", http.StatusInternalServerError)
		return
	}

	nowStr := time.Now().Format("2006-01-02 15:04")
	_, _ = h.DB.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, nowStr, id)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'user_login', ?)`, id, fmt.Sprintf("User %s signed in", dbUsername))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ShowAdminLogin serves the unlisted, owner-only administrator sign-in page.
func (h *AuthHandler) ShowAdminLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	_ = h.AdminLoginTemplate.Execute(w, map[string]interface{}{
		"Title":          "Owner Console - PulseMatrix AI",
		"AdminLoginPath": h.AdminLoginPath,
		"Error":          r.URL.Query().Get("error"),
	})
}

// AdminLogin strictly verifies Owner Admin credentials + Owner Access Passkey.
func (h *AuthHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	identifier := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	accessKey := strings.TrimSpace(r.FormValue("access_key"))

	keyValid := h.AdminAccessKey != "" &&
		len(accessKey) == len(h.AdminAccessKey) &&
		subtle.ConstantTimeCompare([]byte(accessKey), []byte(h.AdminAccessKey)) == 1

	var id int64
	var dbUsername, hash, role, status string

	err := h.DB.QueryRow(`SELECT id, username, password, role, status FROM users WHERE (username = ? OR email = ?) AND role = 'admin'`, identifier, identifier).Scan(&id, &dbUsername, &hash, &role, &status)
	if !keyValid || err != nil || role != "admin" || status != "active" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		w.WriteHeader(http.StatusUnauthorized)
		_ = h.AdminLoginTemplate.Execute(w, map[string]interface{}{
			"Title":          "Owner Console - PulseMatrix AI",
			"AdminLoginPath": h.AdminLoginPath,
			"Error":          "Access denied. Invalid owner credentials or passkey.",
		})
		return
	}

	if err := h.issueSessionCookie(w, r, id, dbUsername, "admin"); err != nil {
		http.Error(w, "Failed to generate session token", http.StatusInternalServerError)
		return
	}

	nowStr := time.Now().Format("2006-01-02 15:04")
	_, _ = h.DB.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, nowStr, id)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'owner_admin_login', ?)`, id, fmt.Sprintf("Owner admin %s authenticated", dbUsername))

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	settings := h.DB.GetSystemSettings()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
		"Title":             "Create Account - PulseMatrix AI",
		"Error":             r.URL.Query().Get("error"),
		"AllowRegistration": settings.AllowRegistration,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	settings := h.DB.GetSystemSettings()
	if !settings.AllowRegistration {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title":             "Create Account - PulseMatrix AI",
			"Error":             "New account registrations are currently paused.",
			"AllowRegistration": false,
		})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if username == "" || email == "" || password == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title":             "Create Account - PulseMatrix AI",
			"Error":             "All fields are required.",
			"AllowRegistration": true,
		})
		return
	}

	if len(password) < 6 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title":             "Create Account - PulseMatrix AI",
			"Error":             "Password must be at least 6 characters.",
			"AllowRegistration": true,
		})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error processing security key", http.StatusInternalServerError)
		return
	}

	nowStr := time.Now().Format("2006-01-02 15:04")
	res, err := h.DB.Exec(
		`INSERT INTO users (username, email, password, role, status, preferred_currency, monthly_budget_goal, last_login_at) VALUES (?, ?, ?, 'user', 'active', 'INR', ?, ?)`,
		username, email, string(hashed), settings.DefaultBudgetGoal, nowStr,
	)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title":             "Create Account - PulseMatrix AI",
			"Error":             "Username or email is already registered.",
			"AllowRegistration": true,
		})
		return
	}

	userID, _ := res.LastInsertId()
	_ = h.DB.SeedDefaults(userID)
	_, _ = h.DB.Exec(`INSERT INTO audit_logs (user_id, action, details) VALUES (?, 'user_registered', ?)`, userID, fmt.Sprintf("New user %s (%s) registered", username, email))

	if err := h.issueSessionCookie(w, r, userID, username, "user"); err != nil {
		http.Error(w, "Failed to generate session token", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/?success=Welcome+to+PulseMatrix+AI!", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("preferredCurrency")))
	budgetStr := strings.TrimSpace(r.FormValue("monthlyBudgetGoal"))

	allowedCurrencies := map[string]bool{
		"INR": true, "USD": true, "EUR": true, "GBP": true, "CAD": true, "AUD": true, "JPY": true,
	}
	if allowedCurrencies[currency] {
		_, _ = h.DB.Exec(`UPDATE users SET preferred_currency = ? WHERE id = ?`, currency, userID)
	}
	if budgetStr != "" {
		if goal, err := strconv.ParseFloat(budgetStr, 64); err == nil && goal > 0 {
			_, _ = h.DB.Exec(`UPDATE users SET monthly_budget_goal = ? WHERE id = ?`, goal, userID)
		}
	}

	http.Redirect(w, r, "/?success=Account+settings+updated", http.StatusSeeOther)
}

func (h *AuthHandler) SwitchCurrency(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	allowed := map[string]bool{
		"INR": true, "USD": true, "EUR": true, "GBP": true, "CAD": true, "AUD": true, "JPY": true,
	}
	if allowed[currency] {
		_, _ = h.DB.Exec(`UPDATE users SET preferred_currency = ? WHERE id = ?`, currency, userID)
	}
	back := r.Header.Get("Referer")
	if back == "" || !strings.HasPrefix(back, "http") {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
