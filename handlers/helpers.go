package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/models"
	"expense-matrix/services"

	"github.com/golang-jwt/jwt/v5"
)

type ViewContext map[string]interface{}

// GetBaseContext retrieves standard layout context required by base.html,
// including live INR -> PreferredCurrency conversion rate and currency symbol.
func GetBaseContext(database *db.DB, cs *services.CurrencyService, userID int64, username string) ViewContext {
	var user models.User
	_ = database.QueryRow(`SELECT id, username, email, role, status, preferred_currency, monthly_budget_goal, COALESCE(last_login_at, '') FROM users WHERE id = ?`, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role, &user.Status, &user.PreferredCurrency, &user.MonthlyBudgetGoal, &user.LastLoginAt,
	)
	if user.Username == "" {
		user.Username = username
	}
	if user.Role == "" {
		user.Role = "user"
	}
	if user.PreferredCurrency == "" {
		user.PreferredCurrency = "INR"
	}
	if user.MonthlyBudgetGoal <= 0 {
		user.MonthlyBudgetGoal = 75000.0
	}

	displayRate := 1.0
	if cs != nil && user.PreferredCurrency != "INR" {
		displayRate = cs.RateBetween("INR", user.PreferredCurrency)
	}
	currencySymbol := services.SymbolFor(user.PreferredCurrency)

	catRows, _ := database.Query(`SELECT id, name, icon, color, type, monthly_limit FROM categories WHERE user_id = ? ORDER BY name ASC`, userID)
	categories := make([]models.Category, 0)
	if catRows != nil {
		defer catRows.Close()
		for catRows.Next() {
			var c models.Category
			if err := catRows.Scan(&c.ID, &c.Name, &c.Icon, &c.Color, &c.Type, &c.MonthlyLimit); err == nil {
				categories = append(categories, c)
			}
		}
	}

	sysSettings := database.GetSystemSettings()
	now := time.Now()
	return ViewContext{
		"Username":           user.Username,
		"User":               user,
		"UserRole":           user.Role,
		"IsAdmin":            user.Role == "admin",
		"CurrencyCode":       user.PreferredCurrency,
		"CurrencySymbol":     currencySymbol,
		"DisplayRate":        displayRate,
		"AnnouncementBanner": sysSettings.AnnouncementBanner,
		"CurrentDate":        now.Format("02 Jan 2006"),
		"TodayISO":           now.Format("2006-01-02"),
		"Categories":         categories,
	}
}

// WriteJSONError writes a properly encoded JSON error response.
func WriteJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ExtractOptionalUserID extracts a user ID from a JWT cookie or Authorization header if present.
func ExtractOptionalUserID(r *http.Request, jwtSecret string) (int64, bool) {
	tokenStr := ""
	if cookie, err := r.Cookie("jwt_token"); err == nil {
		tokenStr = cookie.Value
	}
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if tokenStr == "" {
		return 0, false
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	if sub, ok := claims["sub"].(float64); ok && sub > 0 {
		return int64(sub), true
	}
	return 0, false
}
