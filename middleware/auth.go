package middleware

import (
	"context"
	"net/http"
	"strings"

	"expense-matrix/db"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UsernameKey contextKey = "username"
const RoleKey contextKey = "role"
const EmailKey contextKey = "email"

// Auth enforces JWT authorization and live active status/role verification from DB.
func Auth(secret string, database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				handleUnauth(w, r)
				return
			}

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				handleUnauth(w, r)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				handleUnauth(w, r)
				return
			}

			userIDFloat, ok1 := claims["sub"].(float64)
			username, ok2 := claims["username"].(string)
			if !ok1 || !ok2 {
				handleUnauth(w, r)
				return
			}

			userID := int64(userIDFloat)
			role := "user"
			status := "active"
			email := ""

			if database != nil {
				var dbUsername, dbEmail, dbRole, dbStatus string
				if err := database.QueryRow(`SELECT username, email, role, status FROM users WHERE id = ?`, userID).Scan(&dbUsername, &dbEmail, &dbRole, &dbStatus); err != nil {
					handleUnauth(w, r)
					return
				}
				username = dbUsername
				email = dbEmail
				role = dbRole
				status = dbStatus
			} else if rClaim, ok := claims["role"].(string); ok && rClaim != "" {
				role = rClaim
			}

			if status != "active" {
				http.SetCookie(w, &http.Cookie{
					Name:     "jwt_token",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
				})
				if isAPIRequest(r) {
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"error":"Account suspended"}`, http.StatusForbidden)
				} else {
					http.Redirect(w, r, "/login?error=Account+has+been+suspended", http.StatusSeeOther)
				}
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UsernameKey, username)
			ctx = context.WithValue(ctx, RoleKey, role)
			ctx = context.WithValue(ctx, EmailKey, email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireOwnerAdmin strictly allows ONLY the platform owner admin account.
// For any standard user or unauthorized probe, it returns a stealth 404 Not Found
// so regular users never discover or see the admin section.
func RequireOwnerAdmin(ownerEmail string) func(http.Handler) http.Handler {
	normalizedOwner := strings.ToLower(strings.TrimSpace(ownerEmail))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(RoleKey).(string)
			email, _ := r.Context().Value(EmailKey).(string)

			isOwner := role == "admin" && (normalizedOwner == "" || strings.EqualFold(strings.TrimSpace(email), normalizedOwner))
			if !isOwner {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func handleUnauth(w http.ResponseWriter, r *http.Request) {
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Unauthorized access"}`, http.StatusUnauthorized)
		return
	}
	// Stealth 404 for unauthenticated probes to /admin paths
	if strings.HasPrefix(r.URL.Path, "/admin") {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json")
}
