# ⚡ PulseMatrix AI v5.0 — Autonomous Financial Intelligence & Wealth OS

[![Live Demo](https://img.shields.io/badge/Live%20App-pulsematrix--ai.onrender.com-00e5a0?style=for-the-badge)](https://pulsematrix-ai.onrender.com)
[![Deploy to Render](https://img.shields.io/badge/Deploy%20to-Render-46E3B7?style=for-the-badge)](https://render.com/deploy?repo=https://github.com/shubh-aarambh/pulsematrix-ai)

> **Live Deployment:** https://pulsematrix-ai.onrender.com (Health Check: https://pulsematrix-ai.onrender.com/health)
>
> A flagship, production-grade personal finance & wealth analytics platform engineered in **Go 1.23 & Pure-Go SQLite (WAL)**, featuring an **INR-first live multi-currency FX engine** (`₹ INR` ↔ `$ USD` ↔ `€ EUR` ↔ `£ GBP`), **12-Column Carbon Blueprint Bento Grid UI**, **Split-Bill & Shared Expense Settlements**, **Net Worth Assets/Liabilities Balance Sheet**, **30-Day Spend Heatmap**, **Printable Executive PDF Statements**, **Token-Bucket Rate Limiting**, **Prometheus Runtime Telemetry**, **Async Background Workers**, and **Idempotent HMAC-SHA256 Payment Webhooks**.

---

## 🏗️ Backend Systems & Architecture (Resume Highlights)

Add these bullet points directly to your **Projects / Engineering Experience** section:

* **Architected production Go 1.23 fintech backend** with Gorilla Mux, pure-Go SQLite WAL (`modernc.org/sqlite`), clean layered handlers/services/middleware, and **graceful `SIGINT`/`SIGTERM` shutdown** draining in-flight HTTP requests and stopping background goroutines cleanly.
* **Engineered thread-safe per-IP Token-Bucket Rate Limiter (`middleware/ratelimit.go`)** with automatic idle-bucket eviction and RFC-compliant `X-RateLimit-*` / `429 Retry-After` headers, paired with **Request-ID distributed tracing**, structured JSON latency logging, panic recovery, and hardened security headers (`middleware/observability.go`).
* **Built real-time Runtime Telemetry & Prometheus Exporter (`services/metrics_service.go`)** tracking rolling p95/avg latency, throughput, error counts, active goroutines, heap allocation, and GC cycles without external APM dependencies.
* **Implemented Asynchronous Background Worker Pool (`services/worker_service.go`)** running context-cancellable periodic sweeps for **statistical surge anomaly detection (>2.5× category mean)**, automatic subscription renewal rollovers, and proactive live FX cache warming.
* **Hardened Payment App & SMS Ingestion APIs** (`/api/webhooks/payment`, `/api/v1/ingest/sms`, `/api/webhooks/bank`) with **SQLite-backed `X-Idempotency-Key` deduplication** and **HMAC-SHA256 (`X-Webhook-Signature`) payload verification**, plus automated Google Gemini AI + heuristic NLP receipt parsing.
* **Designed Stealth Owner-Only RBAC Control Plane** using constant-time 3-factor access key verification (`crypto/subtle`), live DB session status checks, and zero-visibility 404 stealth responses for non-owner probes.
* **Delivered full-stack wealth management features**: INR-first multi-currency live FX normalization, **Net Worth Balance Sheet**, **Split-Bill Tracker with Ledger Sync**, **Category Budget Envelopes & Savings Goals**, **30-Day Spending Heatmap**, **Burn-Rate Forecast**, **CSV Bulk Import/Export**, **Command Palette (`Ctrl+K`)**, and **Printable PDF Statements**.
* **Verified with automated Go unit/race tests (`go test -race ./...`)**, end-to-end integration tests, and **CGO-free multi-stage Alpine Docker deployment** on Render.

---

## 🛠️ Technology Stack

| Layer | Technology |
| :--- | :--- |
| **Backend Core** | Go 1.23, Gorilla Mux, Context-Cancellable Worker Goroutines, Graceful Shutdown |
| **Resilience & Observability** | Per-IP Token-Bucket Rate Limiter, Request-ID Tracing, Panic Recovery, Prometheus Metrics |
| **Database** | Pure-Go SQLite (`modernc.org/sqlite`) with WAL Mode, Foreign Keys & Idempotency Store |
| **AI & Live FX** | Google Gemini API + Heuristic NLP Fallback, Live Multi-Provider FX Engine (`INR`/`USD`/`EUR`/`GBP`) |
| **Security & Auth** | JWT (`golang-jwt/v5`), Bcrypt, Constant-Time Owner Access Key, HMAC-SHA256 Webhooks |
| **Frontend UI** | Server-Rendered `html/template`, Carbon Blueprint Bento Grid CSS3, Dark/Light Themes, Chart.js |
| **DevOps & Testing** | Multi-Stage Docker, `render.yaml` Blueprint, `go test -race ./...`, Live Health Checks |

---

## 🚀 Quick Start (Local)

```bash
cd expense-matrix
go test -v -race ./...
go build -o pulsematrix.exe .
.\pulsematrix.exe
```

Open **`http://localhost:8080`** in your browser.
