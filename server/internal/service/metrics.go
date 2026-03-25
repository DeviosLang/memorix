package service

import (
	"log/slog"
	"math"
	"sort"
	"strings"
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

	// Agent type tracking
	agentTypes map[string]domain.AgentType // agentID -> type

	// Agent tenant mapping
	agentTenants map[string]string // agentID -> tenantID

	// Activity metrics for timeline (capped at ~7 days of activity)
	activityMetrics []domain.ActivityMetric
	maxActivities   int

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
		agentTypes:          make(map[string]domain.AgentType),
		agentTenants:        make(map[string]string),
		activityMetrics:     make([]domain.ActivityMetric, 0, 50000),
		maxActivities:       50000,
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

		// Track agent tenant mapping
		if metric.TenantID != "" {
			m.agentTenants[metric.AgentID] = metric.TenantID
		}

		// Infer agent type from agent ID pattern
		agentType := inferAgentType(metric.AgentID)
		m.agentTypes[metric.AgentID] = agentType
	}

	// Trim if exceeds max
	if len(m.requestMetrics) > m.maxRequests {
		m.requestMetrics = m.requestMetrics[len(m.requestMetrics)-m.maxRequests:]
	}
}

// RecordActivity records an agent activity event.
func (m *MetricsCollector) RecordActivity(metric domain.ActivityMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activityMetrics = append(m.activityMetrics, metric)

	// Track agent tenant mapping
	if metric.TenantID != "" && metric.AgentID != "" {
		m.agentTenants[metric.AgentID] = metric.TenantID
	}

	// Trim if exceeds max
	if len(m.activityMetrics) > m.maxActivities {
		m.activityMetrics = m.activityMetrics[len(m.activityMetrics)-m.maxActivities:]
	}
}

// inferAgentType determines the agent type from the agent ID.
func inferAgentType(agentID string) domain.AgentType {
	// Common patterns:
	// Claude Code: often uses "claude-code", "claude", or user-defined names
	// OpenClaw: often uses "openclaw" or similar
	// OpenCode: often uses "opencode" or similar
	agentIDLower := strings.ToLower(agentID)

	if strings.Contains(agentIDLower, "claude") {
		return domain.AgentTypeClaudeCode
	}
	if strings.Contains(agentIDLower, "openclaw") {
		return domain.AgentTypeOpenClaw
	}
	if strings.Contains(agentIDLower, "opencode") {
		return domain.AgentTypeOpenCode
	}

	return domain.AgentTypeUnknown
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

// GetAgentActivity returns agent activity data with 7-day timeline.
func (m *MetricsCollector) GetAgentActivity() domain.AgentActivityResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	response := domain.AgentActivityResponse{
		CollectedAt: now,
		ByType:      make(map[string]int),
	}

	// Build activity timeline for each agent
	agentData := make(map[string]*agentActivityBuilder)

	// Process activity metrics from the last 7 days
	cutoff := now.Add(-7 * 24 * time.Hour)
	for _, activity := range m.activityMetrics {
		if activity.Timestamp.Before(cutoff) {
			continue
		}

		if _, exists := agentData[activity.AgentID]; !exists {
			agentData[activity.AgentID] = &agentActivityBuilder{
				writesByDate: make(map[string]int),
				readsByDate:  make(map[string]int),
			}
		}

		dateKey := activity.Timestamp.Format("2006-01-02")
		if activity.Operation == "write" {
			agentData[activity.AgentID].writesByDate[dateKey]++
		} else if activity.Operation == "read" {
			agentData[activity.AgentID].readsByDate[dateKey]++
		}
	}

	// Build final agent list
	for agentID, data := range agentData {
		agentType := m.agentTypes[agentID]
		if agentType == "" {
			agentType = domain.AgentTypeUnknown
		}

		tenantID := m.agentTenants[agentID]
		lastActive, exists := m.activeAgents[agentID]

		activity := domain.AgentActivity{
			AgentID:      agentID,
			AgentType:    string(agentType),
			TenantID:     tenantID,
			LastActiveAt: lastActive,
			Timeline:     data.buildTimeline(now),
		}

		if exists {
			response.Agents = append(response.Agents, activity)
			response.ByType[string(agentType)]++
		}
	}

	response.TotalAgents = len(response.Agents)
	return response
}

// agentActivityBuilder helps build activity timeline for an agent.
type agentActivityBuilder struct {
	writesByDate map[string]int
	readsByDate  map[string]int
}

func (b *agentActivityBuilder) buildTimeline(now time.Time) []domain.ActivityDataPoint {
	timeline := make([]domain.ActivityDataPoint, 7)

	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		dateKey := date.Format("2006-01-02")

		timeline[6-i] = domain.ActivityDataPoint{
			Date:     dateKey,
			Writes:   b.writesByDate[dateKey],
			Reads:    b.readsByDate[dateKey],
			TotalOps: b.writesByDate[dateKey] + b.readsByDate[dateKey],
		}
	}

	return timeline
}

// GetAgentTypes returns the mapping of agent IDs to their types.
func (m *MetricsCollector) GetAgentTypes() map[string]domain.AgentType {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]domain.AgentType, len(m.agentTypes))
	for k, v := range m.agentTypes {
		result[k] = v
	}
	return result
}

// GetAgentTenants returns the mapping of agent IDs to their tenant IDs.
func (m *MetricsCollector) GetAgentTenants() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.agentTenants))
	for k, v := range m.agentTenants {
		result[k] = v
	}
	return result
}

// GetAgentLastSeen returns the last seen time for each agent.
func (m *MetricsCollector) GetAgentLastSeen() map[string]time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]time.Time, len(m.activeAgents))
	for k, v := range m.activeAgents {
		result[k] = v
	}
	return result
}
