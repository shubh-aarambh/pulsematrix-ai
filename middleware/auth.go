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

// Auth returns a Gorilla Mux middleware enforcing JWT authorization and live user status/role check.
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
				handleUnauth(w, r, "/login")
				return
			}

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				handleUnauth(w, r, "/login")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				handleUnauth(w, r, "/login")
				return
			}

			userIDFloat, ok1 := claims["sub"].(float64)
			username, ok2 := claims["username"].(string)
			if !ok1 || !ok2 {
				handleUnauth(w, r, "/login")
				return
			}

			userID := int64(userIDFloat)
			role := "user"
			status := "active"

			if database != nil {
				var dbUsername, dbRole, dbStatus string
				if err := database.QueryRow(`SELECT username, role, status FROM users WHERE id = ?`, userID).Scan(&dbUsername, &dbRole, &dbStatus); err != nil {
					handleUnauth(w, r, "/login")
					return
				}
				username = dbUsername
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
					http.Error(w, `{"error":"Account suspended by administrator"}`, http.StatusForbidden)
				} else {
					http.Redirect(w, r, "/login?error=Account+has+been+suspended+by+an+administrator", http.StatusSeeOther)
				}
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UsernameKey, username)
			ctx = context.WithValue(ctx, RoleKey, role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin enforces that the authenticated user has the 'admin' role.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(RoleKey).(string)
			if role != "admin" {
				if isAPIRequest(r) {
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"error":"Administrator access required"}`, http.StatusForbidden)
				} else {
					http.Redirect(w, r, "/admin/login?error=Administrator+privileges+required", http.StatusSeeOther)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func handleUnauth(w http.ResponseWriter, r *http.Request, redirectTo string) {
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Unauthorized access"}`, http.StatusUnauthorized)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin") {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json")
}
