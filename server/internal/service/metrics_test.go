package service

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

func TestMetricsCollector_RecordRequest(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record some requests
	for i := 0; i < 10; i++ {
		collector.RecordRequest(domain.RequestMetric{
			Path:       "/v1alpha1/memorix/test/memories",
			Method:     "GET",
			StatusCode: 200,
			LatencyMs:  float64(10 + i),
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
			AgentID:    "agent-1",
		})
	}

	stats := collector.GetRequestStats()
	if stats.TotalRequests != 10 {
		t.Errorf("expected 10 total requests, got %d", stats.TotalRequests)
	}

	if stats.RequestsByPath["/v1alpha1/memorix/test/memories"] != 10 {
		t.Errorf("expected 10 requests by path, got %d", stats.RequestsByPath["/v1alpha1/memorix/test/memories"])
	}

	if stats.RequestsByCode[200] != 10 {
		t.Errorf("expected 10 requests by code, got %d", stats.RequestsByCode[200])
	}
}

func TestMetricsCollector_RecordSearch(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record vector searches
	for i := 0; i < 5; i++ {
		collector.RecordSearch(domain.SearchMetric{
			SearchType: "vector",
			LatencyMs:  float64(50 + i*10),
			Success:    true,
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
		})
	}

	// Record keyword searches
	for i := 0; i < 3; i++ {
		collector.RecordSearch(domain.SearchMetric{
			SearchType: "keyword",
			LatencyMs:  float64(20 + i*5),
			Success:    true,
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
		})
	}

	stats := collector.GetSearchStats()
	if stats.VectorSearches != 5 {
		t.Errorf("expected 5 vector searches, got %d", stats.VectorSearches)
	}

	if stats.KeywordSearches != 3 {
		t.Errorf("expected 3 keyword searches, got %d", stats.KeywordSearches)
	}

	// Check percentages
	expectedVectorPercent := 5.0 / 8.0 * 100
	if stats.VectorSearchPercent < expectedVectorPercent-0.1 || stats.VectorSearchPercent > expectedVectorPercent+0.1 {
		t.Errorf("expected vector percent around %.2f, got %.2f", expectedVectorPercent, stats.VectorSearchPercent)
	}
}

func TestMetricsCollector_RecordConflict(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record LWW resolutions
	for i := 0; i < 7; i++ {
		collector.RecordConflict(domain.ConflictMetric{
			FactID:     "fact-" + string(rune('0'+i)),
			UserID:     "user-1",
			Resolution: "lww",
			Success:    true,
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
		})
	}

	// Record LLM merge resolutions
	for i := 0; i < 3; i++ {
		collector.RecordConflict(domain.ConflictMetric{
			FactID:     "fact-" + string(rune('a'+i)),
			UserID:     "user-2",
			Resolution: "llm_merge",
			Success:    true,
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
		})
	}

	stats := collector.GetConflictStats()
	if stats.TotalConflicts != 10 {
		t.Errorf("expected 10 total conflicts, got %d", stats.TotalConflicts)
	}

	if stats.LWWResolutions != 7 {
		t.Errorf("expected 7 LWW resolutions, got %d", stats.LWWResolutions)
	}

	if stats.LLMMergeResolutions != 3 {
		t.Errorf("expected 3 LLM merge resolutions, got %d", stats.LLMMergeResolutions)
	}
}

func TestMetricsCollector_GetOverview(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record some requests
	collector.RecordRequest(domain.RequestMetric{
		Path:       "/v1alpha1/memorix/test/memories",
		Method:     "POST",
		StatusCode: 201,
		LatencyMs:  15.5,
		Timestamp:  time.Now(),
		TenantID:   "tenant-1",
		AgentID:    "agent-1",
	})

	overview := collector.GetOverview()
	if overview.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", overview.Status)
	}

	if overview.Uptime == "" {
		t.Error("expected uptime to be set")
	}

	if overview.RequestStats.TotalRequests != 1 {
		t.Errorf("expected 1 total request, got %d", overview.RequestStats.TotalRequests)
	}
}

func TestMetricsCollector_Percentiles(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record 100 requests with varying latencies
	for i := 0; i < 100; i++ {
		collector.RecordRequest(domain.RequestMetric{
			Path:       "/test",
			Method:     "GET",
			StatusCode: 200,
			LatencyMs:  float64(i + 1), // 1ms to 100ms
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
			AgentID:    "agent-1",
		})
	}

	stats := collector.GetRequestStats()

	// P50 should be around 50ms
	if stats.P50LatencyMs < 48 || stats.P50LatencyMs > 52 {
		t.Errorf("expected P50 around 50ms, got %.2fms", stats.P50LatencyMs)
	}

	// P95 should be around 95ms
	if stats.P95LatencyMs < 93 || stats.P95LatencyMs > 97 {
		t.Errorf("expected P95 around 95ms, got %.2fms", stats.P95LatencyMs)
	}

	// P99 should be around 99ms
	if stats.P99LatencyMs < 97 || stats.P99LatencyMs > 100 {
		t.Errorf("expected P99 around 99ms, got %.2fms", stats.P99LatencyMs)
	}
}

func TestMetricsCollector_MaxRequests(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)
	collector.maxRequests = 100

	// Record more than max requests
	for i := 0; i < 150; i++ {
		collector.RecordRequest(domain.RequestMetric{
			Path:       "/test",
			Method:     "GET",
			StatusCode: 200,
			LatencyMs:  float64(i),
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
			AgentID:    "agent-1",
		})
	}

	// Total should still be 150 (counter)
	if collector.totalRequests != 150 {
		t.Errorf("expected 150 total requests, got %d", collector.totalRequests)
	}

	// But stored metrics should be trimmed to max
	if len(collector.requestMetrics) != 100 {
		t.Errorf("expected 100 stored metrics, got %d", len(collector.requestMetrics))
	}
}

func TestMetricsCollector_ActiveAgents(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record requests from multiple agents
	for i := 0; i < 5; i++ {
		collector.RecordRequest(domain.RequestMetric{
			Path:       "/test",
			Method:     "GET",
			StatusCode: 200,
			LatencyMs:  10,
			Timestamp:  time.Now(),
			TenantID:   "tenant-1",
			AgentID:    "agent-" + string(rune('0'+i)),
		})
	}

	activeAgents := collector.GetActiveAgents()
	if activeAgents != 5 {
		t.Errorf("expected 5 active agents, got %d", activeAgents)
	}
}

func TestMetricsCollector_TenantRequestCounts(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	collector := NewMetricsCollector(logger)

	// Record requests for multiple tenants
	collector.RecordRequest(domain.RequestMetric{
		Path:       "/test",
		Method:     "GET",
		StatusCode: 200,
		LatencyMs:  10,
		Timestamp:  time.Now(),
		TenantID:   "tenant-1",
	})

	collector.RecordRequest(domain.RequestMetric{
		Path:       "/test",
		Method:     "GET",
		StatusCode: 200,
		LatencyMs:  10,
		Timestamp:  time.Now(),
		TenantID:   "tenant-2",
	})

	collector.RecordRequest(domain.RequestMetric{
		Path:       "/test",
		Method:     "GET",
		StatusCode: 200,
		LatencyMs:  10,
		Timestamp:  time.Now(),
		TenantID:   "tenant-1",
	})

	counts := collector.GetTenantRequestCounts()
	if counts["tenant-1"] != 2 {
		t.Errorf("expected 2 requests for tenant-1, got %d", counts["tenant-1"])
	}

	if counts["tenant-2"] != 1 {
		t.Errorf("expected 1 request for tenant-2, got %d", counts["tenant-2"])
	}
}
