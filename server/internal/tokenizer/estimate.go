package tokenizer

import (
	"fmt"
	"math"
	"unicode/utf8"
)

// estimateTokenizer implements Tokenizer using character-based estimation.
// This is a fallback for when tiktoken is not available or for models
// where the exact tokenizer is not known.
type estimateTokenizer struct {
	charsPerToken float64
	name          string
}

// newEstimate creates a new estimation-based tokenizer.
func newEstimate(charsPerToken float64) Tokenizer {
	return &estimateTokenizer{
		charsPerToken: charsPerToken,
		name:          fmt.Sprintf("estimate(%.1f)", charsPerToken),
	}
}

// CountTokens returns an estimated token count based on character count.
func (t *estimateTokenizer) CountTokens(text string) int {
	return estimateTokens(text, t.charsPerToken)
}

// Name returns the tokenizer name.
func (t *estimateTokenizer) Name() string {
	return t.name
}

// estimateTokens calculates estimated token count from text.
func estimateTokens(text string, charsPerToken float64) int {
	if text == "" {
		return 0
	}

	// Use rune count for proper Unicode handling
	charCount := utf8.RuneCountInString(text)

	// Add overhead for whitespace and special characters
	// (they often require more tokens)
	adjusted := float64(charCount) * 1.05

	// Calculate tokens
	tokens := int(math.Ceil(adjusted / charsPerToken))

	// Minimum 1 token for non-empty text
	if tokens < 1 {
		return 1
	}

	return tokens
}

// AnthropicEstimate provides token estimation specific to Anthropic models.
// Anthropic uses a different tokenizer with approximately 3.5 chars per token
// for English text, but can be higher for code or other languages.
func AnthropicEstimate(text string) int {
	if text == "" {
		return 0
	}

	// Anthropic's tokenizer is closer to 3.5 chars per token
	return estimateTokens(text, 3.5)
}

// AnthropicMessagesEstimate estimates tokens for Anthropic-style messages.
// Anthropic uses a different message format than OpenAI, so we account for that.
func AnthropicMessagesEstimate(messages []Message) int {
	total := 0
	for _, m := range messages {
		// Anthropic's message format overhead is different
		// Each message has role and content markers
		total += AnthropicEstimate(m.Content) + AnthropicEstimate(m.Role) + 5
	}
	// Add some buffer for system prompt handling
	return total + 10
}
