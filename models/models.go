package models

import "time"

type User struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	Password          string    `json:"-"`
	Role              string    `json:"role"`   // user | admin
	Status            string    `json:"status"` // active | suspended
	PreferredCurrency string    `json:"preferredCurrency"`
	MonthlyBudgetGoal float64   `json:"monthlyBudgetGoal"`
	LastLoginAt       string    `json:"lastLoginAt"`
	CreatedAt         time.Time `json:"createdAt"`
}

type Category struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId"`
	Name         string    `json:"name"`
	Icon         string    `json:"icon"`
	Color        string    `json:"color"`
	Type         string    `json:"type"` // expense | income | investment
	MonthlyLimit float64   `json:"monthlyLimit"`
	CreatedAt    time.Time `json:"createdAt"`
}

type CategoryBudgetStatus struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Icon         string  `json:"icon"`
	Color        string  `json:"color"`
	Type         string  `json:"type"`
	MonthlyLimit float64 `json:"monthlyLimit"`
	Spent        float64 `json:"spent"`
	Remaining    float64 `json:"remaining"`
	Utilization  float64 `json:"utilization"`
	Status       string  `json:"status"` // ok | warning | exceeded | unset
}

type SavingsGoal struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"userId"`
	Name          string  `json:"name"`
	Icon          string  `json:"icon"`
	TargetAmount  float64 `json:"targetAmount"`
	CurrentAmount float64 `json:"currentAmount"`
	TargetDate    string  `json:"targetDate"`
	Category      string  `json:"category"`
	Status        string  `json:"status"` // active | completed
	ProgressPct   float64 `json:"progressPct"`
	Remaining     float64 `json:"remaining"`
}

type Transaction struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"userId"`
	CategoryID       *int64    `json:"categoryId"`
	CategoryName     string    `json:"categoryName"`
	CategoryIcon     string    `json:"categoryIcon"`
	CategoryColor    string    `json:"categoryColor"`
	Amount           float64   `json:"amount"`           // Normalized in base currency
	OriginalAmount   float64   `json:"originalAmount"`   // In original spent currency
	OriginalCurrency string    `json:"originalCurrency"` // USD, EUR, INR, etc.
	ExchangeRate     float64   `json:"exchangeRate"`
	Vendor           string    `json:"vendor"`
	Description      string    `json:"description"`
	Date             string    `json:"date"`
	Source           string    `json:"source"` // manual | ai_ocr | bank_sync | csv_import | payment_app_api | sms_auto_sync
	PaymentMethod    string    `json:"paymentMethod"`
	IsRecurring      bool      `json:"isRecurring"`
	IsAnomaly        bool      `json:"isAnomaly"`
	AnomalyReason    string    `json:"anomalyReason"`
	ReceiptURL       string    `json:"receiptUrl"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Subscription struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"userId"`
	Name            string    `json:"name"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	CategoryName    string    `json:"categoryName"`
	BillingCycle    string    `json:"billingCycle"` // monthly | yearly | weekly
	NextBillingDate string    `json:"nextBillingDate"`
	Status          string    `json:"status"` // active | paused | cancelled
	DetectedPattern string    `json:"detectedPattern"`
	CreatedAt       time.Time `json:"createdAt"`
}

type FinancialHealth struct {
	Score              int      `json:"score"`              // 0 - 100
	Rating             string   `json:"rating"`             // Optimal | Healthy | Caution | Critical
	MonthlyIncome      float64  `json:"monthlyIncome"`
	MonthlySpend       float64  `json:"monthlySpend"`
	NetSavings         float64  `json:"netSavings"`
	SavingsRate        float64  `json:"savingsRate"`        // percentage
	SubscriptionWeight float64  `json:"subscriptionWeight"` // percentage of spend
	BudgetUtilization  float64  `json:"budgetUtilization"`  // percentage of goal
	TopCategory        string   `json:"topCategory"`
	AnomalyCount       int      `json:"anomalyCount"`
	Recommendations    []string `json:"recommendations"`
}

type AIParsedReceipt struct {
	Vendor            string  `json:"vendor"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	SuggestedCategory string  `json:"suggestedCategory"`
	Date              string  `json:"date"`
	PaymentMethod     string  `json:"paymentMethod"`
	ConfidenceScore   float64 `json:"confidenceScore"`
	IsAnomaly         bool    `json:"isAnomaly"`
	AnomalyReason     string  `json:"anomalyReason"`
	RawSummary        string  `json:"rawSummary"`
}

type AIChatMessage struct {
	Role      string    `json:"role"` // user | assistant
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type MonthlyTrend struct {
	Month      string  `json:"month"`
	Income     float64 `json:"income"`
	Expense    float64 `json:"expense"`
	Savings    float64 `json:"savings"`
	BudgetPace float64 `json:"budgetPace"`
}

type CategorySpend struct {
	Name       string  `json:"name"`
	Icon       string  `json:"icon"`
	Color      string  `json:"color"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type BankWebhookPayload struct {
	AccountID     string  `json:"accountId"`
	TransactionID string  `json:"transactionId"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Merchant      string  `json:"merchant"`
	Category      string  `json:"category"`
	Date          string  `json:"date"`
	Pending       bool    `json:"pending"`
	SecurityToken string  `json:"securityToken"`
}

type AdminUserSummary struct {
	ID                int64
	Username          string
	Email             string
	Role              string
	Status            string
	PreferredCurrency string
	MonthlyBudgetGoal float64
	TxCount           int
	TotalSpend        float64
	SubCount          int
	LastLoginAt       string
	CreatedAt         string
}

type PlatformStats struct {
	TotalUsers        int
	ActiveUsers       int
	SuspendedUsers    int
	AdminCount        int
	TotalTransactions int
	TotalVolumeUSD    float64
	TotalAnomalies    int
	ActiveSubs        int
	TotalGoals        int
}

type SystemSettings struct {
	AllowRegistration  bool
	AnnouncementBanner string
	AnomalyThreshold   float64
	DefaultBudgetGoal  float64
}
