package tokenizer

import (
	"fmt"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// tiktokenTokenizer implements Tokenizer using OpenAI's tiktoken.
type tiktokenTokenizer struct {
	encoding *tiktoken.Tiktoken
	name     string
	mu       sync.RWMutex
}

// newTiktoken creates a new tiktoken-based tokenizer.
func newTiktoken(encoding string) (Tokenizer, error) {
	enc, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding %s: %w", encoding, err)
	}

	return &tiktokenTokenizer{
		encoding: enc,
		name:     encoding,
	}, nil
}

// CountTokens returns the number of tokens in the given text.
func (t *tiktokenTokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.encoding.Encode(text, nil, nil))
}

// Name returns the tokenizer name.
func (t *tiktokenTokenizer) Name() string {
	return t.name
}
