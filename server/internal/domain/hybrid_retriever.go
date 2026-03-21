package domain

import (
	"time"
)

// HybridRetrieverResult represents a single retrieved item with comprehensive metadata.
// It provides full tracing information about which layer the result came from,
// how it was scored, and how it was combined with other results.
type HybridRetrieverResult struct {
	// ID is the unique identifier for the result.
	// For UserProfileFact: FactID
	// For ConversationSummary: SummaryID
	// For Experience: ExperienceID
	ID string `json:"id"`

	// Content is the textual content of the result.
	Content string `json:"content"`

	// SourceLayer indicates which memory layer this result came from.
	SourceLayer RetrievalSourceLayer `json:"source_layer"`

	// SourceType indicates the type of data within the layer.
	// For LayerUserProfile: "fact"
	// For LayerConversationSummary: "summary"
	// For LayerExperience: "experience"
	SourceType string `json:"source_type"`

	// ScoreBreakdown contains the detailed scoring components.
	ScoreBreakdown ScoreBreakdown `json:"score_breakdown"`

	// FinalScore is the weighted combination of all score components.
	FinalScore float64 `json:"final_score"`

	// Metadata contains additional context-specific metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// TokenCount is the estimated token count of the content.
	TokenCount int `json:"token_count"`

	// CreatedAt is when this item was originally created.
	CreatedAt time.Time `json:"created_at"`
}

// RetrievalSourceLayer identifies the source memory layer for a result.
type RetrievalSourceLayer string

const (
	// RetrievalLayerUserProfile represents the User Profile Store (first layer).
	// Contains structured facts about the user: name, preferences, goals.
	// Retrieval: exact match on key/category.
	RetrievalLayerUserProfile RetrievalSourceLayer = "user_profile"

	// RetrievalLayerConversationSummary represents the Conversation Summary Pool (second layer).
	// Contains pre-computed summaries of recent conversations.
	// Retrieval: keyword match + time range filter.
	RetrievalLayerConversationSummary RetrievalSourceLayer = "conversation_summary"

	// RetrievalLayerExperience represents the Vector Store (third layer).
	// Contains semantic experiences extracted from past conversations.
	// Retrieval: semantic similarity (cosine distance).
	RetrievalLayerExperience RetrievalSourceLayer = "experience"
)

// ScoreBreakdown contains the detailed components of a result's score.
// Final score = w1 * ExactMatchScore + w2 * SemanticScore + w3 * TimeDecayScore + w4 * ImportanceScore
type ScoreBreakdown struct {
	// ExactMatchScore is 1.0 if the query exactly matches a key/category in user profile.
	// For other layers, this is 0.0.
	ExactMatchScore float64 `json:"exact_match_score"`

	// SemanticScore is the cosine similarity from vector search.
	// For user profile (exact match), this is 1.0.
	// For conversation summary, this is keyword overlap score (0-1).
	// For experience, this is the actual cosine similarity.
	SemanticScore float64 `json:"semantic_score"`

	// TimeDecayScore represents recency: newer items score higher.
	// Calculated as: exp(-decay_rate * hours_since_creation)
	// Range: 0.0 (very old) to 1.0 (just created).
	TimeDecayScore float64 `json:"time_decay_score"`

	// ImportanceScore represents the inherent importance of the item.
	// For user profile: confidence field
	// For experience: confidence field from metadata
	// For conversation summary: derived from key topics count
	ImportanceScore float64 `json:"importance_score"`

	// Individual weights applied during scoring (for tracing).
	Weights ScoringWeights `json:"weights"`
}

// ScoringWeights contains the weights used for combining scores.
type ScoringWeights struct {
	W1ExactMatch  float64 `json:"w1_exact_match"`
	W2Semantic    float64 `json:"w2_semantic"`
	W3TimeDecay   float64 `json:"w3_time_decay"`
	W4Importance  float64 `json:"w4_importance"`
}

// HybridRetrievalContext contains additional context for retrieval.
type HybridRetrievalContext struct {
	// SessionID is the current session identifier.
	SessionID string `json:"session_id,omitempty"`

	// MaxTokens is the maximum total tokens for the result set.
	// Results will be trimmed to fit within this budget.
	MaxTokens int `json:"max_tokens,omitempty"`

	// TimeRange limits results to a specific time window.
	// Only results within this range will be considered.
	TimeRange *TimeRange `json:"time_range,omitempty"`

	// Categories filters user profile results by category.
	Categories []FactCategory `json:"categories,omitempty"`

	// Topics filters experience results by topic.
	Topics []string `json:"topics,omitempty"`

	// MinScore sets a minimum score threshold for results.
	MinScore float64 `json:"min_score,omitempty"`

	// TopK sets the maximum number of results per layer.
	TopK int `json:"top_k,omitempty"`

	// IncludeLayers specifies which layers to include in retrieval.
	// If empty, all layers are included.
	IncludeLayers []RetrievalSourceLayer `json:"include_layers,omitempty"`

	// CustomWeights allows overriding default scoring weights.
	CustomWeights *ScoringWeights `json:"custom_weights,omitempty"`

	// TimeDecayRate controls how quickly scores decay with time.
	// Default: 0.001 (slow decay). Higher = faster decay.
	TimeDecayRate float64 `json:"time_decay_rate,omitempty"`
}

// TimeRange specifies a time window for filtering results.
type TimeRange struct {
	// Start is the start of the time range (inclusive).
	Start time.Time `json:"start"`

	// End is the end of the time range (inclusive).
	End time.Time `json:"end"`
}

// HybridRetrievalResult contains the complete retrieval result with tracing.
type HybridRetrievalResult struct {
	// Results is the deduplicated, sorted list of retrieved items.
	Results []HybridRetrieverResult `json:"results"`

	// TotalTokens is the sum of all result token counts.
	TotalTokens int `json:"total_tokens"`

	// MaxTokens is the token budget that was used.
	MaxTokens int `json:"max_tokens"`

	// Truncated indicates whether results were trimmed to fit budget.
	Truncated bool `json:"truncated"`

	// LayerStatistics contains per-layer retrieval statistics.
	LayerStatistics []LayerRetrievalStats `json:"layer_statistics"`

	// RetrievalTrace contains detailed tracing information.
	RetrievalTrace RetrievalTrace `json:"retrieval_trace"`

	// Query is the original query string.
	Query string `json:"query"`

	// UserID is the user ID for this retrieval.
	UserID string `json:"user_id"`

	// LatencyMs is the total retrieval latency in milliseconds.
	LatencyMs int64 `json:"latency_ms"`
}

// LayerRetrievalStats contains statistics for a single layer retrieval.
type LayerRetrievalStats struct {
	// Layer identifies the retrieval layer.
	Layer RetrievalSourceLayer `json:"layer"`

	// CandidatesFound is the number of items found before deduplication.
	CandidatesFound int `json:"candidates_found"`

	// CandidatesReturned is the number of items returned after deduplication.
	CandidatesReturned int `json:"candidates_returned"`

	// AvgScore is the average score of returned items.
	AvgScore float64 `json:"avg_score"`

	// LatencyMs is the retrieval latency for this layer.
	LatencyMs int64 `json:"latency_ms"`

	// Error contains any error that occurred during retrieval.
	Error string `json:"error,omitempty"`
}

// RetrievalTrace contains detailed tracing information for debugging.
type RetrievalTrace struct {
	// QueryIntent is the detected intent of the query.
	// "exact_fact": Query seeks a specific fact (e.g., "What's my name?")
	// "semantic": Query seeks related experiences (e.g., "discussed architecture")
	// "hybrid": Query combines both
	QueryIntent string `json:"query_intent"`

	// ExtractedKeywords contains keywords extracted from the query.
	ExtractedKeywords []string `json:"extracted_keywords,omitempty"`

	// DetectedCategories contains categories detected from the query.
	// Used for user profile retrieval.
	DetectedCategories []FactCategory `json:"detected_categories,omitempty"`

	// DeduplicationStats contains information about deduplication.
	DeduplicationStats DeduplicationStats `json:"deduplication_stats"`

	// ScoringDetails contains additional scoring details.
	ScoringDetails string `json:"scoring_details,omitempty"`
}

// DeduplicationStats contains statistics about the deduplication process.
type DeduplicationStats struct {
	// TotalCandidates is the total number of candidates before deduplication.
	TotalCandidates int `json:"total_candidates"`

	// DuplicatesRemoved is the number of duplicates removed.
	DuplicatesRemoved int `json:"duplicates_removed"`

	// DeduplicationMethod describes how duplicates were identified.
	DeduplicationMethod string `json:"deduplication_method"`
}

// HybridRetrieverConfig contains configuration for the hybrid retriever.
type HybridRetrieverConfig struct {
	// DefaultTopK is the default number of results per layer.
	DefaultTopK int `json:"default_top_k"`

	// DefaultMaxTokens is the default token budget.
	DefaultMaxTokens int `json:"default_max_tokens"`

	// DefaultMinScore is the default minimum score threshold.
	DefaultMinScore float64 `json:"default_min_score"`

	// DefaultWeights are the default scoring weights.
	DefaultWeights ScoringWeights `json:"default_weights"`

	// DefaultTimeDecayRate is the default time decay rate.
	DefaultTimeDecayRate float64 `json:"default_time_decay_rate"`

	// SummaryTimeRangeHours limits conversation summary retrieval to recent hours.
	// Default: 168 (7 days).
	SummaryTimeRangeHours int `json:"summary_time_range_hours"`
}

// DefaultHybridRetrieverConfig returns sensible defaults.
func DefaultHybridRetrieverConfig() HybridRetrieverConfig {
	return HybridRetrieverConfig{
		DefaultTopK:            10,
		DefaultMaxTokens:       2000,
		DefaultMinScore:        0.3,
		DefaultTimeDecayRate:   0.001, // Slow decay
		SummaryTimeRangeHours: 168,    // 7 days
		DefaultWeights: ScoringWeights{
			W1ExactMatch: 0.4,
			W2Semantic:   0.3,
			W3TimeDecay:  0.15,
			W4Importance: 0.15,
		},
	}
}
