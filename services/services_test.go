package services

import (
	"testing"
	"time"

	"expense-matrix/config"
)

func TestHeuristicReceiptAndSMSParser(t *testing.T) {
	ai := NewAIService(&config.Config{EnableAIServices: true})

	parsed, err := ai.ParseReceiptOrText("Paid Rs 450.00 to Starbucks Coffee via Google Pay UPI on 25-Nov-2026 ref 9981273")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Currency != "INR" {
		t.Errorf("expected currency INR, got %s", parsed.Currency)
	}
	if parsed.Amount != 450.0 {
		t.Errorf("expected amount 450.0, got %.2f", parsed.Amount)
	}
	if parsed.Vendor != "Starbucks Coffee" {
		t.Errorf("expected vendor Starbucks Coffee, got %s", parsed.Vendor)
	}
	if parsed.SuggestedCategory != "Food & Dining" {
		t.Errorf("expected category Food & Dining, got %s", parsed.SuggestedCategory)
	}
	if parsed.PaymentMethod != "Google Pay / UPI" {
		t.Errorf("expected payment method Google Pay / UPI, got %s", parsed.PaymentMethod)
	}
}

func TestCurrencyConversionAndSymbols(t *testing.T) {
	cs := NewCurrencyService(&config.Config{})
	// Set lastFetch to now so test runs offline deterministically against defaults
	cs.mu.Lock()
	cs.lastFetch = time.Now()
	cs.mu.Unlock()

	if SymbolFor("INR") != "₹" || SymbolFor("USD") != "$" || SymbolFor("EUR") != "€" {
		t.Fatalf("unexpected currency symbols")
	}

	inrAmt, rate, err := cs.Convert(10.0, "USD", "INR")
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if inrAmt < 700 || rate < 70 {
		t.Fatalf("unexpected USD->INR conversion: amt=%.2f rate=%.2f", inrAmt, rate)
	}
}

func TestMetricsServicePercentiles(t *testing.T) {
	m := NewMetricsService()
	m.RecordRequest(200, 10*time.Millisecond)
	m.RecordRequest(200, 20*time.Millisecond)
	m.RecordRequest(500, 50*time.Millisecond)
	m.RecordRequest(429, 5*time.Millisecond)

	snap := m.Snapshot()
	if snap.TotalRequests != 4 || snap.ErrorRequests != 1 || snap.RateLimited != 1 {
		t.Fatalf("unexpected counters: %+v", snap)
	}
	if snap.P95LatencyMs < 20 {
		t.Fatalf("expected p95 >= 20ms, got %.2f", snap.P95LatencyMs)
	}
}
