package service

import (
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// MetricsCollector collects and aggregates system metrics for dashboard display.
type MetricsCollector struct {
	mu sync.RWMutex

	// Server start time for uptime calculation
	startTime time.Time

	// Request metrics
	requestMetrics []domain.RequestMetric
	maxRequests    int // Maximum number of requests to keep in memory

	// Search metrics
	searchMetrics []domain.SearchMetric
	maxSearches   int

	// Conflict metrics
	conflictMetrics []domain.ConflictMetric
	maxConflicts    int

	// Counters
	totalRequests   int64
	totalSearches   int64
	totalConflicts  int64

	// Per-tenant request counts
	tenantRequestCounts map[string]int64

	// Agent tracking (active in last 24h)
	activeAgents map[string]time.Time // agentID -> last seen time

	logger *slog.Logger
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(logger *slog.Logger) *MetricsCollector {
	return &MetricsCollector{
		startTime:           time.Now(),
		requestMetrics:      make([]domain.RequestMetric, 0, 10000),
		maxRequests:         10000,
		searchMetrics:       make([]domain.SearchMetric, 0, 5000),
		maxSearches:         5000,
		conflictMetrics:     make([]domain.ConflictMetric, 0, 1000),
		maxConflicts:        1000,
		tenantRequestCounts: make(map[string]int64),
		activeAgents:        make(map[string]time.Time),
		logger:              logger,
	}
}

// RecordRequest records a request metric.
func (m *MetricsCollector) RecordRequest(metric domain.RequestMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests++
	m.requestMetrics = append(m.requestMetrics, metric)

	// Track tenant requests
	if metric.TenantID != "" {
		m.tenantRequestCounts[metric.TenantID]++
	}

	// Track active agents
	if metric.AgentID != "" {
		m.activeAgents[metric.AgentID] = time.Now()
	}

	// Trim if exceeds max
	if len(m.requestMetrics) > m.maxRequests {
		m.requestMetrics = m.requestMetrics[len(m.requestMetrics)-m.maxRequests:]
	}
}

// RecordSearch records a search metric.
func (m *MetricsCollector) RecordSearch(metric domain.SearchMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalSearches++
	m.searchMetrics = append(m.searchMetrics, metric)

	// Trim if exceeds max
	if len(m.searchMetrics) > m.maxSearches {
		m.searchMetrics = m.searchMetrics[len(m.searchMetrics)-m.maxSearches:]
	}
}

// RecordConflict records a conflict resolution metric.
func (m *MetricsCollector) RecordConflict(metric domain.ConflictMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalConflicts++
	m.conflictMetrics = append(m.conflictMetrics, metric)

	// Trim if exceeds max
	if len(m.conflictMetrics) > m.maxConflicts {
		m.conflictMetrics = m.conflictMetrics[len(m.conflictMetrics)-m.maxConflicts:]
	}
}

// GetOverview returns the dashboard overview metrics.
func (m *MetricsCollector) GetOverview() domain.DashboardOverview {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	uptime := now.Sub(m.startTime)

	// Calculate request stats
	requestStats := m.calculateRequestStatsLocked()

	// Clean up stale agents (older than 24h)
	activeAgents := 0
	cutoff := now.Add(-24 * time.Hour)
	for _, lastSeen := range m.activeAgents {
		if lastSeen.After(cutoff) {
			activeAgents++
		}
	}

	return domain.DashboardOverview{
		Status:        "healthy",
		StartTime:     m.startTime,
		Uptime:        uptime.Round(time.Second).String(),
		RequestStats:  requestStats,
		ActiveAgents:  activeAgents,
		CollectedAt:   now,
	}
}

// GetRequestStats returns request statistics.
func (m *MetricsCollector) GetRequestStats() domain.RequestStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calculateRequestStatsLocked()
}

func (m *MetricsCollector) calculateRequestStatsLocked() domain.RequestStats {
	if len(m.requestMetrics) == 0 {
		return domain.RequestStats{
			RequestsByPath: make(map[string]int64),
			RequestsByCode: make(map[int]int64),
		}
	}

	stats := domain.RequestStats{
		TotalRequests:  m.totalRequests,
		RequestsByPath: make(map[string]int64),
		RequestsByCode: make(map[int]int64),
	}

	// Calculate distributions and collect latencies
	latencies := make([]float64, 0, len(m.requestMetrics))
	var totalLatency float64
	var errorCount int64

	for _, req := range m.requestMetrics {
		stats.RequestsByPath[req.Path]++
		stats.RequestsByCode[req.StatusCode]++
		latencies = append(latencies, req.LatencyMs)
		totalLatency += req.LatencyMs

		if req.StatusCode >= 400 {
			errorCount++
		}
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		stats.AvgLatencyMs = totalLatency / float64(len(latencies))
		stats.P50LatencyMs = percentile(latencies, 50)
		stats.P95LatencyMs = percentile(latencies, 95)
		stats.P99LatencyMs = percentile(latencies, 99)
	}

	// Calculate rates
	uptimeSeconds := time.Since(m.startTime).Seconds()
	if uptimeSeconds > 0 {
		stats.RequestsPerSec = float64(m.totalRequests) / uptimeSeconds
	}

	if m.totalRequests > 0 {
		stats.ErrorRate = float64(errorCount) / float64(m.totalRequests)
	}

	return stats
}

// GetSearchStats returns search statistics.
func (m *MetricsCollector) GetSearchStats() domain.DashboardSearchStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	stats := domain.DashboardSearchStats{
		CollectedAt: now,
	}

	if len(m.searchMetrics) == 0 {
		return stats
	}

	latencies := make([]float64, 0, len(m.searchMetrics))
	var totalLatency float64
	var successCount int64

	for _, s := range m.searchMetrics {
		switch s.SearchType {
		case "vector":
			stats.VectorSearches++
		case "keyword":
			stats.KeywordSearches++
		case "hybrid":
			stats.HybridSearches++
		case "fts":
			stats.FTSSearches++
		}

		latencies = append(latencies, s.LatencyMs)
		totalLatency += s.LatencyMs
		if s.Success {
			successCount++
		}
	}

	// Calculate percentages
	total := stats.VectorSearches + stats.KeywordSearches + stats.HybridSearches + stats.FTSSearches
	if total > 0 {
		stats.VectorSearchPercent = float64(stats.VectorSearches) / float64(total) * 100
		stats.KeywordSearchPercent = float64(stats.KeywordSearches) / float64(total) * 100
		stats.HybridSearchPercent = float64(stats.HybridSearches) / float64(total) * 100
		stats.FTSSearchPercent = float64(stats.FTSSearches) / float64(total) * 100
		stats.SuccessRate = float64(successCount) / float64(total)
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		stats.AvgSearchLatencyMs = totalLatency / float64(len(latencies))
		stats.P50SearchLatencyMs = percentile(latencies, 50)
		stats.P95SearchLatencyMs = percentile(latencies, 95)
		stats.P99SearchLatencyMs = percentile(latencies, 99)
	}

	return stats
}

// GetConflictStats returns conflict resolution statistics.
func (m *MetricsCollector) GetConflictStats() domain.DashboardConflictStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	stats := domain.DashboardConflictStats{
		CollectedAt: now,
	}

	if len(m.conflictMetrics) == 0 {
		return stats
	}

	var lwwCount, llmCount int64
	var successCount int64

	// Get last 10 conflicts for summary
	recentIdx := len(m.conflictMetrics) - 10
	if recentIdx < 0 {
		recentIdx = 0
	}

	for i, c := range m.conflictMetrics {
		switch c.Resolution {
		case "lww":
			lwwCount++
		case "llm_merge":
			llmCount++
		}

		if c.Success {
			successCount++
		}

		// Add to recent if in range
		if i >= recentIdx {
			stats.RecentConflicts = append(stats.RecentConflicts, domain.ConflictSummary{
				FactID:     c.FactID,
				UserID:     c.UserID,
				Resolution: c.Resolution,
				ResolvedAt: c.Timestamp,
			})
		}
	}

	stats.TotalConflicts = m.totalConflicts
	stats.LWWResolutions = lwwCount
	stats.LLMMergeResolutions = llmCount

	if stats.TotalConflicts > 0 {
		stats.LWWPercent = float64(lwwCount) / float64(stats.TotalConflicts) * 100
		stats.LLMMergePercent = float64(llmCount) / float64(stats.TotalConflicts) * 100
		stats.MergeSuccessRate = float64(successCount) / float64(stats.TotalConflicts)
	}

	return stats
}

// GetTenantRequestCounts returns per-tenant request counts.
func (m *MetricsCollector) GetTenantRequestCounts() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64, len(m.tenantRequestCounts))
	for k, v := range m.tenantRequestCounts {
		result[k] = v
	}
	return result
}

// GetActiveAgents returns the count of active agents (seen in last 24h).
func (m *MetricsCollector) GetActiveAgents() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, lastSeen := range m.activeAgents {
		if lastSeen.After(cutoff) {
			count++
		}
	}
	return count
}

// percentile calculates the percentile of a sorted slice of values.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Make a copy to avoid modifying the original
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
