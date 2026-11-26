package services

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsService collects real-time backend telemetry (latency percentiles, throughput,
// runtime heap/goroutines, and domain counters) without external dependencies.
type MetricsService struct {
	startedAt       time.Time
	totalRequests   atomic.Int64
	errorRequests   atomic.Int64
	rateLimited     atomic.Int64
	webhooksIngest  atomic.Int64
	workerRuns      atomic.Int64
	anomaliesTagged atomic.Int64

	latMu     sync.Mutex
	latencies []float64 // rolling window of last 500 request durations (ms)
}

func NewMetricsService() *MetricsService {
	return &MetricsService{
		startedAt: time.Now(),
		latencies: make([]float64, 0, 500),
	}
}

func (m *MetricsService) RecordRequest(status int, duration time.Duration) {
	m.totalRequests.Add(1)
	if status >= 500 {
		m.errorRequests.Add(1)
	}
	if status == 429 {
		m.rateLimited.Add(1)
	}
	ms := float64(duration.Microseconds()) / 1000.0
	m.latMu.Lock()
	if len(m.latencies) >= 500 {
		m.latencies = m.latencies[1:]
	}
	m.latencies = append(m.latencies, ms)
	m.latMu.Unlock()
}

func (m *MetricsService) IncWebhook()            { m.webhooksIngest.Add(1) }
func (m *MetricsService) IncWorkerRun()          { m.workerRuns.Add(1) }
func (m *MetricsService) AddAnomalies(n int64)   { m.anomaliesTagged.Add(n) }

type MetricsSnapshot struct {
	UptimeSeconds   int64   `json:"uptimeSeconds"`
	UptimeHuman     string  `json:"uptimeHuman"`
	TotalRequests   int64   `json:"totalRequests"`
	ErrorRequests   int64   `json:"errorRequests"`
	RateLimited     int64   `json:"rateLimited"`
	WebhooksIngest  int64   `json:"webhooksIngested"`
	WorkerRuns      int64   `json:"backgroundWorkerRuns"`
	AnomaliesTagged int64   `json:"anomaliesFlaggedByWorker"`
	AvgLatencyMs    float64 `json:"avgLatencyMs"`
	P95LatencyMs    float64 `json:"p95LatencyMs"`
	Goroutines      int     `json:"goroutines"`
	HeapAllocMB     float64 `json:"heapAllocMB"`
	NumGC           uint32  `json:"numGC"`
}

func (m *MetricsService) Snapshot() MetricsSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	m.latMu.Lock()
	cp := make([]float64, len(m.latencies))
	copy(cp, m.latencies)
	m.latMu.Unlock()

	var avg, p95 float64
	if len(cp) > 0 {
		var sum float64
		for _, v := range cp {
			sum += v
		}
		avg = sum / float64(len(cp))
		sort.Float64s(cp)
		idx := int(float64(len(cp)-1) * 0.95)
		p95 = cp[idx]
	}

	uptime := time.Since(m.startedAt)
	return MetricsSnapshot{
		UptimeSeconds:   int64(uptime.Seconds()),
		UptimeHuman:     uptime.Truncate(time.Second).String(),
		TotalRequests:   m.totalRequests.Load(),
		ErrorRequests:   m.errorRequests.Load(),
		RateLimited:     m.rateLimited.Load(),
		WebhooksIngest:  m.webhooksIngest.Load(),
		WorkerRuns:      m.workerRuns.Load(),
		AnomaliesTagged: m.anomaliesTagged.Load(),
		AvgLatencyMs:    float64(int(avg*100)) / 100,
		P95LatencyMs:    float64(int(p95*100)) / 100,
		Goroutines:      runtime.NumGoroutine(),
		HeapAllocMB:     float64(int((float64(mem.HeapAlloc)/(1024*1024))*100)) / 100,
		NumGC:           mem.NumGC,
	}
}

// PrometheusText exports metrics in Prometheus text exposition format.
func (m *MetricsService) PrometheusText() string {
	s := m.Snapshot()
	return fmt.Sprintf(
		"# HELP pulsematrix_http_requests_total Total HTTP requests handled\n"+
			"# TYPE pulsematrix_http_requests_total counter\n"+
			"pulsematrix_http_requests_total %d\n"+
			"# HELP pulsematrix_http_errors_total Total 5xx HTTP responses\n"+
			"# TYPE pulsematrix_http_errors_total counter\n"+
			"pulsematrix_http_errors_total %d\n"+
			"# HELP pulsematrix_rate_limited_total Total 429 rate-limited requests\n"+
			"# TYPE pulsematrix_rate_limited_total counter\n"+
			"pulsematrix_rate_limited_total %d\n"+
			"# HELP pulsematrix_http_latency_p95_ms 95th percentile request latency in ms\n"+
			"# TYPE pulsematrix_http_latency_p95_ms gauge\n"+
			"pulsematrix_http_latency_p95_ms %.2f\n"+
			"# HELP pulsematrix_runtime_goroutines Active Go goroutines\n"+
			"# TYPE pulsematrix_runtime_goroutines gauge\n"+
			"pulsematrix_runtime_goroutines %d\n"+
			"# HELP pulsematrix_runtime_heap_mb Allocated heap memory in MB\n"+
			"# TYPE pulsematrix_runtime_heap_mb gauge\n"+
			"pulsematrix_runtime_heap_mb %.2f\n"+
			"# HELP pulsematrix_worker_runs_total Background worker cycles completed\n"+
			"# TYPE pulsematrix_worker_runs_total counter\n"+
			"pulsematrix_worker_runs_total %d\n",
		s.TotalRequests, s.ErrorRequests, s.RateLimited, s.P95LatencyMs, s.Goroutines, s.HeapAllocMB, s.WorkerRuns,
	)
}
