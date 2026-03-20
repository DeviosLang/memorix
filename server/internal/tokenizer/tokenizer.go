// Package tokenizer provides token counting for various LLM tokenizers.
// It supports OpenAI's tiktoken (cl100k_base) and provides estimation-based
// fallbacks for other providers like Anthropic.
package tokenizer

import (
	"fmt"
	"strings"
)

// Tokenizer defines the interface for counting tokens in text.
type Tokenizer interface {
	// CountTokens returns the number of tokens in the given text.
	CountTokens(text string) int

	// Name returns the tokenizer name (e.g., "cl100k_base", "estimate").
	Name() string
}

// TokenizerType represents the type of tokenizer to use.
type TokenizerType string

const (
	// TypeTiktoken uses OpenAI's tiktoken encoding.
	TypeTiktoken TokenizerType = "tiktoken"
	// TypeEstimate uses character-based estimation (roughly 4 chars per token).
	TypeEstimate TokenizerType = "estimate"
)

// Config holds tokenizer configuration.
type Config struct {
	// Type specifies which tokenizer to use.
	Type TokenizerType

	// Model specifies the model name for model-specific encoding selection.
	// For tiktoken, this determines the encoding (e.g., "gpt-4" -> cl100k_base).
	Model string

	// Encoding explicitly specifies the encoding name (e.g., "cl100k_base").
	// If set, this takes precedence over Model.
	Encoding string

	// CharsPerToken is used for estimation-based tokenizers.
	// Default is 4 (rough approximation for English text).
	CharsPerToken float64
}

// New creates a new tokenizer based on the configuration.
func New(cfg Config) (Tokenizer, error) {
	// Normalize type
	t := cfg.Type
	if t == "" {
		t = TypeTiktoken
	}

	switch t {
	case TypeTiktoken:
		encoding := cfg.Encoding
		if encoding == "" {
			encoding = encodingForModel(cfg.Model)
		}
		return newTiktoken(encoding)
	case TypeEstimate:
		charsPerToken := cfg.CharsPerToken
		if charsPerToken <= 0 {
			charsPerToken = 4.0 // Default: ~4 chars per token
		}
		return newEstimate(charsPerToken), nil
	default:
		return nil, fmt.Errorf("unknown tokenizer type: %s", t)
	}
}

// NewDefault creates a tokenizer with sensible defaults.
// Uses tiktoken with cl100k_base encoding (GPT-4/GPT-4-turbo/GPT-3.5-turbo).
func NewDefault() Tokenizer {
	t, err := New(Config{Type: TypeTiktoken, Encoding: "cl100k_base"})
	if err != nil {
		// Fall back to estimation if tiktoken fails to load
		return newEstimate(4.0)
	}
	return t
}

// encodingForModel returns the tiktoken encoding name for a given model.
func encodingForModel(model string) string {
	// Normalize model name
	model = strings.ToLower(model)
	model = strings.TrimPrefix(model, "openai/")
	model = strings.TrimPrefix(model, "gpt-")

	// GPT-4 family uses cl100k_base
	if strings.HasPrefix(model, "4") || strings.HasPrefix(model, "4o") {
		return "cl100k_base"
	}
	// GPT-3.5-turbo uses cl100k_base
	if strings.HasPrefix(model, "3.5") {
		return "cl100k_base"
	}
	// o1 family uses o200k_base
	if strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") {
		return "o200k_base"
	}
	// GPT-4o family uses cl100k_base (for compatibility)
	if strings.HasPrefix(model, "4o") {
		return "cl100k_base"
	}

	// Default to cl100k_base (most common for current models)
	return "cl100k_base"
}

// CountMessagesTokens counts the total tokens in a slice of messages.
// This is a convenience function for the common pattern of counting
// tokens across an entire conversation.
func CountMessagesTokens(t Tokenizer, messages []Message) int {
	total := 0
	for _, m := range messages {
		// Every message follows <im_start>{role/name}\n{content}<im_end>\n format
		// This adds approximately 4 tokens of overhead per message
		total += t.CountTokens(m.Content) + 4
	}
	// Every reply is primed with <im_start>assistant
	total += 3
	return total
}

// Message represents a chat message with role and content.
type Message struct {
	Role    string
	Content string
}

// QuickCount provides a fast token estimation without loading a tokenizer.
// Uses character-based estimation (~4 chars per token).
// Useful for quick checks where exact counts aren't critical.
func QuickCount(text string) int {
	return estimateTokens(text, 4.0)
}

// QuickCountMessages provides fast token estimation for messages.
func QuickCountMessages(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += QuickCount(m.Content) + 4
	}
	return total + 3
}
