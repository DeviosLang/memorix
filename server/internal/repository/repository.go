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
}
