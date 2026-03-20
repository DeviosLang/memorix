package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/llm"
	"github.com/devioslang/memorix/server/internal/repository"
)

// ConversationSummarizerService generates and manages conversation summaries.
// Implements the "fourth layer" of ChatGPT's memory system - recent conversation summaries.
// Key features:
// - Pre-computed summaries for zero retrieval latency
// - Sliding window of 15-20 summaries per user
// - LLM-generated title, summary, key topics, and user intent
type ConversationSummarizerService struct {
	summaries    repository.ConversationSummaryRepo
	llm          LLMClient
	maxSummaries int
}

// NewConversationSummarizerService creates a new ConversationSummarizerService.
func NewConversationSummarizerService(
	summaries repository.ConversationSummaryRepo,
	llmClient LLMClient,
	maxSummaries int,
) *ConversationSummarizerService {
	if maxSummaries <= 0 {
		maxSummaries = domain.DefaultMaxSummariesPerUser
	}
	return &ConversationSummarizerService{
		summaries:    summaries,
		llm:          llmClient,
		maxSummaries: maxSummaries,
	}
}

// SummarizeRequest contains the input for generating a conversation summary.
type SummarizeRequest struct {
	UserID    string                   `json:"user_id"`
	SessionID string                   `json:"session_id,omitempty"`
	Messages  []domain.SummaryMessage `json:"messages"`
}

// SummarizeResult contains the result of the summarization process.
type SummarizeResult struct {
	Status     string                        `json:"status"` // complete | failed
	SummaryID  string                        `json:"summary_id,omitempty"`
	Summary    *domain.ConversationSummary  `json:"summary,omitempty"`
	Error      string                        `json:"error,omitempty"`
	Skipped    bool                          `json:"skipped,omitempty"` // true if summary already exists
}

// Summarize generates a summary for a completed conversation.
// It:
// 1. Checks if a summary already exists for the session (skips if so)
// 2. Calls LLM to generate structured summary
// 3. Enforces sliding window by deleting oldest summaries if needed
// 4. Stores the new summary
func (s *ConversationSummarizerService) Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResult, error) {
	slog.Info("summarizer started", "user_id", req.UserID, "session_id", req.SessionID, "messages", len(req.Messages))

	// Validate input
	if req.UserID == "" {
		return nil, &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if len(req.Messages) == 0 {
		return nil, &domain.ValidationError{Field: "messages", Message: "required"}
	}

	// Check if summary already exists for this session
	if req.SessionID != "" {
		existing, err := s.summaries.GetBySessionID(ctx, req.SessionID)
		if err != nil && err != domain.ErrNotFound {
			slog.Warn("failed to check existing summary", "err", err, "session_id", req.SessionID)
			// Continue anyway
		}
		if existing != nil {
			slog.Info("summary already exists for session", "session_id", req.SessionID, "summary_id", existing.SummaryID)
			return &SummarizeResult{
				Status:    "complete",
				SummaryID: existing.SummaryID,
				Summary:   existing,
				Skipped:   true,
			}, nil
		}
	}

	// If no LLM client, we cannot generate summaries
	if s.llm == nil {
		return &SummarizeResult{
			Status: "failed",
			Error:  "LLM client not configured",
		}, nil
	}

	// Format conversation for LLM
	conversation := formatConversationForSummary(req.Messages)
	if conversation == "" {
		return &SummarizeResult{
			Status: "complete", // Empty conversation, nothing to summarize
		}, nil
	}

	// Truncate if too long (avoid LLM token limits)
	const maxConversationRunes = 50000
	if utf8.RuneCountInString(conversation) > maxConversationRunes {
		runes := []rune(conversation)
		conversation = string(runes[:maxConversationRunes]) + "\n...[truncated]"
	}

	// Call LLM to generate summary
	summaryData, err := s.generateSummary(ctx, conversation)
	if err != nil {
		slog.Error("failed to generate summary", "err", err, "user_id", req.UserID)
		return &SummarizeResult{
			Status: "failed",
			Error:  err.Error(),
		}, nil
	}

	// Check capacity and enforce sliding window
	count, err := s.summaries.CountByUserID(ctx, req.UserID)
	if err != nil {
		slog.Warn("failed to count summaries", "err", err, "user_id", req.UserID)
		count = 0 // Continue anyway
	}

	// If at capacity, delete oldest to maintain sliding window
	if count >= s.maxSummaries {
		toDelete := count - s.maxSummaries + 1
		if toDelete > 0 {
			deleted, err := s.summaries.DeleteOldest(ctx, req.UserID, toDelete)
			if err != nil {
				slog.Warn("failed to delete oldest summaries", "err", err, "user_id", req.UserID, "count", toDelete)
			} else {
				slog.Info("deleted oldest summaries for sliding window", "user_id", req.UserID, "deleted", deleted)
			}
		}
	}

	// Create and store the summary
	summary := &domain.ConversationSummary{
		SummaryID:  uuid.New().String(),
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		Title:      summaryData.Title,
		Summary:    summaryData.Summary,
		KeyTopics:  summaryData.KeyTopics,
		UserIntent: summaryData.UserIntent,
		CreatedAt:  time.Now(),
	}

	if err := s.summaries.Create(ctx, summary); err != nil {
		return nil, fmt.Errorf("store summary: %w", err)
	}

	slog.Info("summary created", "summary_id", summary.SummaryID, "user_id", req.UserID, "title", summary.Title)

	return &SummarizeResult{
		Status:    "complete",
		SummaryID: summary.SummaryID,
		Summary:   summary,
	}, nil
}

// generateSummary calls the LLM to generate a structured conversation summary.
func (s *ConversationSummarizerService) generateSummary(ctx context.Context, conversation string) (*domain.SummaryGenerationResult, error) {
	currentDate := time.Now().Format("2006-01-02")

	systemPrompt := `You are a conversation summarization engine. Your task is to analyze a conversation and generate a structured summary with title, summary text, key topics, and user intent.

## Requirements

1. **title**: A brief, descriptive title for the conversation (max 100 characters)
   - Should capture the main topic or purpose
   - Use the user's primary language
   - Example: "Go concurrency patterns discussion" or "Python 数据处理方案"

2. **summary**: A concise summary of the conversation (max 200 Chinese characters or ~100 English words)
   - Focus on what was discussed, not the mechanics of the conversation
   - Include key decisions, solutions, or outcomes
   - Preserve the user's language
   - Omit greetings, filler, and meta-commentary

3. **key_topics**: An array of 2-5 topic tags that categorize this conversation
   - Use lowercase English snake_case
   - Examples: ["go_programming", "concurrency", "best_practices"]
   - Keep topics general enough for cross-conversation relevance

4. **user_intent**: What the user wanted to achieve from this conversation (max 100 characters)
   - Focus on the user's goal, not what happened
   - Examples: "Learn Go concurrency patterns" or "Fix Python data pipeline bug"

## Examples

Input:
User: Can you help me understand Go channels?
Assistant: Of course! Go channels are a powerful concurrency primitive. They allow goroutines to communicate and synchronize...
User: What's the difference between buffered and unbuffered channels?
Assistant: Unbuffered channels block until both sender and receiver are ready. Buffered channels have a capacity...
User: When should I use which one?
Assistant: Use unbuffered for synchronization, buffered when you need to decouple sender/receiver rates...

Output:
{
  "title": "Go channels deep dive",
  "summary": "Explored Go channels including buffered vs unbuffered differences, use cases, and synchronization patterns. Covered when to use each type.",
  "key_topics": ["go", "concurrency", "channels", "goroutines"],
  "user_intent": "Understand Go channel types and usage"
}

Input:
User: 我的 Python 脚本处理大数据集时内存不足，有什么解决方案？
Assistant: 有几个方案：1. 使用生成器逐行处理；2. 使用 pandas 的 chunk 读取；3. 使用 Dask 进行分布式处理...
User: 能详细说说 pandas chunk 怎么用吗？
Assistant: 可以使用 pd.read_csv 的 chunksize 参数...

Output:
{
  "title": "Python 大数据处理内存优化",
  "summary": "讨论了处理大数据集时的内存问题解决方案，重点介绍了 pandas 的分块读取方法。用户需要处理大型 CSV 文件。",
  "key_topics": ["python", "pandas", "memory_optimization", "big_data"],
  "user_intent": "解决大数据处理的内存溢出问题"
}

## Output Format

Return ONLY valid JSON. No markdown fences, no explanation.

{
  "title": "...",
  "summary": "...",
  "key_topics": ["...", "..."],
  "user_intent": "..."
}`

	userPrompt := fmt.Sprintf("Generate a structured summary for this conversation. Today's date is %s.\n\n%s", currentDate, conversation)

	raw, err := s.llm.CompleteJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("summary LLM call: %w", err)
	}

	parsed, err := llm.ParseJSON[domain.SummaryGenerationResult](raw)
	if err != nil {
		// Retry once without JSON format requirement
		raw2, retryErr := s.llm.CompleteJSON(ctx, systemPrompt,
			"Your previous response was not valid JSON. Return ONLY the JSON object.\n\n"+userPrompt)
		if retryErr != nil {
			return nil, fmt.Errorf("summary retry: %w", retryErr)
		}
		parsed, err = llm.ParseJSON[domain.SummaryGenerationResult](raw2)
		if err != nil {
			return nil, fmt.Errorf("summary parse after retry: %w", err)
		}
	}

	// Validate and truncate fields
	if parsed.Title == "" {
		parsed.Title = "Untitled conversation"
	}
	if utf8.RuneCountInString(parsed.Title) > domain.MaxTitleLength {
		parsed.Title = string([]rune(parsed.Title)[:domain.MaxTitleLength])
	}
	if utf8.RuneCountInString(parsed.Summary) > domain.MaxSummaryLength {
		parsed.Summary = string([]rune(parsed.Summary)[:domain.MaxSummaryLength])
	}
	if utf8.RuneCountInString(parsed.UserIntent) > domain.MaxUserIntentLength {
		parsed.UserIntent = string([]rune(parsed.UserIntent)[:domain.MaxUserIntentLength])
	}
	if len(parsed.KeyTopics) > domain.MaxTopicsCount {
		parsed.KeyTopics = parsed.KeyTopics[:domain.MaxTopicsCount]
	}

	// Clean up topics
	for i, topic := range parsed.KeyTopics {
		topic = strings.TrimSpace(strings.ToLower(topic))
		topic = strings.ReplaceAll(topic, " ", "_")
		parsed.KeyTopics[i] = topic
	}

	return &parsed, nil
}

// GetSummary retrieves a single summary by ID.
func (s *ConversationSummarizerService) GetSummary(ctx context.Context, summaryID string) (*domain.ConversationSummary, error) {
	return s.summaries.GetByID(ctx, summaryID)
}

// GetSummariesByUser retrieves all summaries for a user.
func (s *ConversationSummarizerService) GetSummariesByUser(ctx context.Context, userID string, limit int) ([]domain.ConversationSummary, error) {
	if limit <= 0 {
		limit = s.maxSummaries
	}
	return s.summaries.GetByUserID(ctx, userID, limit)
}

// ListSummaries retrieves summaries with filtering and pagination.
func (s *ConversationSummarizerService) ListSummaries(ctx context.Context, filter domain.ConversationSummaryFilter) ([]domain.ConversationSummary, int, error) {
	if filter.UserID == "" {
		return nil, 0, &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	return s.summaries.List(ctx, filter)
}

// DeleteSummary deletes a summary by ID.
func (s *ConversationSummarizerService) DeleteSummary(ctx context.Context, summaryID string) error {
	return s.summaries.Delete(ctx, summaryID)
}

// CountSummariesByUser returns the count of summaries for a user.
func (s *ConversationSummarizerService) CountSummariesByUser(ctx context.Context, userID string) (int, error) {
	return s.summaries.CountByUserID(ctx, userID)
}

// HasLLM returns true if an LLM client is configured.
func (s *ConversationSummarizerService) HasLLM() bool {
	return s.llm != nil
}

// formatConversationForSummary formats messages into a conversation string for summarization.
func formatConversationForSummary(messages []domain.SummaryMessage) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := msg.Role
		if r, _ := utf8.DecodeRuneInString(role); r != utf8.RuneError {
			role = strings.ToUpper(string(r)) + role[utf8.RuneLen(r):]
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String())
}
