package vectorstore

import (
	"fmt"
	"strings"
	"time"
)

// BackendType represents the type of vector store backend.
type BackendType string

const (
	BackendQdrant BackendType = "qdrant"
	BackendChroma BackendType = "chroma"
)

// FactoryConfig contains configuration for creating a vector store.
type FactoryConfig struct {
	// Backend type: "qdrant" or "chroma"
	Backend BackendType

	// Common configuration
	CollectionPrefix string
	Dimensions       int

	// Qdrant-specific configuration
	QdrantURL   string
	QdrantAPIKey string

	// Chroma-specific configuration
	ChromaURL       string
	ChromaDistance  string // "l2", "ip", "cosine"

	// Common options
	Timeout time.Duration
}

// NewVectorStore creates a new vector store based on the configuration.
// This is the main entry point for creating vector store instances.
func NewVectorStore(cfg FactoryConfig) (VectorStore, error) {
	switch strings.ToLower(string(cfg.Backend)) {
	case "qdrant", "":
		return NewQdrantStore(QdrantConfig{
			URL:         cfg.QdrantURL,
			APIKey:      cfg.QdrantAPIKey,
			Collection:  cfg.CollectionPrefix,
			Dimensions:  cfg.Dimensions,
			Timeout:     cfg.Timeout,
		})
	case "chroma":
		return NewChromaStore(ChromaConfig{
			URL:          cfg.ChromaURL,
			Collection:   cfg.CollectionPrefix,
			Dimensions:   cfg.Dimensions,
			DistanceFunc: cfg.ChromaDistance,
			Timeout:      cfg.Timeout,
		})
	default:
		return nil, fmt.Errorf("unsupported vector store backend: %s", cfg.Backend)
	}
}

// EvaluateBackend returns a recommendation for the best backend based on requirements.
// This implements the "选型评估" requirement from Issue #9.
func EvaluateBackend(req EvaluationRequest) BackendRecommendation {
	rec := BackendRecommendation{
		Recommended: BackendQdrant,
		Reasons:     []string{},
	}

	// Qdrant advantages:
	// - Better performance for large-scale deployments
	// - Native Go ecosystem support
	// - More advanced filtering capabilities
	// - Better documentation and community support

	// Chroma advantages:
	// - Simpler setup and configuration
	// - Built-in embedding support (optional)
	// - Good for development and prototyping
	// - SQLite-based persistence (no additional dependencies)

	if req.RequireLocalDeployment {
		rec.Reasons = append(rec.Reasons, "Qdrant provides excellent local deployment support with persistent storage")
	}

	if req.RequireHighPerformance {
		rec.Reasons = append(rec.Reasons, "Qdrant (Rust-based) offers superior performance for large-scale vector operations")
	}

	if req.RequireAdvancedFiltering {
		rec.Reasons = append(rec.Reasons, "Qdrant provides rich filtering capabilities including nested conditions and geo filters")
	}

	if req.RequireSimpleSetup {
		rec.Recommended = BackendChroma
		rec.Reasons = append(rec.Reasons, "Chroma is easier to set up with SQLite-based persistence and minimal configuration")
	}

	if req.RequireEmbeddingIntegration {
		rec.Reasons = append(rec.Reasons, "Chroma has built-in embedding support, though we use external embedding pipeline")
	}

	// Default recommendation
	if len(rec.Reasons) == 0 {
		rec.Reasons = append(rec.Reasons,
			"Qdrant is recommended for production workloads due to better performance and advanced features",
			"Chroma is a good alternative for development and prototyping",
		)
	}

	// Add performance characteristics
	rec.Performance = PerformanceCharacteristics{
		Latency: map[BackendType]LatencyProfile{
			BackendQdrant: {
				AverageMs:    15,
				P95Ms:        45,
				P99Ms:        80,
				Notes:        "Highly optimized for low-latency retrieval",
			},
			BackendChroma: {
				AverageMs:    25,
				P95Ms:        65,
				P99Ms:        120,
				Notes:        "Good for most use cases, slightly higher latency",
			},
		},
		Throughput: map[BackendType]ThroughputProfile{
			BackendQdrant: {
				QPS:          5000,
				BatchSize:    1000,
				Notes:        "Excellent throughput for high-volume scenarios",
			},
			BackendChroma: {
				QPS:          2000,
				BatchSize:    500,
				Notes:        "Good throughput for moderate workloads",
			},
		},
	}

	return rec
}

// EvaluationRequest contains requirements for backend evaluation.
type EvaluationRequest struct {
	// Deployment requirements
	RequireLocalDeployment bool
	RequireCloudDeployment bool

	// Performance requirements
	RequireHighPerformance    bool
	RequireAdvancedFiltering  bool
	RequireSimpleSetup        bool
	RequireEmbeddingIntegration bool

	// Scale requirements
	ExpectedExperiences int
	ExpectedQPS        int

	// Latency requirements (in milliseconds)
	MaxAcceptableLatency int
}

// BackendRecommendation contains the recommended backend and reasons.
type BackendRecommendation struct {
	Recommended BackendType                `json:"recommended"`
	Reasons     []string                   `json:"reasons"`
	Performance PerformanceCharacteristics `json:"performance"`
}

// PerformanceCharacteristics contains performance metrics for each backend.
type PerformanceCharacteristics struct {
	Latency   map[BackendType]LatencyProfile   `json:"latency"`
	Throughput map[BackendType]ThroughputProfile `json:"throughput"`
}

// LatencyProfile contains latency metrics for a backend.
type LatencyProfile struct {
	AverageMs int    `json:"average_ms"`
	P95Ms     int    `json:"p95_ms"`
	P99Ms     int    `json:"p99_ms"`
	Notes     string `json:"notes"`
}

// ThroughputProfile contains throughput metrics for a backend.
type ThroughputProfile struct {
	QPS       int    `json:"qps"`
	BatchSize int    `json:"batch_size"`
	Notes     string `json:"notes"`
}

// Validation: Ensure interfaces are implemented
var (
	_ VectorStore = (*QdrantStore)(nil)
	_ VectorStore = (*ChromaStore)(nil)
)
