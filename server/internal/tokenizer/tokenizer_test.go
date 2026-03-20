package tokenizer

import (
	"testing"
)

func TestNewDefault(t *testing.T) {
	tok := NewDefault()
	if tok == nil {
		t.Fatal("expected non-nil tokenizer")
	}
	// Should return some tokenizer (tiktoken or estimate fallback)
	if tok.Name() == "" {
		t.Error("expected non-empty tokenizer name")
	}
}

func TestEstimateTokenizer(t *testing.T) {
	tok, err := New(Config{Type: TypeEstimate})
	if err != nil {
		t.Fatalf("failed to create estimate tokenizer: %v", err)
	}

	tests := []struct {
		text     string
		minToken int
		maxToken int
	}{
		{"", 0, 0},
		{"hello", 1, 5},
		{"hello world", 2, 6},
		{"The quick brown fox jumps over the lazy dog", 8, 20},
		{repeat("x", 1000), 200, 300},
	}

	for _, tt := range tests {
		count := tok.CountTokens(tt.text)
		if count < tt.minToken || count > tt.maxToken {
			t.Errorf("CountTokens(%q) = %d, want between %d and %d", tt.text, count, tt.minToken, tt.maxToken)
		}
	}
}

func TestTiktokenTokenizer(t *testing.T) {
	tok, err := New(Config{Type: TypeTiktoken, Encoding: "cl100k_base"})
	if err != nil {
		t.Skipf("tiktoken not available: %v", err)
	}

	// Known token counts for cl100k_base
	tests := []struct {
		text      string
		wantRange [2]int // acceptable range
	}{
		{"", [2]int{0, 0}},
		{"hello", [2]int{1, 1}},
		{"hello world", [2]int{2, 3}},
		{"The quick brown fox jumps over the lazy dog", [2]int{9, 10}},
	}

	for _, tt := range tests {
		count := tok.CountTokens(tt.text)
		if count < tt.wantRange[0] || count > tt.wantRange[1] {
			t.Errorf("CountTokens(%q) = %d, want between %d and %d",
				tt.text, count, tt.wantRange[0], tt.wantRange[1])
		}
	}
}

func TestCountMessagesTokens(t *testing.T) {
	tok := NewDefault()

	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	count := CountMessagesTokens(tok, messages)
	if count < 5 {
		t.Errorf("CountMessagesTokens returned %d, expected at least 5", count)
	}
}

func TestQuickCount(t *testing.T) {
	tests := []struct {
		text string
		min  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
	}

	for _, tt := range tests {
		count := QuickCount(tt.text)
		if count < tt.min {
			t.Errorf("QuickCount(%q) = %d, want at least %d", tt.text, count, tt.min)
		}
	}
}

func TestEncodingForModel(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"gpt-4", "cl100k_base"},
		{"gpt-4-turbo", "cl100k_base"},
			{"gpt-4o", "cl100k_base"}, // gpt-4o uses cl100k_base
		{"gpt-3.5-turbo", "cl100k_base"},
		{"o1-preview", "o200k_base"},
		{"unknown-model", "cl100k_base"},
	}

	for _, tt := range tests {
		got := encodingForModel(tt.model)
		if got != tt.want {
			t.Errorf("encodingForModel(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestAnthropicEstimate(t *testing.T) {
	// Test that Anthropic estimation produces reasonable results
	tests := []struct {
		text string
		min  int
	}{
		{"", 0},
		{"hello", 1},
		{"The quick brown fox jumps over the lazy dog", 8},
	}

	for _, tt := range tests {
		count := AnthropicEstimate(tt.text)
		if count < tt.min {
			t.Errorf("AnthropicEstimate(%q) = %d, want at least %d", tt.text, count, tt.min)
		}
	}
}

func TestUnicodeHandling(t *testing.T) {
	tok := NewDefault()

	// Test with Unicode characters
	unicodeTests := []struct {
		text string
		min  int
	}{
		{"你好世界", 1}, // Chinese
		{"こんにちは", 1}, // Japanese
		{"🎉🎊🎁", 1},   // Emoji
		{"Hello 你好", 1}, // Mixed
	}

	for _, tt := range unicodeTests {
		count := tok.CountTokens(tt.text)
		if count < tt.min {
			t.Errorf("CountTokens(%q) = %d, want at least %d", tt.text, count, tt.min)
		}
	}
}

// Helper function
func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
