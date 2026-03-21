package repository

import (
	"context"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// MemoryRepo defines storage operations for memories.
type MemoryRepo interface {
	Create(ctx context.Context, m *domain.Memory) error
	GetByID(ctx context.Context, id string) (*domain.Memory, error)
	UpdateOptimistic(ctx context.Context, m *domain.Memory, expectedVersion int) error
	SoftDelete(ctx context.Context, id, agentName string) error
	ArchiveMemory(ctx context.Context, id, supersededBy string) error
	ArchiveAndCreate(ctx context.Context, archiveID, supersededBy string, newMem *domain.Memory) error
	SetState(ctx context.Context, id string, state domain.MemoryState) error
	List(ctx context.Context, f domain.MemoryFilter) (memories []domain.Memory, total int, err error)
	Count(ctx context.Context) (int, error)
	BulkCreate(ctx context.Context, memories []*domain.Memory) error

	// VectorSearch performs ANN search using cosine distance with a pre-computed vector.
	VectorSearch(ctx context.Context, queryVec []float32, f domain.MemoryFilter, limit int) ([]domain.Memory, error)

	// AutoVectorSearch performs ANN search using VEC_EMBED_COSINE_DISTANCE with a plain-text query.
	// TiDB Serverless auto-embeds the query text.
	AutoVectorSearch(ctx context.Context, queryText string, f domain.MemoryFilter, limit int) ([]domain.Memory, error)

	KeywordSearch(ctx context.Context, query string, f domain.MemoryFilter, limit int) ([]domain.Memory, error)

	// FTSSearch performs full-text search using FTS_MATCH_WORD with BM25 ranking.
	// Results include a fts_score field used for RRF merge.
	FTSSearch(ctx context.Context, query string, f domain.MemoryFilter, limit int) ([]domain.Memory, error)
	// FTSAvailable reports whether full-text search is usable on this database.
	FTSAvailable() bool

	ListBootstrap(ctx context.Context, limit int) ([]domain.Memory, error)
}

// TenantRepo manages tenant records in the control plane DB.
type TenantRepo interface {
	Create(ctx context.Context, t *domain.Tenant) error
	GetByID(ctx context.Context, id string) (*domain.Tenant, error)
	GetByName(ctx context.Context, name string) (*domain.Tenant, error)
	UpdateStatus(ctx context.Context, id string, status domain.TenantStatus) error
	UpdateSchemaVersion(ctx context.Context, id string, version int) error
}

// TenantTokenRepo manages tenant API tokens.
type TenantTokenRepo interface {
	CreateToken(ctx context.Context, tt *domain.TenantToken) error
	GetByToken(ctx context.Context, token string) (*domain.TenantToken, error)
	ListByTenant(ctx context.Context, tenantID string) ([]domain.TenantToken, error)
}

// UploadTaskRepo manages upload task records in the control plane DB.
type UploadTaskRepo interface {
	Create(ctx context.Context, task *domain.UploadTask) error
	GetByID(ctx context.Context, taskID string) (*domain.UploadTask, error)
	ListByTenant(ctx context.Context, tenantID string) ([]domain.UploadTask, error)
	UpdateStatus(ctx context.Context, taskID string, status domain.TaskStatus, errorMsg string) error
	UpdateProgress(ctx context.Context, taskID string, doneChunks int) error
	UpdateTotalChunks(ctx context.Context, taskID string, totalChunks int) error
	FetchPending(ctx context.Context, limit int) ([]domain.UploadTask, error)
	ResetProcessing(ctx context.Context, staleTimeout time.Duration) (int64, error)
}

// UserProfileFactRepo manages user profile facts (structured long-term facts about users).
// This implements the "third layer" of ChatGPT's memory system - structured user profile storage.
type UserProfileFactRepo interface {
	Create(ctx context.Context, fact *domain.UserProfileFact) error
	GetByID(ctx context.Context, factID string) (*domain.UserProfileFact, error)
	GetByUserID(ctx context.Context, userID string) ([]domain.UserProfileFact, error)
	GetByUserIDAndCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error)
	List(ctx context.Context, f domain.UserProfileFactFilter) (facts []domain.UserProfileFact, total int, err error)
	Update(ctx context.Context, fact *domain.UserProfileFact) error
	Delete(ctx context.Context, factID string) error
	DeleteByUserID(ctx context.Context, userID string) error

	// CountByUserID returns the total number of facts for a user.
	CountByUserID(ctx context.Context, userID string) (int, error)

	// DeleteOldestLowConfidence deletes facts that are oldest and have lowest confidence.
	// Used when user exceeds capacity limit. Returns number of facts deleted.
	DeleteOldestLowConfidence(ctx context.Context, userID string, count int) (int64, error)

	// TouchLastAccessed updates the last_accessed_at timestamp for a fact.
	TouchLastAccessed(ctx context.Context, factID string) error

	// GetByKey retrieves a fact by user_id, category, and key.
	// Returns ErrNotFound if no matching fact exists.
	GetByKey(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error)

	// SearchByValue performs a fuzzy search for facts with similar values.
	// Returns facts where the value is similar to the query (used for deduplication).
	SearchByValue(ctx context.Context, userID string, value string, limit int) ([]domain.UserProfileFact, error)
}

// ReconcileAuditRepo manages audit logs for memory reconciliation decisions.
// Every time the LLM reconciler makes a decision, an audit log is created
// for traceability and debugging.
type ReconcileAuditRepo interface {
	// Create stores a new reconciliation audit log.
	Create(ctx context.Context, log *domain.ReconcileAuditLog) error

	// GetByID retrieves an audit log by its ID.
	GetByID(ctx context.Context, logID string) (*domain.ReconcileAuditLog, error)

	// ListByUserID retrieves all audit logs for a user, ordered by created_at desc.
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]domain.ReconcileAuditLog, error)

	// ListByFactID retrieves all audit logs for a specific fact, ordered by created_at desc.
	ListByFactID(ctx context.Context, factID string, limit, offset int) ([]domain.ReconcileAuditLog, error)

	// List returns audit logs based on filter criteria.
	List(ctx context.Context, f domain.ReconcileAuditFilter) (logs []domain.ReconcileAuditLog, total int, err error)

	// DeleteByUserID deletes all audit logs for a user (used when user is deleted).
	DeleteByUserID(ctx context.Context, userID string) error
}

// ConversationSummaryRepo manages conversation summaries.
// This implements the "fourth layer" of ChatGPT's memory system - recent conversation summaries.
// Key feature: sliding window of 15-20 summaries per user with zero retrieval latency.
type ConversationSummaryRepo interface {
	// Create stores a new conversation summary.
	Create(ctx context.Context, summary *domain.ConversationSummary) error

	// GetByID retrieves a summary by its ID.
	GetByID(ctx context.Context, summaryID string) (*domain.ConversationSummary, error)

	// GetByUserID retrieves all summaries for a user, ordered by created_at desc.
	GetByUserID(ctx context.Context, userID string, limit int) ([]domain.ConversationSummary, error)

	// GetBySessionID retrieves a summary by session ID.
	GetBySessionID(ctx context.Context, sessionID string) (*domain.ConversationSummary, error)

	// List retrieves summaries with filtering and pagination.
	List(ctx context.Context, f domain.ConversationSummaryFilter) (summaries []domain.ConversationSummary, total int, err error)

	// Delete removes a summary by ID.
	Delete(ctx context.Context, summaryID string) error

	// DeleteOldest removes the oldest summaries for a user to maintain the sliding window.
	// Returns the number of summaries deleted.
	DeleteOldest(ctx context.Context, userID string, count int) (int64, error)

	// CountByUserID returns the total number of summaries for a user.
	CountByUserID(ctx context.Context, userID string) (int, error)
}
