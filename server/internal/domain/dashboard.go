package domain

import (
	"time"
)

// Dashboard types for metrics collection and API responses.

// DashboardOverview provides a system-wide overview.
type DashboardOverview struct {
	// System status
	Status    string    `json:"status"`     // "healthy", "degraded", "unhealthy"
	StartTime time.Time `json:"start_time"` // Server start time
	Uptime    string    `json:"uptime"`     // Human-readable uptime duration

	// Request statistics
	RequestStats RequestStats `json:"request_stats"`

	// Active resources
	ActiveTenants int `json:"active_tenants"`
	ActiveAgents  int `json:"active_agents"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// RequestStats contains request-level metrics.
type RequestStats struct {
	TotalRequests   int64            `json:"total_requests"`
	RequestsByPath  map[string]int64 `json:"requests_by_path"`
	RequestsByCode  map[int]int64    `json:"requests_by_code"`
	AvgLatencyMs    float64          `json:"avg_latency_ms"`
	P50LatencyMs    float64          `json:"p50_latency_ms"`
	P95LatencyMs    float64          `json:"p95_latency_ms"`
	P99LatencyMs    float64          `json:"p99_latency_ms"`
	RequestsPerSec  float64          `json:"requests_per_sec"`
	ErrorRate       float64          `json:"error_rate"` // 0.0 to 1.0
}

// DashboardMemoryStats provides memory-related statistics.
type DashboardMemoryStats struct {
	// Total counts
	TotalMemories int `json:"total_memories"`

	// Distribution by state
	ByState map[string]int `json:"by_state"`

	// Distribution by type
	ByType map[string]int `json:"by_type"`

	// Storage metrics
	TotalContentBytes int64   `json:"total_content_bytes"`
	AvgContentBytes   float64 `json:"avg_content_bytes"`

	// Per-tenant breakdown (top tenants)
	TopTenants []TenantMemoryCount `json:"top_tenants,omitempty"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// TenantMemoryCount shows memory count for a tenant.
type TenantMemoryCount struct {
	TenantID    string `json:"tenant_id"`
	TenantName  string `json:"tenant_name,omitempty"`
	MemoryCount int    `json:"memory_count"`
}

// DashboardSearchStats provides search performance metrics.
type DashboardSearchStats struct {
	// Search counts by type
	VectorSearches int64 `json:"vector_searches"`
	KeywordSearches int64 `json:"keyword_searches"`
	HybridSearches  int64 `json:"hybrid_searches"`
	FTSSearches     int64 `json:"fts_searches"`

	// Search distribution (percentage of each type)
	VectorSearchPercent  float64 `json:"vector_search_percent"`
	KeywordSearchPercent float64 `json:"keyword_search_percent"`
	HybridSearchPercent  float64 `json:"hybrid_search_percent"`
	FTSSearchPercent     float64 `json:"fts_search_percent"`

	// Latency metrics
	AvgSearchLatencyMs float64 `json:"avg_search_latency_ms"`
	P50SearchLatencyMs float64 `json:"p50_search_latency_ms"`
	P95SearchLatencyMs float64 `json:"p95_search_latency_ms"`
	P99SearchLatencyMs float64 `json:"p99_search_latency_ms"`

	// Success rate
	SuccessRate float64 `json:"success_rate"` // 0.0 to 1.0

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// DashboardGCStats provides garbage collection statistics.
type DashboardGCStats struct {
	// Last run info
	LastRunTime      *time.Time `json:"last_run_time,omitempty"`
	LastRunID        string     `json:"last_run_id,omitempty"`
	LastRunDeleted   int        `json:"last_run_deleted"`
	LastRunDuration  string     `json:"last_run_duration,omitempty"`

	// Historical totals
	TotalRuns      int64 `json:"total_runs"`
	TotalDeleted   int64 `json:"total_deleted"`
	TotalRecovered int64 `json:"total_recovered"`

	// Next scheduled run
	NextScheduledTime *time.Time `json:"next_scheduled_time,omitempty"`

	// Recent GC logs (last 10 runs)
	RecentRuns []GCSummary `json:"recent_runs,omitempty"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// GCSummary is a summary of a GC run.
type GCSummary struct {
	GCRunID      string     `json:"gc_run_id"`
	RunTime      time.Time  `json:"run_time"`
	DeletedCount int        `json:"deleted_count"`
	Duration     string     `json:"duration"`
	Reason       string     `json:"reason"`
}

// DashboardSpaceStats provides tenant and agent statistics.
type DashboardSpaceStats struct {
	// Tenant stats
	TotalTenants   int `json:"total_tenants"`
	ActiveTenants  int `json:"active_tenants"`
	SuspendedTenants int `json:"suspended_tenants"`

	// Agent stats
	TotalAgents    int            `json:"total_agents"`
	ActiveAgents   int            `json:"active_agents"` // Agents active in last 24h
	AgentsByTenant map[string]int `json:"agents_by_tenant,omitempty"`

	// Top tenants by activity
	TopActiveTenants []TenantActivity `json:"top_active_tenants,omitempty"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// TenantActivity shows activity metrics for a tenant.
type TenantActivity struct {
	TenantID        string `json:"tenant_id"`
	TenantName      string `json:"tenant_name,omitempty"`
	AgentCount      int    `json:"agent_count"`
	MemoryCount     int    `json:"memory_count"`
	RequestCount    int64  `json:"request_count"`
	LastActivityAt  string `json:"last_activity_at,omitempty"`
}

// DashboardConflictStats provides conflict resolution statistics.
type DashboardConflictStats struct {
	// Resolution counts
	TotalConflicts    int64 `json:"total_conflicts"`
	LWWResolutions    int64 `json:"lww_resolutions"`    // Last-Write-Wins
	LLMMergeResolutions int64 `json:"llm_merge_resolutions"` // LLM-based merge

	// Resolution distribution
	LWWPercent    float64 `json:"lww_percent"`
	LLMMergePercent float64 `json:"llm_merge_percent"`

	// Success metrics
	MergeSuccessRate float64 `json:"merge_success_rate"` // 0.0 to 1.0

	// Recent conflicts (sample)
	RecentConflicts []ConflictSummary `json:"recent_conflicts,omitempty"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// ConflictSummary is a summary of a conflict resolution.
type ConflictSummary struct {
	FactID      string    `json:"fact_id"`
	UserID      string    `json:"user_id"`
	Resolution  string    `json:"resolution"` // "lww" or "llm_merge"
	ResolvedAt  time.Time `json:"resolved_at"`
}

// RequestMetric is a single request metric data point.
type RequestMetric struct {
	Path       string
	Method     string
	StatusCode int
	LatencyMs  float64
	Timestamp  time.Time
	TenantID   string
	AgentID    string
}

// SearchMetric is a single search metric data point.
type SearchMetric struct {
	SearchType string // "vector", "keyword", "hybrid", "fts"
	LatencyMs  float64
	Success    bool
	Timestamp  time.Time
	TenantID   string
}

// ConflictMetric is a single conflict resolution metric.
type ConflictMetric struct {
	FactID     string
	UserID     string
	Resolution string // "lww" or "llm_merge"
	Success    bool
	Timestamp  time.Time
	TenantID   string
}
