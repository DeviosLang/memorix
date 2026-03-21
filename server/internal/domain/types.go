package domain

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MemoryType classifies how a memory was created.
type MemoryType string

const (
	TypePinned  MemoryType = "pinned"
	TypeInsight MemoryType = "insight"
)

// MemoryState represents the lifecycle state of a memory.
type MemoryState string

const (
	StateActive   MemoryState = "active"
	StatePaused   MemoryState = "paused"
	StateArchived MemoryState = "archived"
	StateDeleted  MemoryState = "deleted"
	StateStale    MemoryState = "stale" // GC: memory marked for cleanup due to time decay
)

// Memory represents a piece of shared knowledge stored in a space.
type Memory struct {
	ID         string          `json:"id"`
	Content    string          `json:"content"`
	MemoryType MemoryType      `json:"memory_type"`
	Source     string          `json:"source,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	Embedding  []float32       `json:"-"`

	AgentID      string `json:"agent_id,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	UpdatedBy    string `json:"updated_by,omitempty"`
	SupersededBy string `json:"superseded_by,omitempty"`

	State     MemoryState `json:"state"`
	Version   int         `json:"version"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`

	Score *float64 `json:"score,omitempty"`

	// GC (Memory Garbage Collection) fields
	Confidence      float64    `json:"confidence,omitempty"`
	AccessCount     int        `json:"access_count,omitempty"`
	LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty"`
	ImportanceScore *float64   `json:"importance_score,omitempty"`
}

type AuthInfo struct {
	AgentName string

	// Dedicated-cluster model (non-empty when using tenant token)
	TenantID string
	TenantDB *sql.DB
}

// MemoryFilter encapsulates search/list query parameters.
type MemoryFilter struct {
	Query      string
	Tags       []string
	Source     string
	State      string
	MemoryType string
	AgentID    string
	SessionID  string
	Limit      int
	Offset     int
	MinScore   float64 // minimum cosine similarity for vector results; 0 = use default (0.3); -1 = disabled (return all)
}

// TenantStatus represents the lifecycle status of a tenant.
type TenantStatus string

const (
	TenantProvisioning TenantStatus = "provisioning"
	TenantActive       TenantStatus = "active"
	TenantSuspended    TenantStatus = "suspended"
	TenantDeleted      TenantStatus = "deleted"
)

// Tenant represents a provisioned customer with a dedicated TiDB cluster.
type Tenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Connection info (never exposed in API responses)
	DBHost     string `json:"-"`
	DBPort     int    `json:"-"`
	DBUser     string `json:"-"`
	DBPassword string `json:"-"`
	DBName     string `json:"-"`
	DBTLS      bool   `json:"-"`

	// Provisioning metadata
	Provider       string     `json:"provider"`
	ClusterID      string     `json:"cluster_id,omitempty"`
	ClaimURL       string     `json:"-"`
	ClaimExpiresAt *time.Time `json:"-"`

	// Lifecycle
	Status        TenantStatus `json:"status"`
	SchemaVersion int          `json:"schema_version"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	DeletedAt     *time.Time   `json:"-"`
}

// DSN builds a MySQL connection string for this tenant's database.
func (t *Tenant) DSN() string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		t.DBUser, t.DBPassword, t.DBHost, t.DBPort, t.DBName)
	if t.DBTLS {
		dsn += "&tls=true"
	}
	return dsn
}

// TenantToken represents an API token bound to a tenant.
type TenantToken struct {
	APIToken  string    `json:"api_token"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TenantInfo describes tenant metadata.
type TenantInfo struct {
	TenantID    string       `json:"tenant_id"`
	Name        string       `json:"name"`
	Status      TenantStatus `json:"status"`
	Provider    string       `json:"provider"`
	MemoryCount int          `json:"memory_count"`
	CreatedAt   time.Time    `json:"created_at"`
}

// DeletionReason represents why a memory was garbage collected.
type DeletionReason string

const (
	DeletionReasonStale         DeletionReason = "stale"          // Time decay: not accessed for too long
	DeletionReasonLowImportance DeletionReason = "low_importance" // Low importance score
	DeletionReasonCapacity      DeletionReason = "capacity"       // Capacity limit exceeded
	DeletionReasonManual        DeletionReason = "manual"         // Manual GC trigger
)

// MemoryGCLog represents an audit log entry for memory garbage collection.
type MemoryGCLog struct {
	LogID           string         `json:"log_id"`
	MemoryID        string         `json:"memory_id"`
	TenantID        string         `json:"tenant_id"`
	ContentPreview  string         `json:"content_preview,omitempty"`
	Source          string         `json:"source,omitempty"`
	MemoryType      MemoryType     `json:"memory_type,omitempty"`
	State           MemoryState    `json:"state"`
	Confidence      float64        `json:"confidence,omitempty"`
	AccessCount     int            `json:"access_count,omitempty"`
	LastAccessedAt  *time.Time     `json:"last_accessed_at,omitempty"`
	ImportanceScore *float64       `json:"importance_score,omitempty"`
	DeletionReason  DeletionReason `json:"deletion_reason"`
	GCRunID         string         `json:"gc_run_id"`
	CreatedAt       time.Time      `json:"created_at"`
}

// MemoryGCSnapshot represents a pre-deletion backup for recovery.
type MemoryGCSnapshot struct {
	SnapshotID  string    `json:"snapshot_id"`
	GCRunID     string    `json:"gc_run_id"`
	TenantID    string    `json:"tenant_id"`
	MemoryCount int       `json:"memory_count"`
	SnapshotData string   `json:"-"` // Not exposed in JSON
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// GCConfig holds configuration for memory garbage collection.
type GCConfig struct {
	// Enabled controls whether GC runs automatically.
	Enabled bool

	// Interval is the time between GC runs (default: 24h for daily runs).
	Interval time.Duration

	// StaleThreshold is the duration after which unaccessed memories become stale.
	// Default: 90 days.
	StaleThreshold time.Duration

	// LowConfidenceThreshold is the confidence below which memories are candidates for cleanup.
	// Default: 0.5.
	LowConfidenceThreshold float64

	// MaxMemoriesPerTenant is the capacity limit per tenant.
	// When exceeded, lowest importance memories are cleaned up.
	// Default: 10000.
	MaxMemoriesPerTenant int

	// SnapshotRetentionDays is how long to keep GC snapshots for recovery.
	// Default: 30 days.
	SnapshotRetentionDays int

	// BatchSize is the number of memories to process per GC iteration.
	// Default: 100.
	BatchSize int
}

// GCResult contains the results of a GC run.
type GCResult struct {
	GCRunID            string `json:"gc_run_id"`
	TenantID           string `json:"tenant_id"`
	StaleMarked        int    `json:"stale_marked"`
	LowImportanceCount int    `json:"low_importance_count"`
	CapacityCleaned    int    `json:"capacity_cleaned"`
	TotalDeleted       int    `json:"total_deleted"`
	SnapshotCreated    bool   `json:"snapshot_created"`
	SnapshotID         string `json:"snapshot_id,omitempty"`
	DryRun             bool   `json:"dry_run"`
}

// GCPreview contains preview information for dry-run mode.
type GCPreview struct {
	TenantID           string         `json:"tenant_id"`
	StaleMemories      []Memory       `json:"stale_memories"`
	LowImportance      []Memory       `json:"low_importance"`
	OverCapacity       []Memory       `json:"over_capacity"`
	TotalToDelete      int            `json:"total_to_delete"`
	StaleThreshold     time.Duration  `json:"stale_threshold"`
	CurrentMemoryCount int            `json:"current_memory_count"`
	MaxMemories        int            `json:"max_memories"`
}
