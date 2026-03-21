// Package vectorstore provides an abstraction layer for vector databases.
// It supports multiple backends (Qdrant, Chroma) with a unified interface
// for storing and retrieving experiences via semantic search.
package vectorstore

import (
	"context"
	"errors"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// Errors
var (
	ErrNotFound       = errors.New("experience not found")
	ErrUnauthorized   = errors.New("unauthorized: user_id mismatch")
	ErrInvalidInput   = errors.New("invalid input")
	ErrConnectionLost = errors.New("connection to vector store lost")
)

// VectorStore defines the interface for vector database operations.
// Implementations must support:
// - User-level data isolation (all operations are scoped by user_id)
// - Semantic search with configurable similarity thresholds
// - Efficient batch operations for bulk writes
// - Sub-100ms retrieval latency for typical workloads
type VectorStore interface {
	// Write stores a single experience with its embedding.
	// The embedding is generated externally by the embedding pipeline.
	Write(ctx context.Context, exp *domain.Experience) error

	// WriteBatch stores multiple experiences in a single transaction.
	// Returns the number of successfully written experiences.
	WriteBatch(ctx context.Context, experiences []*domain.Experience) (int, error)

	// Search performs semantic search for experiences similar to the query vector.
	// Returns experiences sorted by similarity score (highest first).
	// Filters by user_id automatically for data isolation.
	Search(ctx context.Context, userID string, queryVec []float32, filter domain.ExperienceFilter) (*domain.ExperienceSearchResult, error)

	// GetByID retrieves a single experience by ID.
	// Returns ErrNotFound if not found or if userID doesn't match.
	GetByID(ctx context.Context, userID, experienceID string) (*domain.Experience, error)

	// Delete removes an experience by ID.
	// Returns ErrNotFound if not found or if userID doesn't match.
	Delete(ctx context.Context, userID, experienceID string) error

	// DeleteByUser removes all experiences for a user.
	// Used for data cleanup or user deletion.
	DeleteByUser(ctx context.Context, userID string) error

	// Stats returns statistics about stored experiences for a user.
	Stats(ctx context.Context, userID string) (*domain.ExperienceStats, error)

	// Health checks if the vector store is accessible.
	Health(ctx context.Context) error

	// Close releases resources used by the vector store.
	Close() error
}

// Config holds common configuration for vector stores.
type Config struct {
	// Collection name prefix (actual collection: prefix + "_" + user_id or shared collection with filtering)
	CollectionPrefix string

	// Embedding dimensions (must match the embedding model)
	Dimensions int

	// Distance metric: "cosine", "euclidean", "dot"
	DistanceMetric string

	// Timeout for operations
	Timeout time.Duration

	// Maximum batch size for bulk writes
	MaxBatchSize int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		CollectionPrefix: "experiences",
		Dimensions:       1536, // OpenAI text-embedding-3-small
		DistanceMetric:   "cosine",
		Timeout:          30 * time.Second,
		MaxBatchSize:     100,
	}
}


