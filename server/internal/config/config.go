package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port      string
	DSN       string
	RateLimit float64
	RateBurst int

	// Auto-embedding: TiDB Serverless generates embeddings via EMBED_TEXT().
	// When set, takes priority over client-side embedding.
	// Example: "tidbcloud_free/amazon/titan-embed-text-v2"
	EmbedAutoModel string
	EmbedAutoDims  int

	// Client-side embedding provider (optional — omit for keyword-only search).
	EmbedAPIKey  string
	EmbedBaseURL string
	EmbedModel   string
	EmbedDims    int

	LLMAPIKey      string
	LLMBaseURL     string
	LLMModel       string
	LLMTemperature float64
	IngestMode     string

	TiDBZeroEnabled       bool
	TiDBZeroAPIURL        string
	TenantPoolMaxIdle     int
	TenantPoolMaxOpen     int
	TenantPoolIdleTimeout time.Duration
	TenantPoolTotalLimit  int

	// FTSEnabled controls whether full-text search is attempted.
	// Set MNEMO_FTS_ENABLED=true only when the TiDB cluster supports
	// FULLTEXT INDEX and FTS_MATCH_WORD with constant strings.
	// Defaults to false (safe for all TiDB Serverless / TiDB Zero tiers).
	FTSEnabled bool

	// WorkerConcurrency controls how many upload tasks are processed in parallel.
	// Defaults to 5.
	WorkerConcurrency int

	// Upload directory for file storage.
	// Files are stored at {UploadDir}/{tenantID}/{agentID}/{filename}.
	UploadDir string

	// Context window configuration for sliding window management.
	// MaxContextTokens is the maximum number of tokens allowed in a context window.
	// Default is 8192 (8K), recommended range is 8K-16K depending on model.
	MaxContextTokens int

	// TokenizerType specifies which tokenizer to use for token counting.
	// Valid values: "tiktoken" (default), "estimate".
	TokenizerType string

	// TokenizerModel specifies the model for tokenizer encoding selection.
	// For tiktoken, this determines the encoding (e.g., "gpt-4" -> cl100k_base).
	TokenizerModel string

	// SystemPromptReservedTokens reserves tokens for system prompts.
	// Default is 500 tokens.
	SystemPromptReservedTokens int

	// MemoryReservedTokens reserves tokens for memory injection area.
	// Default is 2000 tokens.
	MemoryReservedTokens int

	// MetadataReservedTokens reserves tokens for session metadata injection.
	// Default is 200 tokens (per acceptance criteria).
	MetadataReservedTokens int

	// Context Builder elastic budget configuration.
	// These control the token budget ranges for elastic layers.

	// UserMemoryBudgetMin is the minimum tokens for user memory layer.
	// Default is 500 tokens.
	UserMemoryBudgetMin int

	// UserMemoryBudgetMax is the maximum tokens for user memory layer.
	// Default is 1500 tokens.
	UserMemoryBudgetMax int

	// SummaryBudgetMin is the minimum tokens for conversation summary layer.
	// Default is 300 tokens.
	SummaryBudgetMin int

	// SummaryBudgetMax is the maximum tokens for conversation summary layer.
	// Default is 800 tokens.
	SummaryBudgetMax int

	// Memory GC configuration

	// GCEnabled controls whether memory garbage collection runs automatically.
	// Default is true.
	GCEnabled bool

	// GCInterval is the time between GC runs.
	// Default is 24h (daily).
	GCInterval time.Duration

	// GCStaleThreshold is the duration after which unaccessed memories become stale.
	// Default is 90 days.
	GCStaleThreshold time.Duration

	// GCLowConfidenceThreshold is the confidence below which memories are candidates for cleanup.
	// Default is 0.5.
	GCLowConfidenceThreshold float64

	// GCMaxMemoriesPerTenant is the capacity limit per tenant.
	// When exceeded, lowest importance memories are cleaned up.
	// Default is 10000.
	GCMaxMemoriesPerTenant int

	// GCSnapshotRetentionDays is how long to keep GC snapshots for recovery.
	// Default is 30 days.
	GCSnapshotRetentionDays int

	// GCBatchSize is the number of memories to process per GC iteration.
	// Default is 100.
	GCBatchSize int

	// Experience layer configuration (Issue #9: 向量存储集成)

	// ExperienceEnabled controls whether the experience recall layer is enabled.
	// Default is false (requires explicit configuration).
	ExperienceEnabled bool

	// ExperienceBackend specifies the vector store backend.
	// Valid values: "qdrant" (default), "chroma".
	ExperienceBackend string

	// ExperienceMaxPerUser is the maximum experiences per user.
	// Default is 10000.
	ExperienceMaxPerUser int

	// Qdrant configuration
	QdrantURL    string
	QdrantAPIKey string

	// Chroma configuration
	ChromaURL      string
	ChromaDistance string // "l2", "ip", "cosine"

	// Rules configuration (Issue #11: 工程化与团队协作)

	// RulesEnabled controls whether the rules system is enabled.
	// Default is true.
	RulesEnabled bool

	// RulesOrganizationPath is the path to organization-level rules.
	// Default: /etc/agent/rules.md
	RulesOrganizationPath string

	// RulesUserPath is the path to user-level rules.
	// Default: ~/.agent/rules.md
	RulesUserPath string

	// RulesInjectionEnabled controls whether rules are injected into system prompts.
	// Default is true.
	RulesInjectionEnabled bool

	// RulesInjectionMaxTokens is the maximum tokens for rules content.
	// Default is 2000.
	RulesInjectionMaxTokens int

	// RulesInjectionHeader is prepended to rules content.
	// Default: "## Project Rules\n\n"
	RulesInjectionHeader string
}

func Load() (*Config, error) {
	dsn := os.Getenv("MNEMO_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("MNEMO_DSN is required")
	}

	// Get home directory for user-level paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "" // Will fall back to env var or empty
	}

	cfg := &Config{
		Port:                  envOr("MNEMO_PORT", "8080"),
		DSN:                   dsn,
		RateLimit:             envFloat("MNEMO_RATE_LIMIT", 100),
		RateBurst:             envInt("MNEMO_RATE_BURST", 200),
		EmbedAutoModel:        os.Getenv("MNEMO_EMBED_AUTO_MODEL"),
		EmbedAutoDims:         envInt("MNEMO_EMBED_AUTO_DIMS", 1024),
		EmbedAPIKey:           os.Getenv("MNEMO_EMBED_API_KEY"),
		EmbedBaseURL:          os.Getenv("MNEMO_EMBED_BASE_URL"),
		EmbedModel:            os.Getenv("MNEMO_EMBED_MODEL"),
		EmbedDims:             envInt("MNEMO_EMBED_DIMS", 1536),
		LLMAPIKey:             os.Getenv("MNEMO_LLM_API_KEY"),
		LLMBaseURL:            os.Getenv("MNEMO_LLM_BASE_URL"),
		LLMModel:              envOr("MNEMO_LLM_MODEL", "gpt-4o-mini"),
		LLMTemperature:        envFloat("MNEMO_LLM_TEMPERATURE", 0.1),
		IngestMode:            envOr("MNEMO_INGEST_MODE", "smart"),
		TiDBZeroEnabled:       envBool("MNEMO_TIDB_ZERO_ENABLED", true),
		TiDBZeroAPIURL:        envOr("MNEMO_TIDB_ZERO_API_URL", "https://zero.tidbapi.com/v1alpha1"),
		TenantPoolMaxIdle:     envInt("MNEMO_TENANT_POOL_MAX_IDLE", 5),
		TenantPoolMaxOpen:     envInt("MNEMO_TENANT_POOL_MAX_OPEN", 10),
		TenantPoolIdleTimeout: envDuration("MNEMO_TENANT_POOL_IDLE_TIMEOUT", 10*time.Minute),
		TenantPoolTotalLimit:  envInt("MNEMO_TENANT_POOL_TOTAL_LIMIT", 200),
		UploadDir:             envOr("MNEMO_UPLOAD_DIR", "./uploads"),
		FTSEnabled:            envBool("MNEMO_FTS_ENABLED", false),
		WorkerConcurrency:     envInt("MNEMO_WORKER_CONCURRENCY", 5),
		MaxContextTokens:      envInt("MNEMO_MAX_CONTEXT_TOKENS", 8192),
		TokenizerType:         envOr("MNEMO_TOKENIZER_TYPE", "tiktoken"),
		TokenizerModel:        envOr("MNEMO_TOKENIZER_MODEL", "gpt-4"),
		SystemPromptReservedTokens:  envInt("MNEMO_SYSTEM_PROMPT_RESERVED_TOKENS", 500),
		MemoryReservedTokens:  envInt("MNEMO_MEMORY_RESERVED_TOKENS", 2000),
		MetadataReservedTokens: envInt("MNEMO_METADATA_RESERVED_TOKENS", 200),
		UserMemoryBudgetMin:   envInt("MNEMO_USER_MEMORY_BUDGET_MIN", 500),
		UserMemoryBudgetMax:   envInt("MNEMO_USER_MEMORY_BUDGET_MAX", 1500),
		SummaryBudgetMin:      envInt("MNEMO_SUMMARY_BUDGET_MIN", 300),
		SummaryBudgetMax:      envInt("MNEMO_SUMMARY_BUDGET_MAX", 800),

		// Memory GC configuration
		GCEnabled:                envBool("MNEMO_GC_ENABLED", true),
		GCInterval:               envDuration("MNEMO_GC_INTERVAL", 24*time.Hour),
		GCStaleThreshold:         envDuration("MNEMO_GC_STALE_THRESHOLD", 90*24*time.Hour), // 90 days
		GCLowConfidenceThreshold: envFloat("MNEMO_GC_LOW_CONFIDENCE_THRESHOLD", 0.5),
		GCMaxMemoriesPerTenant:   envInt("MNEMO_GC_MAX_MEMORIES_PER_TENANT", 10000),
		GCSnapshotRetentionDays:  envInt("MNEMO_GC_SNAPSHOT_RETENTION_DAYS", 30),
		GCBatchSize:              envInt("MNEMO_GC_BATCH_SIZE", 100),

		// Experience layer configuration
		ExperienceEnabled:    envBool("MNEMO_EXPERIENCE_ENABLED", false),
		ExperienceBackend:    envOr("MNEMO_EXPERIENCE_BACKEND", "qdrant"),
		ExperienceMaxPerUser: envInt("MNEMO_EXPERIENCE_MAX_PER_USER", 10000),
		QdrantURL:            envOr("MNEMO_QDRANT_URL", "http://localhost:6333"),
		QdrantAPIKey:         os.Getenv("MNEMO_QDRANT_API_KEY"),
		ChromaURL:            envOr("MNEMO_CHROMA_URL", "http://localhost:8000"),
		ChromaDistance:       envOr("MNEMO_CHROMA_DISTANCE", "cosine"),

		// Rules configuration
		RulesEnabled:             envBool("MNEMO_RULES_ENABLED", true),
		RulesOrganizationPath:    envOr("MNEMO_RULES_ORGANIZATION_PATH", "/etc/agent/rules.md"),
		RulesUserPath:            envOr("MNEMO_RULES_USER_PATH", func() string {
			if homeDir != "" {
				return homeDir + "/.agent/rules.md"
			}
			return ""
		}()),
		RulesInjectionEnabled:    envBool("MNEMO_RULES_INJECTION_ENABLED", true),
		RulesInjectionMaxTokens:  envInt("MNEMO_RULES_INJECTION_MAX_TOKENS", 2000),
		RulesInjectionHeader:     envOr("MNEMO_RULES_INJECTION_HEADER", "## Project Rules\n\n"),
	}
	// Validate ingest mode.
	switch cfg.IngestMode {
	case "smart", "raw", "":
		// ok
	default:
		return nil, fmt.Errorf("unsupported MNEMO_INGEST_MODE %q; valid values are \"smart\" and \"raw\"", cfg.IngestMode)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
