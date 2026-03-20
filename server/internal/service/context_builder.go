package service

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/tokenizer"
)

// ContextBuilderConfig holds configuration for the context builder.
type ContextBuilderConfig struct {
	// MaxTokens is the maximum total tokens for the assembled prompt.
	MaxTokens int

	// SystemBudget is the fixed token budget for system instructions.
	// Default: 500 tokens
	SystemBudget int

	// MetadataBudget is the fixed token budget for session metadata.
	// Default: 200 tokens
	MetadataBudget int

	// UserMemoryBudgetMin is the minimum tokens for user memory layer.
	// Default: 500 tokens
	UserMemoryBudgetMin int

	// UserMemoryBudgetMax is the maximum tokens for user memory layer.
	// Default: 1500 tokens
	UserMemoryBudgetMax int

	// SummaryBudgetMin is the minimum tokens for conversation summary layer.
	// Default: 300 tokens
	SummaryBudgetMin int

	// SummaryBudgetMax is the maximum tokens for conversation summary layer.
	// Default: 800 tokens
	SummaryBudgetMax int

	// Tokenizer is used for counting tokens.
	Tokenizer tokenizer.Tokenizer
}

// DefaultContextBuilderConfig returns sensible defaults.
func DefaultContextBuilderConfig() ContextBuilderConfig {
	return ContextBuilderConfig{
		MaxTokens:            8192,
		SystemBudget:         500,
		MetadataBudget:       200,
		UserMemoryBudgetMin:  500,
		UserMemoryBudgetMax:  1500,
		SummaryBudgetMin:     300,
		SummaryBudgetMax:     800,
		Tokenizer:            tokenizer.NewDefault(),
	}
}

// ContextBuilder assembles system prompts from multiple context layers.
// It implements a layered injection strategy with priority-based truncation:
//
// Priority order (highest to lowest):
// 1. System instructions - never truncated
// 2. Session metadata - rarely truncated
// 3. User memory - elastic budget
// 4. Conversation summary - elastic budget
// 5. Current session - truncated first
type ContextBuilder struct {
	config    ContextBuilderConfig
	tokenizer tokenizer.Tokenizer
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder(config ContextBuilderConfig) *ContextBuilder {
	if config.Tokenizer == nil {
		config.Tokenizer = tokenizer.NewDefault()
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 8192
	}
	if config.SystemBudget <= 0 {
		config.SystemBudget = 500
	}
	if config.MetadataBudget <= 0 {
		config.MetadataBudget = 200
	}
	if config.UserMemoryBudgetMin <= 0 {
		config.UserMemoryBudgetMin = 500
	}
	if config.UserMemoryBudgetMax <= 0 {
		config.UserMemoryBudgetMax = 1500
	}
	if config.SummaryBudgetMin <= 0 {
		config.SummaryBudgetMin = 300
	}
	if config.SummaryBudgetMax <= 0 {
		config.SummaryBudgetMax = 800
	}

	return &ContextBuilder{
		config:    config,
		tokenizer: config.Tokenizer,
	}
}

// Build assembles the context prompt from all layers within the token budget.
func (b *ContextBuilder) Build(req *domain.BuildContextRequest) (*domain.ContextBuildResult, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = b.config.MaxTokens
	}

	// Prepare layer content
	layers := b.prepareLayers(req)

	// Calculate initial token counts
	for i := range layers {
		layers[i].TokenCount = b.tokenizer.CountTokens(layers[i].Content)
	}

	// Allocate budgets and truncate if needed
	b.allocateBudgets(layers, maxTokens)

	// Build the final prompt
	prompt := b.assemblePrompt(layers)

	// Calculate statistics
	result := b.buildResult(layers, prompt, maxTokens)

	// Log assembly details
	b.logAssembly(result)

	return result, nil
}

// layerWithStats tracks both content and statistics for a layer.
type layerWithStats struct {
	*domain.LayerContent
	originalTokens int
	finalTokens    int
	budgetAllocated int
	truncated      bool
	truncationReason string
}

// prepareLayers creates LayerContent from the build request.
func (b *ContextBuilder) prepareLayers(req *domain.BuildContextRequest) []*layerWithStats {
	var layers []*layerWithStats

	// System instructions layer
	if req.SystemInstructions != "" {
		layers = append(layers, &layerWithStats{
			LayerContent: &domain.LayerContent{
				Layer:   domain.LayerSystem,
				Content: req.SystemInstructions,
				Source:  "system_instructions",
			},
		})
	}

	// Session metadata layer
	if req.SessionMetadata != nil {
		formatter := NewSessionMetadataFormatter(SessionMetadataConfig{
			MaxTokens: b.config.MetadataBudget,
			Tokenizer: b.tokenizer,
		})
		content, _ := formatter.FormatAndValidate(req.SessionMetadata)
		if content != "" {
			layers = append(layers, &layerWithStats{
				LayerContent: &domain.LayerContent{
					Layer:   domain.LayerMetadata,
					Content: content,
					Source:  "session_metadata",
				},
			})
		}
	}

	// User memory layer (memories + profile facts)
	userMemoryContent := b.formatUserMemory(req)
	if userMemoryContent != "" {
		layers = append(layers, &layerWithStats{
			LayerContent: &domain.LayerContent{
				Layer:   domain.LayerUserMemory,
				Content: userMemoryContent,
				Source:  "user_memory",
			},
		})
	}

	// Conversation summary layer
	if req.ConversationSummary != "" {
		layers = append(layers, &layerWithStats{
			LayerContent: &domain.LayerContent{
				Layer:   domain.LayerConversationSummary,
				Content: req.ConversationSummary,
				Source:  "conversation_summary",
			},
		})
	}

	// Current session layer
	if len(req.CurrentMessages) > 0 {
		content := b.formatCurrentSession(req.CurrentMessages)
		if content != "" {
			layers = append(layers, &layerWithStats{
				LayerContent: &domain.LayerContent{
					Layer:   domain.LayerCurrentSession,
					Content: content,
					Source:  "current_session",
				},
			})
		}
	}

	return layers
}

// formatUserMemory formats user memories and profile facts into a single string.
func (b *ContextBuilder) formatUserMemory(req *domain.BuildContextRequest) string {
	var parts []string

	// Format user profile facts
	if len(req.UserProfileFacts) > 0 {
		var factLines []string
		for _, fact := range req.UserProfileFacts {
			factLines = append(factLines, fmt.Sprintf("- [%s] %s: %s", fact.Category, fact.Key, fact.Value))
		}
		parts = append(parts, "<user-profile>\n"+strings.Join(factLines, "\n")+"\n</user-profile>")
	}

	// Format user memories
	if len(req.UserMemories) > 0 {
		var memLines []string
		for _, mem := range req.UserMemories {
			memLines = append(memLines, "- "+mem.Content)
		}
		parts = append(parts, "<user-memories>\n"+strings.Join(memLines, "\n")+"\n</user-memories>")
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// formatCurrentSession formats current session messages into a string.
func (b *ContextBuilder) formatCurrentSession(messages []domain.ContextMessage) string {
	var lines []string
	for _, msg := range messages {
		var prefix string
		switch msg.Role {
		case "user":
			prefix = "User: "
		case "assistant":
			prefix = "Assistant: "
		default:
			prefix = msg.Role + ": "
		}
		lines = append(lines, prefix+msg.Content)
	}
	return strings.Join(lines, "\n")
}

// allocateBudgets allocates token budgets and truncates layers as needed.
func (b *ContextBuilder) allocateBudgets(layers []*layerWithStats, maxTokens int) {
	// Sort layers by priority (highest first)
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Priority() > layers[j].Priority()
	})

	// Calculate fixed budgets
	fixedBudget := b.config.SystemBudget + b.config.MetadataBudget

	// Calculate remaining budget for elastic layers
	remainingBudget := maxTokens - fixedBudget

	// Track allocated tokens
	allocated := 0

	// First pass: allocate minimum budgets for elastic layers
	for _, layer := range layers {
		switch layer.Layer {
		case domain.LayerSystem:
			layer.budgetAllocated = b.config.SystemBudget
		case domain.LayerMetadata:
			layer.budgetAllocated = b.config.MetadataBudget
		case domain.LayerUserMemory:
			layer.budgetAllocated = b.config.UserMemoryBudgetMin
			remainingBudget -= b.config.UserMemoryBudgetMin
		case domain.LayerConversationSummary:
			layer.budgetAllocated = b.config.SummaryBudgetMin
			remainingBudget -= b.config.SummaryBudgetMin
		case domain.LayerCurrentSession:
			// Will get remaining tokens
			layer.budgetAllocated = 0
		}
		allocated += layer.budgetAllocated
		layer.originalTokens = layer.TokenCount
		layer.finalTokens = layer.TokenCount
	}

	// Second pass: expand elastic budgets if we have room
	if remainingBudget > 0 {
		for _, layer := range layers {
			if layer.Layer == domain.LayerUserMemory && layer.TokenCount > layer.budgetAllocated {
				extra := min(layer.TokenCount-layer.budgetAllocated,
					b.config.UserMemoryBudgetMax-layer.budgetAllocated,
					remainingBudget)
				layer.budgetAllocated += extra
				remainingBudget -= extra
			}
			if layer.Layer == domain.LayerConversationSummary && layer.TokenCount > layer.budgetAllocated {
				extra := min(layer.TokenCount-layer.budgetAllocated,
					b.config.SummaryBudgetMax-layer.budgetAllocated,
					remainingBudget)
				layer.budgetAllocated += extra
				remainingBudget -= extra
			}
		}
	}

	// Third pass: allocate remaining to current session
	for _, layer := range layers {
		if layer.Layer == domain.LayerCurrentSession {
			layer.budgetAllocated = remainingBudget
			break
		}
	}

	// Fourth pass: truncate layers that exceed their budget (lowest priority first)
	// Sort by priority (lowest first) for truncation
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Priority() < layers[j].Priority()
	})

	for _, layer := range layers {
		if layer.TokenCount > layer.budgetAllocated {
			truncated := b.truncateContent(layer.LayerContent, layer.budgetAllocated)
			layer.Content = truncated
			layer.finalTokens = b.tokenizer.CountTokens(truncated)
			layer.truncated = true
			layer.truncationReason = fmt.Sprintf("exceeded budget: %d > %d", layer.TokenCount, layer.budgetAllocated)
		}
	}
}

// truncateContent truncates content to fit within the token budget.
func (b *ContextBuilder) truncateContent(layer *domain.LayerContent, maxTokens int) string {
	if layer.TokenCount <= maxTokens {
		return layer.Content
	}

	// For system instructions, we try to preserve as much as possible
	if layer.Layer == domain.LayerSystem {
		// System instructions are critical - truncate minimally
		return b.truncatePreservingStart(layer.Content, maxTokens)
	}

	// For current session, truncate from the beginning (oldest messages)
	if layer.Layer == domain.LayerCurrentSession {
		return b.truncatePreservingEnd(layer.Content, maxTokens)
	}

	// For other layers, truncate from the end
	return b.truncatePreservingStart(layer.Content, maxTokens)
}

// truncatePreservingStart truncates content while preserving the beginning.
func (b *ContextBuilder) truncatePreservingStart(content string, maxTokens int) string {
	if content == "" {
		return content
	}

	currentTokens := b.tokenizer.CountTokens(content)
	if currentTokens <= maxTokens {
		return content
	}

	// Estimate character position based on token ratio
	ratio := float64(maxTokens) / float64(currentTokens)
	targetChars := int(float64(len(content)) * ratio * 0.9) // 90% for safety margin

	if targetChars >= len(content) {
		return content
	}

	// Find a good breaking point (newline or space)
	truncated := content[:targetChars]
	lastBreak := strings.LastIndexAny(truncated, "\n ")
	if lastBreak > targetChars/2 {
		truncated = content[:lastBreak]
	}

	return truncated + "\n[...truncated...]"
}

// truncatePreservingEnd truncates content while preserving the end (most recent).
func (b *ContextBuilder) truncatePreservingEnd(content string, maxTokens int) string {
	if content == "" {
		return content
	}

	currentTokens := b.tokenizer.CountTokens(content)
	if currentTokens <= maxTokens {
		return content
	}

	// Estimate character position based on token ratio
	ratio := float64(maxTokens) / float64(currentTokens)
	charsToKeep := int(float64(len(content)) * ratio * 0.9)

	if charsToKeep >= len(content) {
		return content
	}

	// Find a good breaking point from the end
	startPos := len(content) - charsToKeep
	firstBreak := strings.IndexAny(content[startPos:], "\n ")
	if firstBreak > 0 && firstBreak < charsToKeep/2 {
		startPos += firstBreak + 1
	}

	return "[...earlier messages truncated...]\n" + content[startPos:]
}

// assemblePrompt assembles the final prompt from all layers.
func (b *ContextBuilder) assemblePrompt(layers []*layerWithStats) string {
	// Sort by priority (highest first) for assembly
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Priority() > layers[j].Priority()
	})

	var parts []string
	for _, layer := range layers {
		if layer.Content != "" {
			parts = append(parts, layer.Content)
		}
	}

	return strings.Join(parts, "\n\n")
}

// buildResult creates the result structure with statistics.
func (b *ContextBuilder) buildResult(layers []*layerWithStats, prompt string, maxTokens int) *domain.ContextBuildResult {
	totalTokens := b.tokenizer.CountTokens(prompt)

	var stats []domain.LayerStats
	var truncationDetails []domain.TruncationDetail
	truncated := false

	for _, layer := range layers {
		percentage := 0.0
		if totalTokens > 0 {
			percentage = float64(layer.finalTokens) / float64(totalTokens) * 100
		}

		stats = append(stats, domain.LayerStats{
			Layer:          layer.Layer,
			OriginalTokens: layer.originalTokens,
			FinalTokens:    layer.finalTokens,
			BudgetUsed:     layer.budgetAllocated,
			Truncated:      layer.truncated,
			Percentage:     percentage,
		})

		if layer.truncated {
			truncated = true
			truncationDetails = append(truncationDetails, domain.TruncationDetail{
				Layer:            layer.Layer,
				TokensRemoved:    layer.originalTokens - layer.finalTokens,
				Reason:           layer.truncationReason,
			})
		}
	}

	return &domain.ContextBuildResult{
		Prompt:            prompt,
		TotalTokens:       totalTokens,
		MaxTokens:         maxTokens,
		LayerStats:        stats,
		Truncated:         truncated,
		TruncationDetails: truncationDetails,
	}
}

// logAssembly logs the assembly details.
func (b *ContextBuilder) logAssembly(result *domain.ContextBuildResult) {
	// Build layer summary
	var layerSummary []string
	for _, stat := range result.LayerStats {
		layerSummary = append(layerSummary,
			fmt.Sprintf("%s: %d/%d tokens (%.1f%%)",
				stat.Layer, stat.FinalTokens, stat.BudgetUsed, stat.Percentage))
	}

	slog.Info("context assembly completed",
		"total_tokens", result.TotalTokens,
		"max_tokens", result.MaxTokens,
		"truncated", result.Truncated,
		"layers", strings.Join(layerSummary, ", "),
	)

	if result.Truncated {
		for _, detail := range result.TruncationDetails {
			slog.Warn("layer truncated",
				"layer", detail.Layer,
				"tokens_removed", detail.TokensRemoved,
				"reason", detail.Reason,
			)
		}
	}
}

// min returns the minimum of multiple integers.
func min(vals ...int) int {
	if len(vals) == 0 {
		return 0
	}
	result := vals[0]
	for _, v := range vals[1:] {
		if v < result {
			result = v
		}
	}
	return result
}
