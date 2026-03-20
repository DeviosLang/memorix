package domain

// ContextLayer represents a priority layer in the context assembly.
// Layers are assembled in priority order (highest priority first).
// When truncation is needed, lower priority layers are truncated first.
type ContextLayer string

const (
	// LayerSystem is the highest priority layer containing system instructions.
	// This layer is never truncated.
	LayerSystem ContextLayer = "system"

	// LayerMetadata contains session metadata (device type, timezone, etc.).
	// This layer has high priority and is rarely truncated.
	LayerMetadata ContextLayer = "metadata"

	// LayerUserMemory contains user-specific memories and facts.
	// This layer has medium-high priority with elastic token budget.
	LayerUserMemory ContextLayer = "user_memory"

	// LayerConversationSummary contains summaries of past conversations.
	// This layer has medium priority with elastic token budget.
	LayerConversationSummary ContextLayer = "conversation_summary"

	// LayerCurrentSession contains the current conversation messages.
	// This layer has the lowest priority and is truncated first when needed.
	LayerCurrentSession ContextLayer = "current_session"
)

// LayerPriority returns the priority of a context layer.
// Higher values indicate higher priority (less likely to be truncated).
func LayerPriority(layer ContextLayer) int {
	switch layer {
	case LayerSystem:
		return 100
	case LayerMetadata:
		return 80
	case LayerUserMemory:
		return 60
	case LayerConversationSummary:
		return 40
	case LayerCurrentSession:
		return 20
	default:
		return 0
	}
}

// TokenBudget represents a token budget with optional elastic range.
type TokenBudget struct {
	// Fixed is the fixed token budget.
	// If non-zero, this is the exact budget allocated.
	Fixed int `json:"fixed,omitempty"`

	// Min is the minimum tokens for elastic budget.
	Min int `json:"min,omitempty"`

	// Max is the maximum tokens for elastic budget.
	Max int `json:"max,omitempty"`

	// Elastic indicates whether this budget is elastic (can expand/contract).
	Elastic bool `json:"elastic"`
}

// NewFixedBudget creates a fixed token budget.
func NewFixedBudget(tokens int) TokenBudget {
	return TokenBudget{Fixed: tokens, Elastic: false}
}

// NewElasticBudget creates an elastic token budget with min/max range.
func NewElasticBudget(min, max int) TokenBudget {
	return TokenBudget{Min: min, Max: max, Elastic: true}
}

// LayerContent represents the content for a context layer.
type LayerContent struct {
	// Layer identifies which context layer this content belongs to.
	Layer ContextLayer `json:"layer"`

	// Content is the text content for this layer.
	Content string `json:"content"`

	// TokenCount is the actual token count of the content.
	// This is computed when the content is added.
	TokenCount int `json:"token_count"`

	// Source indicates where this content came from (for logging/debugging).
	Source string `json:"source,omitempty"`

	// Priority override. If set, uses this instead of LayerPriority(layer).
	PriorityOverride int `json:"priority_override,omitempty"`
}

// Priority returns the effective priority for this layer content.
func (lc *LayerContent) Priority() int {
	if lc.PriorityOverride > 0 {
		return lc.PriorityOverride
	}
	return LayerPriority(lc.Layer)
}

// BuildContextRequest contains the input for building a context prompt.
type BuildContextRequest struct {
	// SessionID is the current session identifier.
	SessionID string `json:"session_id,omitempty"`

	// UserID is the user identifier for fetching user memories/facts.
	UserID string `json:"user_id,omitempty"`

	// SystemInstructions contains the base system prompt/instructions.
	SystemInstructions string `json:"system_instructions,omitempty"`

	// SessionMetadata contains session-level metadata.
	SessionMetadata *SessionMetadata `json:"session_metadata,omitempty"`

	// UserMemories contains relevant user memories to inject.
	UserMemories []Memory `json:"user_memories,omitempty"`

	// UserProfileFacts contains user profile facts.
	UserProfileFacts []UserProfileFact `json:"user_profile_facts,omitempty"`

	// ConversationSummary is a summary of past conversations.
	ConversationSummary string `json:"conversation_summary,omitempty"`

	// CurrentMessages contains the current session messages.
	CurrentMessages []ContextMessage `json:"current_messages,omitempty"`

	// MaxTokens is the maximum total tokens allowed.
	// If not set, uses the default from configuration.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// ContextBuildResult contains the assembled context and metadata.
type ContextBuildResult struct {
	// Prompt is the assembled system prompt.
	Prompt string `json:"prompt"`

	// TotalTokens is the total token count of the assembled prompt.
	TotalTokens int `json:"total_tokens"`

	// MaxTokens is the maximum tokens that were allowed.
	MaxTokens int `json:"max_tokens"`

	// LayerStats contains token statistics for each layer.
	LayerStats []LayerStats `json:"layer_stats"`

	// Truncated indicates whether any truncation occurred.
	Truncated bool `json:"truncated"`

	// TruncationDetails contains details about what was truncated.
	TruncationDetails []TruncationDetail `json:"truncation_details,omitempty"`
}

// LayerStats contains token statistics for a context layer.
type LayerStats struct {
	// Layer identifies the context layer.
	Layer ContextLayer `json:"layer"`

	// OriginalTokens is the token count before truncation.
	OriginalTokens int `json:"original_tokens"`

	// FinalTokens is the token count after truncation.
	FinalTokens int `json:"final_tokens"`

	// BudgetUsed is the token budget that was allocated.
	BudgetUsed int `json:"budget_used"`

	// Truncated indicates whether this layer was truncated.
	Truncated bool `json:"truncated"`

	// Percentage is the percentage of the total prompt this layer occupies.
	Percentage float64 `json:"percentage"`
}

// TruncationDetail contains details about a truncation operation.
type TruncationDetail struct {
	// Layer identifies which layer was truncated.
	Layer ContextLayer `json:"layer"`

	// OriginalContent is the content before truncation (truncated for logging).
	OriginalContent string `json:"original_content,omitempty"`

	// TruncatedContent is the content after truncation.
	TruncatedContent string `json:"truncated_content,omitempty"`

	// TokensRemoved is the number of tokens removed.
	TokensRemoved int `json:"tokens_removed"`

	// Reason explains why truncation occurred.
	Reason string `json:"reason"`
}

// ContextMessage represents a message in the current conversation.
// This is similar to service.ContextMessage but defined here to avoid
// circular dependencies and provide domain-level abstraction.
type ContextMessage struct {
	// Role is the message role: "system", "user", "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content string `json:"content"`

	// ID is an optional identifier for the message.
	ID string `json:"id,omitempty"`

	// TokenCount is the computed token count.
	TokenCount int `json:"token_count,omitempty"`
}
