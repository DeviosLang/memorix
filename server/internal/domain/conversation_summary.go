package domain

import (
	"time"
)

// ConversationSummary represents a summary of a completed conversation session.
// Based on ChatGPT's fourth-layer "recent conversation summary" design from reverse engineering analysis.
// Key advantages: zero retrieval latency (pre-computed), provides context for ongoing conversations.
type ConversationSummary struct {
	SummaryID string `json:"summary_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`

	// LLM-generated content
	Title      string   `json:"title"`        // Conversation title (LLM generated)
	Summary    string   `json:"summary"`      // Summary within 200 Chinese characters
	KeyTopics  []string `json:"key_topics"`   // Key topic tags
	UserIntent string   `json:"user_intent"`  // Core user intent

	CreatedAt time.Time `json:"created_at"`
}

// ConversationSummaryFilter encapsulates query parameters for summary queries.
type ConversationSummaryFilter struct {
	UserID    string
	SessionID string
	KeyTopic  string // Filter by topic (partial match)
	Limit     int
	Offset    int
}

// Default capacity limits for the sliding window
const (
	DefaultMaxSummariesPerUser = 20 // Maximum summaries to keep per user
	MinSummariesPerUser        = 15 // Minimum before cleanup triggers
	MaxSummaryLength           = 200 // Maximum characters for summary field
	MaxTitleLength             = 100 // Maximum characters for title
	MaxTopicsCount             = 5   // Maximum number of key topics
	MaxUserIntentLength        = 100 // Maximum characters for user intent
)

// SummaryGenerationRequest contains the input for generating a conversation summary.
type SummaryGenerationRequest struct {
	UserID    string          `json:"user_id"`
	SessionID string          `json:"session_id"`
	Messages  []SummaryMessage `json:"messages"`
}

// SummaryMessage represents a single message in the conversation.
type SummaryMessage struct {
	Role      string `json:"role"`       // "user" or "assistant"
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SummaryGenerationResult contains the LLM-generated summary data.
type SummaryGenerationResult struct {
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	KeyTopics  []string `json:"key_topics"`
	UserIntent string   `json:"user_intent"`
}
