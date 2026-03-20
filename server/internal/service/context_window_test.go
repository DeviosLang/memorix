package service

import (
	"strings"
	"testing"

	"github.com/devioslang/memorix/server/internal/tokenizer"
)

func TestNewContextWindow(t *testing.T) {
	cfg := ContextWindowConfig{
		MaxTokens:                  4096,
		SystemPromptReservedTokens: 200,
		MemoryReservedTokens:       500,
		Tokenizer:                  tokenizer.NewDefault(),
	}

	cw := NewContextWindow(cfg)
	if cw == nil {
		t.Fatal("expected non-nil context window")
	}

	// Test defaults are applied
	cw2 := NewContextWindow(ContextWindowConfig{})
	if cw2.config.MaxTokens != 8192 {
		t.Errorf("expected default MaxTokens 8192, got %d", cw2.config.MaxTokens)
	}
	if cw2.config.Tokenizer == nil {
		t.Error("expected default tokenizer to be set")
	}
}

func TestTruncateNoOp(t *testing.T) {
	cw := NewContextWindow(ContextWindowConfig{
		MaxTokens:                  10000,
		SystemPromptReservedTokens: 100,
		MemoryReservedTokens:       200,
	})

	messages := []ContextMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result := cw.Truncate(messages)

	if len(result.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.Truncated {
		t.Error("expected no truncation")
	}
	if result.MessagesDropped != 0 {
		t.Errorf("expected 0 messages dropped, got %d", result.MessagesDropped)
	}
}

func TestTruncatePreservesSystemAndMemory(t *testing.T) {
	cw := NewContextWindow(ContextWindowConfig{
		MaxTokens:                  500,
		SystemPromptReservedTokens: 100,
		MemoryReservedTokens:       100,
	})

	// System and memory messages that should be preserved
	system := ContextMessage{Role: "system", Content: "You are a helpful assistant. " + strings.Repeat("Important system context. ", 10)}
	memory := ContextMessage{Role: "memory", Content: "Relevant memories: " + strings.Repeat("Memory item. ", 20)}

	// Long conversation that will need truncation
	messages := []ContextMessage{system, memory}
	for i := 0; i < 20; i++ {
		messages = append(messages,
			ContextMessage{Role: "user", Content: "This is user message number " + string(rune('0'+i%10)) + " " + strings.Repeat("content ", 20)},
			ContextMessage{Role: "assistant", Content: "This is assistant response " + string(rune('0'+i%10)) + " " + strings.Repeat("content ", 20)},
		)
	}

	result := cw.Truncate(messages)

	// System message should always be first
	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if result.Messages[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", result.Messages[0].Role)
	}

	// Memory message should be preserved
	memoryFound := false
	for _, msg := range result.Messages {
		if msg.Role == "memory" {
			memoryFound = true
			break
		}
	}
	if !memoryFound {
		t.Error("expected memory message to be preserved")
	}

	// Should have dropped some messages
	if !result.Truncated {
		t.Error("expected truncation to occur")
	}
	if result.MessagesDropped == 0 {
		t.Error("expected some messages to be dropped")
	}
}

func TestTruncateDropsOldestFirst(t *testing.T) {
	cw := NewContextWindow(ContextWindowConfig{
		MaxTokens:                  200,
		SystemPromptReservedTokens: 20,
		MemoryReservedTokens:       20,
	})

	// Create messages where order matters - make messages longer so truncation occurs
	messages := []ContextMessage{
		{Role: "user", Content: "First user message " + strings.Repeat("padding content here ", 10)},
		{Role: "assistant", Content: "First response " + strings.Repeat("padding content here ", 10)},
		{Role: "user", Content: "Second user message " + strings.Repeat("padding content here ", 10)},
		{Role: "assistant", Content: "Second response " + strings.Repeat("padding content here ", 10)},
		{Role: "user", Content: "Third user message " + strings.Repeat("padding content here ", 10)},
		{Role: "assistant", Content: "Third response " + strings.Repeat("padding content here ", 10)},
	}

	result := cw.Truncate(messages)

	// Check that truncation occurred
	if !result.Truncated {
		t.Error("expected truncation to occur")
	}

	// Check that oldest messages were dropped (they have "First" in content)
	for _, msg := range result.Messages {
		if strings.Contains(msg.Content, "First user") || strings.Contains(msg.Content, "First response") {
			t.Error("expected first message pair to be dropped")
		}
	}

	// Check that at least some newer messages were kept
	keptMessages := 0
	for _, msg := range result.Messages {
		if strings.Contains(msg.Content, "Second") || strings.Contains(msg.Content, "Third") {
			keptMessages++
		}
	}
	if keptMessages == 0 {
		t.Error("expected at least some second or third messages to be kept")
	}
}

func TestTruncationResultMetadata(t *testing.T) {
	cw := NewContextWindow(ContextWindowConfig{
		MaxTokens:                  200,
		SystemPromptReservedTokens: 50,
		MemoryReservedTokens:       50,
	})

	// Create messages that will definitely be truncated
	messages := []ContextMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: strings.Repeat("Long user message content ", 50)},
		{Role: "assistant", Content: strings.Repeat("Long assistant response ", 50)},
		{Role: "user", Content: strings.Repeat("Another long user message ", 50)},
		{Role: "assistant", Content: strings.Repeat("Another long assistant response ", 50)},
	}

	result := cw.Truncate(messages)

	if result.OriginalTokens <= 0 {
		t.Errorf("expected positive OriginalTokens, got %d", result.OriginalTokens)
	}
	if result.FinalTokens <= 0 {
		t.Errorf("expected positive FinalTokens, got %d", result.FinalTokens)
	}
	if result.FinalTokens > result.OriginalTokens {
		t.Errorf("FinalTokens %d should not exceed OriginalTokens %d", result.FinalTokens, result.OriginalTokens)
	}
	if result.MessagesDropped <= 0 {
		t.Error("expected some messages to be dropped")
	}
	if result.DroppedRoles == nil && result.MessagesDropped > 0 {
		t.Error("expected DroppedRoles to be populated when messages are dropped")
	}
}

func TestCountTokens(t *testing.T) {
	cw := NewContextWindow(DefaultContextWindowConfig())

	messages := []ContextMessage{
		{Role: "user", Content: "Hello, world!"},
		{Role: "assistant", Content: "Hi there!"},
	}

	count := cw.CountTokens(messages)
	if count < 5 {
		t.Errorf("expected at least 5 tokens, got %d", count)
	}
}

func TestQuickTruncate(t *testing.T) {
	cw := NewContextWindow(ContextWindowConfig{
		MaxTokens:                  200,
		SystemPromptReservedTokens: 50,
		MemoryReservedTokens:       50,
	})

	messages := []ContextMessage{
		{Role: "system", Content: "System"},
		{Role: "user", Content: strings.Repeat("content ", 100)},
		{Role: "assistant", Content: strings.Repeat("response ", 100)},
	}

	truncated := cw.QuickTruncate(messages)

	// Should return fewer messages
	if len(truncated) >= len(messages) {
		t.Error("expected truncation to reduce message count")
	}

	// System should still be present
	if len(truncated) > 0 && truncated[0].Role != "system" {
		t.Error("expected system message to be preserved")
	}
}

func TestEmptyMessages(t *testing.T) {
	cw := NewContextWindow(DefaultContextWindowConfig())

	result := cw.Truncate([]ContextMessage{})

	if len(result.Messages) != 0 {
		t.Error("expected empty result for empty input")
	}
	if result.Truncated {
		t.Error("expected no truncation for empty input")
	}
}

func TestSingleMessage(t *testing.T) {
	cw := NewContextWindow(DefaultContextWindowConfig())

	messages := []ContextMessage{
		{Role: "user", Content: "Hello"},
	}

	result := cw.Truncate(messages)

	if len(result.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Truncated {
		t.Error("expected no truncation for single short message")
	}
}

func TestMaxTokensLimit(t *testing.T) {
	cw := NewContextWindow(ContextWindowConfig{
		MaxTokens:                  100,
		SystemPromptReservedTokens: 20,
		MemoryReservedTokens:       20,
	})

	// Create a very large conversation
	messages := []ContextMessage{
		{Role: "system", Content: "System prompt"},
	}
	for i := 0; i < 50; i++ {
		messages = append(messages,
			ContextMessage{Role: "user", Content: strings.Repeat("User content ", 30)},
			ContextMessage{Role: "assistant", Content: strings.Repeat("Assistant response ", 30)},
		)
	}

	result := cw.Truncate(messages)

	// Final tokens should be within limit
	if result.FinalTokens > cw.config.MaxTokens+50 { // Allow some buffer for overhead calculation
		t.Errorf("FinalTokens %d exceeds MaxTokens %d", result.FinalTokens, cw.config.MaxTokens)
	}
}
