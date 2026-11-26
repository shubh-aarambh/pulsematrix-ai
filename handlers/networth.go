package handlers

import (
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"

	"expense-matrix/db"
	"expense-matrix/middleware"
	"expense-matrix/models"
	"expense-matrix/services"
)

type NetWorthHandler struct {
	DB              *db.DB
	Templates       *template.Template
	CurrencyService *services.CurrencyService
}

func NewNetWorthHandler(database *db.DB, tmpl *template.Template, cs *services.CurrencyService) *NetWorthHandler {
	return &NetWorthHandler{DB: database, Templates: tmpl, CurrencyService: cs}
}

func (h *NetWorthHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	username, _ := r.Context().Value(middleware.UsernameKey).(string)

	viewCtx := GetBaseContext(h.DB, h.CurrencyService, userID, username)

	rows, err := h.DB.Query(`
		SELECT id, name, category, kind, value, icon, strftime('%Y-%m-%d', updated_at)
		FROM net_worth_items
		WHERE user_id = ?
		ORDER BY kind ASC, value DESC`, userID)

	var assets, liabilities []models.NetWorthItem
	var totalAssets, totalLiabilities float64

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it models.NetWorthItem
			if err := rows.Scan(&it.ID, &it.Name, &it.Category, &it.Kind, &it.Value, &it.Icon, &it.UpdatedAt); err == nil {
				if it.Kind == "asset" {
					assets = append(assets, it)
					totalAssets += it.Value
				} else {
					liabilities = append(liabilities, it)
					totalLiabilities += it.Value
				}
			}
		}
	}

	netWorth := totalAssets - totalLiabilities
	debtRatio := 0.0
	assetSharePct := 100.0
	if totalAssets > 0 {
		debtRatio = math.Round((totalLiabilities/totalAssets)*1000) / 10
	}
	if totalAssets+totalLiabilities > 0 {
		assetSharePct = math.Round((totalAssets/(totalAssets+totalLiabilities))*1000) / 10
	}

	viewCtx["Title"] = "Net Worth & Wealth Balance"
	viewCtx["ActivePage"] = "networth"
	viewCtx["Assets"] = assets
	viewCtx["Liabilities"] = liabilities
	viewCtx["TotalAssets"] = totalAssets
	viewCtx["TotalLiabilities"] = totalLiabilities
	viewCtx["NetWorth"] = netWorth
	viewCtx["DebtRatio"] = debtRatio
	viewCtx["AssetSharePct"] = assetSharePct
	viewCtx["FlashSuccess"] = r.URL.Query().Get("success")
	viewCtx["Error"] = r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.Templates.Execute(w, viewCtx)
}

func (h *NetWorthHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)

	name := strings.TrimSpace(r.FormValue("name"))
	cat := strings.TrimSpace(r.FormValue("category"))
	kind := strings.TrimSpace(r.FormValue("kind"))
	icon := strings.TrimSpace(r.FormValue("icon"))
	val, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("value")), 64)

	if name == "" || err != nil || val < 0 {
		http.Redirect(w, r, "/networth?error=Provide+a+valid+account+name+and+value", http.StatusSeeOther)
		return
	}
	if kind != "asset" && kind != "liability" {
		kind = "asset"
	}
	if cat == "" {
		cat = "Bank Cash"
	}
	if icon == "" {
		if kind == "liability" {
			icon = "💳"
		} else {
			icon = "🏦"
		}
	}

	_, _ = h.DB.Exec(`
		INSERT INTO net_worth_items (user_id, name, category, kind, value, icon)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, name, cat, kind, val, icon,
	)
	http.Redirect(w, r, "/networth?success=Balance+sheet+item+added", http.StatusSeeOther)
}

func (h *NetWorthHandler) UpdateValue(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	itemID, err1 := strconv.ParseInt(r.FormValue("id"), 10, 64)
	val, err2 := strconv.ParseFloat(strings.TrimSpace(r.FormValue("value")), 64)

	if err1 != nil || err2 != nil || itemID <= 0 || val < 0 {
		http.Redirect(w, r, "/networth?error=Invalid+valuation+update", http.StatusSeeOther)
		return
	}

	_, _ = h.DB.Exec(`UPDATE net_worth_items SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, val, itemID, userID)
	http.Redirect(w, r, "/networth?success=Valuation+updated", http.StatusSeeOther)
}

func (h *NetWorthHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int64)
	_, _ = h.DB.Exec(`DELETE FROM net_worth_items WHERE id = ? AND user_id = ?`, r.FormValue("id"), userID)
	http.Redirect(w, r, "/networth?success=Item+removed+from+balance+sheet", http.StatusSeeOther)
}
