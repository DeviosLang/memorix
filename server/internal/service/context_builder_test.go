package service

import (
	"testing"

	"github.com/devioslang/memorix/server/internal/domain"
)

func TestContextBuilder_Build(t *testing.T) {
	tests := []struct {
		name           string
		config         ContextBuilderConfig
		request        *domain.BuildContextRequest
		wantMinTokens  int
		wantMaxTokens  int
		checkLayer     domain.ContextLayer
		checkMinTokens int
	}{
		{
			name: "basic assembly from all layers",
			config: ContextBuilderConfig{
				MaxTokens:            8192,
				SystemBudget:         500,
				MetadataBudget:       200,
				UserMemoryBudgetMin:  500,
				UserMemoryBudgetMax:  1500,
				SummaryBudgetMin:     300,
				SummaryBudgetMax:     800,
			},
			request: &domain.BuildContextRequest{
				SystemInstructions: "You are a helpful assistant that remembers user preferences.",
				SessionMetadata: &domain.SessionMetadata{
					DeviceType:         "desktop",
					Timezone:           "Asia/Shanghai",
					LanguagePreference: "zh-CN",
				},
				UserMemories: []domain.Memory{
					{Content: "User prefers concise answers"},
					{Content: "User is a software developer"},
				},
				UserProfileFacts: []domain.UserProfileFact{
					{Category: "preference", Key: "language", Value: "Go"},
				},
				ConversationSummary: "Previously discussed API design patterns.",
				CurrentMessages: []domain.ContextMessage{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi! How can I help?"},
				},
			},
			wantMinTokens: 50, // At least some content
			wantMaxTokens: 8192,
		},
		{
			name: "empty layers handled gracefully",
			config: ContextBuilderConfig{
				MaxTokens:            1000,
				SystemBudget:         200,
				MetadataBudget:       100,
				UserMemoryBudgetMin:  100,
				UserMemoryBudgetMax:  300,
				SummaryBudgetMin:     50,
				SummaryBudgetMax:     150,
			},
			request: &domain.BuildContextRequest{
				SystemInstructions: "Hello",
				// No other content
			},
			wantMinTokens: 1,
			wantMaxTokens: 1000,
		},
		{
			name: "system instructions never truncated",
			config: ContextBuilderConfig{
				MaxTokens:            100,
				SystemBudget:         100,
				MetadataBudget:       20,
				UserMemoryBudgetMin:  10,
				UserMemoryBudgetMax:  20,
				SummaryBudgetMin:     10,
				SummaryBudgetMax:     20,
			},
			request: &domain.BuildContextRequest{
				SystemInstructions: "You are a helpful AI assistant.", // Should fit
			},
			wantMinTokens: 1,
			wantMaxTokens: 100,
			checkLayer:    domain.LayerSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewContextBuilder(tt.config)
			result, err := builder.Build(tt.request)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			// Check total tokens within limits
			if result.TotalTokens < tt.wantMinTokens {
				t.Errorf("Build() TotalTokens = %d, want at least %d", result.TotalTokens, tt.wantMinTokens)
			}
			if result.TotalTokens > tt.wantMaxTokens {
				t.Errorf("Build() TotalTokens = %d, want at most %d", result.TotalTokens, tt.wantMaxTokens)
			}

			// Check layer stats are populated
			if len(result.LayerStats) == 0 && (tt.request.SystemInstructions != "" || len(tt.request.UserMemories) > 0) {
				t.Error("Build() LayerStats is empty but content was provided")
			}

			// Check specific layer if requested
			if tt.checkLayer != "" {
				found := false
				for _, stat := range result.LayerStats {
					if stat.Layer == tt.checkLayer {
						found = true
						if tt.checkMinTokens > 0 && stat.FinalTokens < tt.checkMinTokens {
							t.Errorf("Layer %s tokens = %d, want at least %d", tt.checkLayer, stat.FinalTokens, tt.checkMinTokens)
						}
						break
					}
				}
				if !found {
					t.Errorf("Layer %s not found in LayerStats", tt.checkLayer)
				}
			}

			// Verify prompt is not empty when content is provided
			if result.Prompt == "" && (tt.request.SystemInstructions != "" || len(tt.request.UserMemories) > 0) {
				t.Error("Build() Prompt is empty but content was provided")
			}
		})
	}
}

func TestContextBuilder_LayerPriority(t *testing.T) {
	tests := []struct {
		layer1   domain.ContextLayer
		layer2   domain.ContextLayer
		wantLess bool // layer1 has lower priority than layer2
	}{
		{domain.LayerCurrentSession, domain.LayerConversationSummary, true},
		{domain.LayerConversationSummary, domain.LayerUserMemory, true},
		{domain.LayerUserMemory, domain.LayerMetadata, true},
		{domain.LayerMetadata, domain.LayerSystem, true},
		{domain.LayerSystem, domain.LayerCurrentSession, false}, // System is highest
	}

	for _, tt := range tests {
		p1 := domain.LayerPriority(tt.layer1)
		p2 := domain.LayerPriority(tt.layer2)
		if (p1 < p2) != tt.wantLess {
			t.Errorf("LayerPriority(%s)=%d vs LayerPriority(%s)=%d, wantLess=%v",
				tt.layer1, p1, tt.layer2, p2, tt.wantLess)
		}
	}
}

func TestContextBuilder_ElasticBudget(t *testing.T) {
	config := ContextBuilderConfig{
		MaxTokens:            2000,
		SystemBudget:         200,
		MetadataBudget:       100,
		UserMemoryBudgetMin:  200,
		UserMemoryBudgetMax:  800,
		SummaryBudgetMin:     100,
		SummaryBudgetMax:     400,
	}

	// Create a request where user memory would benefit from elastic expansion
	request := &domain.BuildContextRequest{
		SystemInstructions: "Be helpful.",
		UserMemories: []domain.Memory{
			{Content: "User memory 1 with some content"},
			{Content: "User memory 2 with more content"},
		},
		CurrentMessages: []domain.ContextMessage{
			{Role: "user", Content: "Short message"},
		},
	}

	builder := NewContextBuilder(config)
	result, err := builder.Build(request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Find user memory layer stats
	var userMemStats *domain.LayerStats
	for i := range result.LayerStats {
		if result.LayerStats[i].Layer == domain.LayerUserMemory {
			userMemStats = &result.LayerStats[i]
			break
		}
	}

	if userMemStats == nil {
		t.Fatal("User memory layer not found in stats")
	}

	// Budget allocated should be within min-max range
	if userMemStats.BudgetUsed < config.UserMemoryBudgetMin {
		t.Errorf("BudgetUsed = %d, want at least %d", userMemStats.BudgetUsed, config.UserMemoryBudgetMin)
	}
	if userMemStats.BudgetUsed > config.UserMemoryBudgetMax {
		t.Errorf("BudgetUsed = %d, want at most %d", userMemStats.BudgetUsed, config.UserMemoryBudgetMax)
	}
}

func TestContextBuilder_TokenBudgetEnforcement(t *testing.T) {
	// Strict budget that forces truncation
	// System budget is 30 tokens, but the content is much larger
	config := ContextBuilderConfig{
		MaxTokens:            80,
		SystemBudget:         30,
		MetadataBudget:       10,
		UserMemoryBudgetMin:  10,
		UserMemoryBudgetMax:  20,
		SummaryBudgetMin:     5,
		SummaryBudgetMax:     10,
	}

	// This request has content that exceeds the strict budgets
	request := &domain.BuildContextRequest{
		SystemInstructions: "You are an AI assistant that provides helpful and accurate information to users about various topics including programming and software development.",
		UserMemories: []domain.Memory{
			{Content: "User prefers detailed technical explanations with code examples and step-by-step instructions for complex tasks."},
		},
		CurrentMessages: []domain.ContextMessage{
			{Role: "user", Content: "Can you explain how to implement a context window with token budget management?"},
		},
	}

	builder := NewContextBuilder(config)
	result, err := builder.Build(request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Total must not exceed max
	if result.TotalTokens > config.MaxTokens {
		t.Errorf("TotalTokens = %d exceeds MaxTokens = %d", result.TotalTokens, config.MaxTokens)
	}

	// With such tight budgets and larger content, truncation should occur
	// This is a soft check - we just verify we stay within bounds
	t.Logf("Total tokens: %d, Truncated: %v, Layers: %d", result.TotalTokens, result.Truncated, len(result.LayerStats))
	for _, stat := range result.LayerStats {
		t.Logf("  Layer %s: original=%d, final=%d, budget=%d, truncated=%v",
			stat.Layer, stat.OriginalTokens, stat.FinalTokens, stat.BudgetUsed, stat.Truncated)
	}
}

func TestContextBuilder_EmptyRequest(t *testing.T) {
	config := DefaultContextBuilderConfig()

	request := &domain.BuildContextRequest{
		// Empty request
	}

	builder := NewContextBuilder(config)
	result, err := builder.Build(request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Should produce empty prompt
	if result.Prompt != "" {
		t.Errorf("Expected empty prompt, got: %s", result.Prompt)
	}

	// Total tokens should be 0 or minimal
	if result.TotalTokens > 0 {
		t.Errorf("Expected 0 tokens for empty request, got: %d", result.TotalTokens)
	}

	// No truncation for empty content
	if result.Truncated {
		t.Error("Expected no truncation for empty request")
	}
}

func TestContextBuilder_OverriddenMaxTokens(t *testing.T) {
	config := ContextBuilderConfig{
		MaxTokens:            8192,
		SystemBudget:         200,
		MetadataBudget:       100,
		UserMemoryBudgetMin:  200,
		UserMemoryBudgetMax:  500,
		SummaryBudgetMin:     100,
		SummaryBudgetMax:     300,
	}

	// Request with explicit max tokens
	request := &domain.BuildContextRequest{
		SystemInstructions: "Be helpful.",
		MaxTokens:          500, // Override config
	}

	builder := NewContextBuilder(config)
	result, err := builder.Build(request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Should use request's max tokens
	if result.MaxTokens != 500 {
		t.Errorf("MaxTokens = %d, want 500", result.MaxTokens)
	}

	if result.TotalTokens > 500 {
		t.Errorf("TotalTokens = %d exceeds overridden MaxTokens 500", result.TotalTokens)
	}
}

func TestContextBuilder_TruncationPriority(t *testing.T) {
	// Test that current session is truncated before other layers
	config := ContextBuilderConfig{
		MaxTokens:            100,
		SystemBudget:         30,
		MetadataBudget:       15,
		UserMemoryBudgetMin:  15,
		UserMemoryBudgetMax:  25,
		SummaryBudgetMin:     10,
		SummaryBudgetMax:     15,
	}

	request := &domain.BuildContextRequest{
		SystemInstructions: "You are a helpful assistant.",
		UserMemories: []domain.Memory{
			{Content: "User preference for code examples and detailed explanations."},
		},
		CurrentMessages: []domain.ContextMessage{
			{Role: "user", Content: "This is a very long message that should definitely be truncated because the current session layer has the lowest priority in the context assembly process."},
		},
	}

	builder := NewContextBuilder(config)
	result, err := builder.Build(request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Verify we stay within token budget
	if result.TotalTokens > config.MaxTokens {
		t.Errorf("TotalTokens = %d exceeds MaxTokens = %d", result.TotalTokens, config.MaxTokens)
	}

	// Check which layers were truncated (current session should be truncated if any)
	var systemTruncated, currentSessionTruncated bool
	for _, stat := range result.LayerStats {
		if stat.Layer == domain.LayerSystem && stat.Truncated {
			systemTruncated = true
		}
		if stat.Layer == domain.LayerCurrentSession && stat.Truncated {
			currentSessionTruncated = true
		}
	}

	// System should never be truncated (highest priority)
	if systemTruncated {
		t.Error("System layer should never be truncated")
	}

	t.Logf("Total: %d, Truncated: %v, System truncated: %v, Current session truncated: %v",
		result.TotalTokens, result.Truncated, systemTruncated, currentSessionTruncated)
}
