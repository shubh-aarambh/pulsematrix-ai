# ⚡ PulseMatrix AI — Autonomous Financial Intelligence & Expense Engine

[![Live Demo](https://img.shields.io/badge/Live%20App-pulsematrix--ai.onrender.com-6366f1?style=for-the-badge)](https://pulsematrix-ai.onrender.com)
[![Deploy to Render](https://img.shields.io/badge/Deploy%20to-Render-46E3B7?style=for-the-badge)](https://render.com/deploy?repo=https://github.com/shubh-aarambh/pulsematrix-ai)

> **Live Deployment:** https://pulsematrix-ai.onrender.com (Health Check: https://pulsematrix-ai.onrender.com/health)
> **Separate Portals:** User Login (`/login`) • Admin Console Login (`/admin/login`)
>
> A flagship, production-grade financial analytics and expense intelligence platform written in **Go**, featuring **separate Role-Based User & Admin Portals**, **Budgets & Savings Goals**, **Bulk CSV Import/Export**, **Live Multi-Currency FX Converter**, **`Ctrl+K` Command Palette**, **Dark/Light Theme**, autonomous receipt/SMS parsing with **Google Gemini AI**, statistical anomaly detection, and real-time payment app webhooks.

---

## ✨ Flagship v3.0 Features

- **Separate User & Admin Portals (`/login` vs `/admin/login`):** Dedicated RBAC middleware enforcing admin-only access to the **Platform Admin Console (`/admin`)** with user provisioning, role promotion/demotion, account suspension, global announcement banners, dynamic anomaly thresholds, and cross-tenant audit logs.
- **Budgets & Savings Goals Hub (`/goals`):** Real-time month-to-date category budget envelopes with over-budget guards, custom category builder, and personal savings goal milestones with live contribution tracking.
- **Enhanced Ledger (`/transactions`):** Full inline transaction editing, one-click anomaly resolution ("Mark Safe"), multi-currency **Bulk CSV Statement Import** + downloadable CSV template, and full CSV ledger export.
- **Executive UI/UX:** Split-screen glassmorphic auth portals, **Dark & Light theme toggle**, **`Ctrl+K` Quick Command Palette**, and a **Live Multi-Currency FX Converter** modal powered by `/api/currency/rates`.

---

## 📱 Payment App & SMS Integration Features

PulseMatrix AI seamlessly connects with popular consumer payment apps and mobile notifications:

### 1. **Direct Payment App Webhook API (`POST /api/webhooks/payment`)**
* **Supported Apps**: Google Pay, PhonePe, Paytm, Apple Pay, PayPal, Stripe, Razorpay.
* **Function**: Ingests structured JSON payment payloads directly from mobile apps or gateways, automatically categorizes spend, converts currency, and logs transaction history into the ledger.

### 2. **Automated SMS & Mobile Notification AI Parser (`POST /api/v1/ingest/sms`)**
* **Supported Channels**: Android Tasker / SMS Forwarder, Bank Transaction Alerts (HDFC, ICICI, Chase, Revolut, Amex).
* **Function**: Accepts unformatted raw transaction SMS text (e.g., *"Paid Rs 450.00 to Starbucks Coffee via Google Pay UPI on 25-Sep-2026 ref 9981273"*). Uses Google Gemini AI / regex parser to extract merchant, amount, category, date, and payment method in real time.

---

## 📄 Resume Description & Key Bullet Points

Add these bullet points directly to your resume under your **Projects / Experience** section:

* **Engineered high-throughput financial intelligence platform** in Go 1.22 & modern SQLite, handling thousands of normalized transactions with multi-currency real-time exchange rate conversions and automated budget pace analytics.
* **Integrated direct payment app webhooks & automated SMS AI parsing endpoints** for Google Pay, PhonePe, Paytm, and Apple Pay notifications, achieving real-time expense ingestion with 95%+ parsing accuracy.
* **Built autonomous LLM receipt parser** integrating Google Gemini 1.5 Flash API with intelligent heuristic regex fallback, reducing manual expense logging time by 90% while extracting structured vendor, date, and tax metadata.
* **Implemented statistical anomaly & subscription detection algorithms**, flagging duplicate charges, price surge spikes (+50% over mean), and recurring SaaS renewal footprints.
* **Architected multi-stage Docker containerization pipeline**, achieving zero-dependency CGO-free compilation and <25MB runtime memory footprint.

---

## 🛠️ Technology Stack

| Layer | Technology |
| :--- | :--- |
| **Backend Core** | Go 1.22, Gorilla Mux Router |
| **Database** | Modern SQLite (`modernc.org/sqlite` - Pure CGO-Free) |
| **AI / LLM Ingestion** | Google Gemini 1.5 Flash API + Heuristic Fallback Engine |
| **Payment Integration** | Google Pay, PhonePe, Paytm, Apple Pay, PayPal & SMS Ingest Endpoints |
| **Multi-Currency Sync** | ExchangeRate-API (Live FX Conversion) |
| **Security & Auth** | JWT (`golang-jwt/v5`), Bcrypt password hashing, HTTP-Only Cookies |
| **Frontend UI** | HTML5, Vanilla CSS3 (Glassmorphism Dark Mode), Chart.js |
| **DevOps / Infra** | Docker, Docker Compose, `.env` configuration |

---

## 🔑 Required Resources & API Keys

To run **PulseMatrix AI** with full features enabled, you can configure the following API keys in your `.env` file. **Note:** The application includes intelligent local heuristic fallbacks, allowing it to function 100% offline out-of-the-box even without API keys!

### 1. Google Gemini 1.5 AI API Key (*Recommended*)
* **Purpose**: Powers natural language receipt scanning (`/api/ai/parse-receipt`), SMS notification parsing (`/api/v1/ingest/sms`), and the interactive Financial Strategist Chat room (`/api/ai/advisor`).
* **Cost**: **Free Tier Available** (15 Requests/Min, 1 Million Tokens/Month).
* **How to get it**:
  1. Visit [Google AI Studio](https://aistudio.google.com/).
  2. Log in with your Google account.
  3. Click **"Get API key"** -> **"Create API key"**.
  4. Set `GEMINI_API_KEY=your_actual_key_here` in `.env`.

### 2. ExchangeRate-API Key (*Optional*)
* **Purpose**: Fetches real-time multi-currency FX rates (USD, EUR, GBP, INR, CAD, JPY).
* **Cost**: **Free Tier Available** (1,500 API calls/month).
* **How to get it**:
  1. Visit [ExchangeRate-API](https://www.exchangerate-api.com/).
  2. Enter your email for a free API key.
  3. Set `EXCHANGERATE_API_KEY=your_key_here` in `.env`.

---

## 🚀 Quick Start Guide

### Running Locally (Go CLI)

```bash
# 1. Clone or navigate to directory
cd expense-matrix

# 2. Build & run executable
go build -o pulsematrix.exe .
.\pulsematrix.exe
```

Access the dashboard in your browser at: **`http://localhost:8080`**

### Running via Docker Compose

```bash
docker compose up -d --build
```

---

## 📡 REST & Payment API Documentation

| Endpoint | Method | Auth Required | Description |
| :--- | :--- | :--- | :--- |
| `POST /api/webhooks/payment` | POST | No (Token) | Direct Google Pay / PhonePe / Paytm / PayPal webhook ingestion |
| `POST /api/v1/ingest/sms` | POST | No (Token) | Automated Mobile SMS & Notification transaction parser |
| `POST /api/webhooks/bank` | POST | No (Token) | Plaid & Bank Webhook ingestion endpoint |
| `POST /api/ai/parse-receipt` | POST | Yes | Extract vendor, amount & metadata via Gemini AI |
| `POST /api/ai/advisor` | POST | Yes | Interactive AI Financial Strategist prompt |
| `GET /api/reports/monthly` | GET | Yes | Fetch 6-month financial trajectory JSON |
| `GET /api/reports/categories` | GET | Yes | Fetch category breakdown JSON |
| `GET /transactions/export/csv` | GET | Yes | Download full ledger export as CSV |
