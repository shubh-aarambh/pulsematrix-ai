package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"expense-matrix/db"
	"expense-matrix/models"

	"github.com/golang-jwt/jwt/v5"
)

type ViewContext map[string]interface{}

// GetBaseContext retrieves standard layout context required by base.html.
func GetBaseContext(database *db.DB, userID int64, username string) ViewContext {
	var user models.User
	_ = database.QueryRow(`SELECT id, username, email, preferred_currency, monthly_budget_goal FROM users WHERE id = ?`, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.PreferredCurrency, &user.MonthlyBudgetGoal,
	)
	if user.Username == "" {
		user.Username = username
	}
	if user.PreferredCurrency == "" {
		user.PreferredCurrency = "USD"
	}
	if user.MonthlyBudgetGoal <= 0 {
		user.MonthlyBudgetGoal = 4500.0
	}

	catRows, _ := database.Query(`SELECT id, name, icon, color FROM categories WHERE user_id = ? ORDER BY name ASC`, userID)
	categories := make([]models.Category, 0)
	if catRows != nil {
		defer catRows.Close()
		for catRows.Next() {
			var c models.Category
			if err := catRows.Scan(&c.ID, &c.Name, &c.Icon, &c.Color); err == nil {
				categories = append(categories, c)
			}
		}
	}

	now := time.Now()
	return ViewContext{
		"Username":    user.Username,
		"User":        user,
		"CurrentDate": now.Format("Jan 02, 2006"),
		"TodayISO":    now.Format("2006-01-02"),
		"Categories":  categories,
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
