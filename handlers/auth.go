package handlers

import (
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
	DB               *db.DB
	LoginTemplate    *template.Template
	RegisterTemplate *template.Template
	JWTSecret        string
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.LoginTemplate.Execute(w, map[string]interface{}{
		"Title": "Login - PulseMatrix AI",
		"Error": r.URL.Query().Get("error"),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	identifier := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	var id int64
	var dbUsername string
	var hash string

	err := h.DB.QueryRow(`SELECT id, username, password FROM users WHERE username = ? OR email = ?`, identifier, identifier).Scan(&id, &dbUsername, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = h.LoginTemplate.Execute(w, map[string]interface{}{
			"Title": "Login - PulseMatrix AI",
			"Error": "Invalid credentials provided.",
		})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      id,
		"username": dbUsername,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		http.Error(w, "Failed to generate session token", http.StatusInternalServerError)
		return
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
		"Title": "Register - PulseMatrix AI",
		"Error": r.URL.Query().Get("error"),
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if username == "" || email == "" || password == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title": "Register - PulseMatrix AI",
			"Error": "All fields are required.",
		})
		return
	}

	if len(password) < 6 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title": "Register - PulseMatrix AI",
			"Error": "Password must be at least 6 characters.",
		})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error processing security key", http.StatusInternalServerError)
		return
	}

	res, err := h.DB.Exec(`INSERT INTO users (username, email, password, preferred_currency, monthly_budget_goal) VALUES (?, ?, ?, 'USD', 4500.0)`, username, email, string(hashed))
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_ = h.RegisterTemplate.Execute(w, map[string]interface{}{
			"Title": "Register - PulseMatrix AI",
			"Error": "Username or email is already registered.",
		})
		return
	}

	userID, _ := res.LastInsertId()

	// Seed default categories & sample dataset
	_ = h.DB.SeedDefaults(userID)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		http.Error(w, "Failed to generate session token", http.StatusInternalServerError)
		return
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
		"USD": true, "EUR": true, "GBP": true, "INR": true, "CAD": true, "AUD": true, "JPY": true,
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
