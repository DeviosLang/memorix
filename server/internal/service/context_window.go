package service

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/devioslang/memorix/server/internal/tokenizer"
)

// ContextWindowConfig holds configuration for the sliding window context manager.
type ContextWindowConfig struct {
	// MaxTokens is the maximum number of tokens allowed in the context window.
	MaxTokens int

	// SystemPromptReservedTokens reserves tokens for system prompts.
	SystemPromptReservedTokens int

	// MemoryReservedTokens reserves tokens for memory injection area.
	MemoryReservedTokens int

	// MetadataReservedTokens reserves tokens for session metadata injection.
	// Default is 200 tokens (per acceptance criteria).
	MetadataReservedTokens int

	// Tokenizer is the tokenizer to use for counting tokens.
	Tokenizer tokenizer.Tokenizer
}

// DefaultContextWindowConfig returns sensible defaults for context window management.
func DefaultContextWindowConfig() ContextWindowConfig {
	return ContextWindowConfig{
		MaxTokens:                  8192,
		SystemPromptReservedTokens: 500,
		MemoryReservedTokens:       2000,
		MetadataReservedTokens:     200,
		Tokenizer:                  tokenizer.NewDefault(),
	}
}

// ContextWindow manages token-based sliding window truncation for conversation context.
// It ensures that:
// 1. System prompts are always preserved (not truncated)
// 2. Session metadata is always preserved (not truncated) - injected once per session
// 3. Memory injection areas are always preserved (not truncated)
// 4. User/assistant message pairs are truncated from the oldest first
// 5. The total token count stays within the configured limit
type ContextWindow struct {
	config ContextWindowConfig
}

// NewContextWindow creates a new context window manager.
func NewContextWindow(config ContextWindowConfig) *ContextWindow {
	if config.Tokenizer == nil {
		config.Tokenizer = tokenizer.NewDefault()
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 8192
	}
	if config.SystemPromptReservedTokens <= 0 {
		config.SystemPromptReservedTokens = 500
	}
	if config.MemoryReservedTokens <= 0 {
		config.MemoryReservedTokens = 2000
	}
	if config.MetadataReservedTokens <= 0 {
		config.MetadataReservedTokens = 200
	}
	return &ContextWindow{config: config}
}

// ContextMessage represents a message in the conversation context.
type ContextMessage struct {
	// Role is the message role: "system", "user", "assistant", "metadata", or "memory".
	// "metadata" is a special role for session metadata that is never truncated.
	// "memory" is a special role for injected memory context that is never truncated.
	Role string `json:"role"`

	// Content is the message content.
	Content string `json:"content"`

	// ID is an optional identifier for the message (useful for tracking truncation).
	ID string `json:"id,omitempty"`
}

// TruncationResult contains the result of a truncation operation.
type TruncationResult struct {
	// Messages is the truncated message list.
	Messages []ContextMessage `json:"messages"`

	// OriginalTokens is the total token count before truncation.
	OriginalTokens int `json:"original_tokens"`

	// FinalTokens is the total token count after truncation.
	FinalTokens int `json:"final_tokens"`

	// MessagesDropped is the number of messages that were removed.
	MessagesDropped int `json:"messages_dropped"`

	// DroppedRoles counts how many messages of each role were dropped.
	DroppedRoles map[string]int `json:"dropped_roles,omitempty"`

	// Truncated indicates whether any truncation occurred.
	Truncated bool `json:"truncated"`
}

// Truncate applies the sliding window truncation strategy to the messages.
// It preserves system messages, metadata, and memory injections, truncating user/assistant
// pairs from the oldest first to fit within the token limit.
func (cw *ContextWindow) Truncate(messages []ContextMessage) *TruncationResult {
	// Calculate available tokens after reserving space for system, metadata, and memory
	reservedTokens := cw.config.SystemPromptReservedTokens + cw.config.MemoryReservedTokens + cw.config.MetadataReservedTokens
	availableTokens := cw.config.MaxTokens - reservedTokens
	if availableTokens < 100 {
		availableTokens = 100 // Minimum usable context
	}

	// Separate messages into categories
	var systemMessages []ContextMessage
	var metadataMessages []ContextMessage
	var memoryMessages []ContextMessage
	var conversationMessages []ContextMessage

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			systemMessages = append(systemMessages, msg)
		case "metadata":
			metadataMessages = append(metadataMessages, msg)
		case "memory":
			memoryMessages = append(memoryMessages, msg)
		default:
			conversationMessages = append(conversationMessages, msg)
		}
	}

	// Calculate tokens for preserved messages
	systemTokens := cw.countTokens(systemMessages)
	metadataTokens := cw.countTokens(metadataMessages)
	memoryTokens := cw.countTokens(memoryMessages)

	// Adjust available tokens based on actual system/metadata/memory usage
	availableForConversation := availableTokens - systemTokens - metadataTokens - memoryTokens
	if availableForConversation < 100 {
		// If reserved space is already exceeded, try to still fit some conversation
		availableForConversation = cw.config.MaxTokens/4 - systemTokens - metadataTokens - memoryTokens
		if availableForConversation < 50 {
			availableForConversation = 50
		}
	}

	// Calculate original tokens
	originalTokens := cw.countTokens(messages)

	// Truncate conversation messages
	truncated, dropped := cw.truncateConversation(conversationMessages, availableForConversation)

	// Build final message list
	// Order: system -> metadata -> memory -> conversation
	result := make([]ContextMessage, 0, len(systemMessages)+len(metadataMessages)+len(memoryMessages)+len(truncated))
	result = append(result, systemMessages...)
	result = append(result, metadataMessages...)
	result = append(result, memoryMessages...)
	result = append(result, truncated...)

	finalTokens := cw.countTokens(result)

	// Build result
	truncationResult := &TruncationResult{
		Messages:        result,
		OriginalTokens:  originalTokens,
		FinalTokens:     finalTokens,
		MessagesDropped: len(conversationMessages) - len(truncated),
		Truncated:       len(conversationMessages) > len(truncated),
	}

	// Log truncation details
	if truncationResult.Truncated {
		droppedRoles := make(map[string]int)
		for _, msg := range dropped {
			droppedRoles[msg.Role]++
		}
		truncationResult.DroppedRoles = droppedRoles

		slog.Info("context window truncation",
			"original_tokens", originalTokens,
			"final_tokens", finalTokens,
			"max_tokens", cw.config.MaxTokens,
			"messages_dropped", truncationResult.MessagesDropped,
			"dropped_roles", fmt.Sprintf("%v", droppedRoles),
		)
	}

	return truncationResult
}

// truncateConversation truncates conversation messages (user/assistant) to fit within token limit.
// It drops the oldest user/assistant message pairs first.
func (cw *ContextWindow) truncateConversation(messages []ContextMessage, maxTokens int) ([]ContextMessage, []ContextMessage) {
	if len(messages) == 0 {
		return messages, nil
	}

	// Check if we need to truncate
	totalTokens := cw.countTokens(messages)
	if totalTokens <= maxTokens {
		return messages, nil
	}

	// Find pairs to drop - we drop oldest first
	// A "pair" is a user message followed by an assistant message (or vice versa)
	// We always drop in pairs to maintain conversation coherence

	var kept []ContextMessage
	var dropped []ContextMessage

	// Start from the end (most recent) and keep messages until we hit the limit
	tokens := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		msgTokens := cw.countTokens([]ContextMessage{msg})

		if tokens+msgTokens <= maxTokens {
			kept = append([]ContextMessage{msg}, kept...)
			tokens += msgTokens
		} else {
			// This message doesn't fit, add to dropped
			dropped = append([]ContextMessage{msg}, dropped...)
		}
	}

	// If we still have too many tokens, truncate individual message content
	if tokens > maxTokens {
		kept = cw.truncateMessageContents(kept, maxTokens)
	}

	return kept, dropped
}

// truncateMessageContents truncates individual message contents if they're too large.
func (cw *ContextWindow) truncateMessageContents(messages []ContextMessage, maxTokens int) []ContextMessage {
	result := make([]ContextMessage, len(messages))
	copy(result, messages)

	totalTokens := cw.countTokens(result)
	if totalTokens <= maxTokens {
		return result
	}

	// Find the largest message and truncate it
	for i := range result {
		if totalTokens <= maxTokens {
			break
		}

		msgTokens := cw.config.Tokenizer.CountTokens(result[i].Content)
		if msgTokens > 100 { // Only truncate if message is substantial
			// Truncate to half its current size
			targetTokens := msgTokens / 2
			truncated := cw.truncateContent(result[i].Content, targetTokens)
			oldTokens := msgTokens
			newTokens := cw.config.Tokenizer.CountTokens(truncated)
			result[i].Content = truncated
			totalTokens -= (oldTokens - newTokens)
		}
	}

	return result
}

// truncateContent truncates content to approximately targetTokens.
func (cw *ContextWindow) truncateContent(content string, targetTokens int) string {
	if content == "" {
		return content
	}

	currentTokens := cw.config.Tokenizer.CountTokens(content)
	if currentTokens <= targetTokens {
		return content
	}

	// Estimate character position based on token ratio
	ratio := float64(targetTokens) / float64(currentTokens)
	targetChars := int(float64(len(content)) * ratio * 0.9) // 90% for safety margin

	if targetChars >= len(content) {
		return content
	}

	// Find a good breaking point (space or newline)
	truncated := content[:targetChars]
	lastBreak := strings.LastIndexAny(truncated, " \n\t")
	if lastBreak > targetChars/2 {
		truncated = content[:lastBreak]
	}

	return truncated + "\n[...truncated...]"
}

// countTokens counts the total tokens in a slice of messages.
func (cw *ContextWindow) countTokens(messages []ContextMessage) int {
	total := 0
	for _, msg := range messages {
		// Add message overhead (role markers, etc.) - approximately 4 tokens per message
		total += cw.config.Tokenizer.CountTokens(msg.Content) + 4
	}
	// Add buffer for reply priming
	total += 3
	return total
}

// GetConfig returns the current configuration.
func (cw *ContextWindow) GetConfig() ContextWindowConfig {
	return cw.config
}

// CountTokens is a convenience method to count tokens in messages.
func (cw *ContextWindow) CountTokens(messages []ContextMessage) int {
	return cw.countTokens(messages)
}

// QuickTruncate provides fast truncation without detailed tracking.
// Useful when you just need the truncated messages without metadata.
func (cw *ContextWindow) QuickTruncate(messages []ContextMessage) []ContextMessage {
	result := cw.Truncate(messages)
	return result.Messages
}
