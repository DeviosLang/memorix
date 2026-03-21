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
	"github.com/devioslang/memorix/server/internal/embed"
	"github.com/devioslang/memorix/server/internal/llm"
	"github.com/devioslang/memorix/server/internal/vectorstore"
)

// ExperienceService manages the Experience Recall Layer.
// It provides:
// - Write interface: Extract experiences from conversation summaries
// - Retrieval interface: Semantic search with user isolation
// - Integration with existing conversation summarizer
type ExperienceService struct {
	store      vectorstore.VectorStore
	embedder   *embed.Embedder
	llm        *llm.Client
	maxPerUser int
}

// NewExperienceService creates a new ExperienceService.
func NewExperienceService(
	store vectorstore.VectorStore,
	embedder *embed.Embedder,
	llmClient *llm.Client,
	maxPerUser int,
) *ExperienceService {
	if maxPerUser <= 0 {
		maxPerUser = 10000 // Default max experiences per user
	}
	return &ExperienceService{
		store:      store,
		embedder:   embedder,
		llm:        llmClient,
		maxPerUser: maxPerUser,
	}
}

// Write stores a new experience with automatic embedding generation.
func (s *ExperienceService) Write(ctx context.Context, req domain.ExperienceWriteRequest) (*domain.Experience, error) {
	// Validate input
	if err := validateWriteRequest(&req); err != nil {
		return nil, err
	}

	// Generate embedding
	embedding, err := s.embedder.Embed(ctx, req.Content)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	now := time.Now()
	exp := &domain.Experience{
		ExperienceID: uuid.New().String(),
		UserID:       req.UserID,
		Content:      req.Content,
		Embedding:    embedding,
		Metadata: domain.ExperienceMetadata{
			SessionID:  req.SessionID,
			Topic:      req.Topic,
			Outcome:    req.Outcome,
			Confidence: req.Confidence,
			Tags:       req.Tags,
			Source:     req.Source,
			Extra:      req.Extra,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Write(ctx, exp); err != nil {
		return nil, fmt.Errorf("store experience: %w", err)
	}

	slog.Info("experience written", "experience_id", exp.ExperienceID, "user_id", exp.UserID, "topic", req.Topic)
	return exp, nil
}

// WriteBatch stores multiple experiences in a single operation.
func (s *ExperienceService) WriteBatch(ctx context.Context, requests []domain.ExperienceWriteRequest) ([]domain.Experience, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	if len(requests) > 100 {
		return nil, &domain.ValidationError{Field: "requests", Message: "too many (max 100)"}
	}

	// Validate all requests first
	for i, req := range requests {
		if err := validateWriteRequest(&req); err != nil {
			return nil, fmt.Errorf("request[%d]: %w", i, err)
		}
	}

	// Batch embed all content
	contents := make([]string, len(requests))
	for i, req := range requests {
		contents[i] = req.Content
	}

	// Generate embeddings for each content
	// Note: If embedder supports batch embedding, use that for efficiency
	experiences := make([]*domain.Experience, 0, len(requests))
	now := time.Now()

	for i, req := range requests {
		embedding, err := s.embedder.Embed(ctx, req.Content)
		if err != nil {
			slog.Warn("failed to embed content in batch, skipping", "index", i, "err", err)
			continue
		}

		experiences = append(experiences, &domain.Experience{
			ExperienceID: uuid.New().String(),
			UserID:       req.UserID,
			Content:      req.Content,
			Embedding:    embedding,
			Metadata: domain.ExperienceMetadata{
				SessionID:  req.SessionID,
				Topic:      req.Topic,
				Outcome:    req.Outcome,
				Confidence: req.Confidence,
				Tags:       req.Tags,
				Source:     req.Source,
				Extra:      req.Extra,
			},
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	// Store all experiences
	count, err := s.store.WriteBatch(ctx, experiences)
	if err != nil {
		return nil, fmt.Errorf("store experiences: %w", err)
	}

	// Return the stored experiences
	result := make([]domain.Experience, count)
	for i := 0; i < count; i++ {
		result[i] = *experiences[i]
	}

	slog.Info("batch experiences written", "count", count, "user_id", requests[0].UserID)
	return result, nil
}

// Search performs semantic search for experiences.
func (s *ExperienceService) Search(ctx context.Context, filter domain.ExperienceFilter) (*domain.ExperienceSearchResult, error) {
	// Validate user_id is provided (required for isolation)
	if filter.UserID == "" {
		return nil, &domain.ValidationError{Field: "user_id", Message: "required for data isolation"}
	}

	// Validate limit
	if filter.Limit <= 0 {
		filter.Limit = domain.DefaultExperienceLimit
	}
	if filter.Limit > domain.MaxExperienceLimit {
		filter.Limit = domain.MaxExperienceLimit
	}

	// Generate query embedding
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}

	queryVec, err := s.embedder.Embed(ctx, filter.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Perform vector search
	result, err := s.store.Search(ctx, filter.UserID, queryVec, filter)
	if err != nil {
		return nil, fmt.Errorf("search experiences: %w", err)
	}

	slog.Info("experience search completed", "user_id", filter.UserID, "query", filter.Query, "results", len(result.Experiences), "latency_ms", result.LatencyMs)
	return result, nil
}

// GetByID retrieves a single experience.
func (s *ExperienceService) GetByID(ctx context.Context, userID, experienceID string) (*domain.Experience, error) {
	if userID == "" {
		return nil, &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if experienceID == "" {
		return nil, &domain.ValidationError{Field: "experience_id", Message: "required"}
	}

	return s.store.GetByID(ctx, userID, experienceID)
}

// Delete removes an experience.
func (s *ExperienceService) Delete(ctx context.Context, userID, experienceID string) error {
	if userID == "" {
		return &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if experienceID == "" {
		return &domain.ValidationError{Field: "experience_id", Message: "required"}
	}

	return s.store.Delete(ctx, userID, experienceID)
}

// DeleteByUser removes all experiences for a user.
func (s *ExperienceService) DeleteByUser(ctx context.Context, userID string) error {
	if userID == "" {
		return &domain.ValidationError{Field: "user_id", Message: "required"}
	}

	return s.store.DeleteByUser(ctx, userID)
}

// Stats returns statistics for a user's experiences.
func (s *ExperienceService) Stats(ctx context.Context, userID string) (*domain.ExperienceStats, error) {
	if userID == "" {
		return nil, &domain.ValidationError{Field: "user_id", Message: "required"}
	}

	return s.store.Stats(ctx, userID)
}

// ExtractFromSummary extracts valuable experiences from a conversation summary.
// This is the main integration point with the ConversationSummarizerService.
// It uses LLM to identify actionable learnings and stores them in the vector store.
func (s *ExperienceService) ExtractFromSummary(ctx context.Context, summary *domain.ConversationSummary) ([]domain.Experience, error) {
	if summary == nil {
		return nil, &domain.ValidationError{Field: "summary", Message: "required"}
	}

	// If no LLM, we cannot extract experiences
	if s.llm == nil {
		slog.Warn("LLM not configured, skipping experience extraction")
		return nil, nil
	}

	// Call LLM to extract experiences from the summary
	extractedExperiences, err := s.extractExperiencesWithLLM(ctx, summary)
	if err != nil {
		return nil, fmt.Errorf("extract experiences: %w", err)
	}

	if len(extractedExperiences) == 0 {
		return nil, nil
	}

	// Store extracted experiences
	result := make([]domain.Experience, 0, len(extractedExperiences))
	for _, exp := range extractedExperiences {
		outcome := domain.ExperienceOutcome(exp.Outcome)
		if !domain.IsValidOutcome(outcome) {
			outcome = domain.OutcomeNeutral
		}
		stored, err := s.Write(ctx, domain.ExperienceWriteRequest{
			UserID:     summary.UserID,
			Content:    exp.Content,
			SessionID:  summary.SessionID,
			Topic:      exp.Topic,
			Outcome:    outcome,
			Confidence: exp.Confidence,
			Tags:       exp.Tags,
			Source:     "llm_extraction",
		})
		if err != nil {
			slog.Warn("failed to store extracted experience", "err", err, "content", truncateString(exp.Content, 50))
			continue
		}
		result = append(result, *stored)
	}

	slog.Info("experiences extracted from summary", "summary_id", summary.SummaryID, "count", len(result))
	return result, nil
}

// extractedExperience is an LLM-extracted experience.
type extractedExperience struct {
	Content    string   `json:"content"`
	Topic      string   `json:"topic"`
	Outcome    string   `json:"outcome"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags"`
}

// extractExperiencesWithLLM calls LLM to extract experiences from a summary.
func (s *ExperienceService) extractExperiencesWithLLM(ctx context.Context, summary *domain.ConversationSummary) ([]extractedExperience, error) {
	systemPrompt := `You are an experience extraction engine. Your task is to identify valuable, reusable experiences from conversation summaries that would help in future similar situations.

## What to Extract

Focus on experiences that have lasting value:
1. **Problem-solving patterns** - Approaches that worked or didn't work
2. **Decision rationale** - Why certain choices were made
3. **Learned preferences** - User preferences discovered during the conversation
4. **Technical insights** - Specific technical details worth remembering
5. **Failure lessons** - Things that didn't work and why

## What NOT to Extract

1. **Ephemeral information** - One-time facts with no future relevance
2. **General knowledge** - Information that's universally known
3. **Small talk** - Greetings, acknowledgments, filler
4. **Context-specific details** - Details that only matter in that specific context

## Output Format

Return a JSON array of experiences. Each experience should be:
- A single, self-contained statement
- Actionable or informative for future use
- In the user's language
- Concise but complete

Example:
{
  "experiences": [
    {
      "content": "When debugging Go race conditions, use -race flag with multiple goroutine spawns to increase detection probability",
      "topic": "debugging",
      "outcome": "success",
      "confidence": 0.9,
      "tags": ["go", "debugging", "concurrency"]
    },
    {
      "content": "User prefers code examples over long explanations for technical topics",
      "topic": "preferences",
      "outcome": "learning",
      "confidence": 0.85,
      "tags": ["preferences", "communication-style"]
    }
  ]
}

## Outcome Types

- success: The approach worked well
- failure: The approach didn't work (valuable lesson)
- learning: General learning without clear success/failure
- neutral: Outcome not determined

Return ONLY valid JSON. No markdown fences, no explanation.`

	userPrompt := fmt.Sprintf(`Extract valuable experiences from this conversation summary.

Title: %s
Summary: %s
Key Topics: %v
User Intent: %s

Identify experiences that would be valuable for future conversations with this user.`, 
		summary.Title, summary.Summary, summary.KeyTopics, summary.UserIntent)

	raw, err := s.llm.CompleteJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("extraction LLM call: %w", err)
	}

	type extractionResult struct {
		Experiences []extractedExperience `json:"experiences"`
	}

	parsed, err := llm.ParseJSON[extractionResult](raw)
	if err != nil {
		// Retry once without JSON format requirement
		raw2, retryErr := s.llm.CompleteJSON(ctx, systemPrompt,
			"Your previous response was not valid JSON. Return ONLY the JSON object.\n\n"+userPrompt)
		if retryErr != nil {
			return nil, fmt.Errorf("extraction retry: %w", retryErr)
		}
		parsed, err = llm.ParseJSON[extractionResult](raw2)
		if err != nil {
			return nil, fmt.Errorf("extraction parse after retry: %w", err)
		}
	}

	// Validate and clean up
	var result []extractedExperience
	for _, exp := range parsed.Experiences {
		if exp.Content == "" {
			continue
		}
		// Truncate content if too long
		if utf8.RuneCountInString(exp.Content) > domain.MaxExperienceContent {
			exp.Content = string([]rune(exp.Content)[:domain.MaxExperienceContent])
		}
		// Validate outcome
		if !domain.IsValidOutcome(domain.ExperienceOutcome(exp.Outcome)) {
			exp.Outcome = string(domain.OutcomeNeutral)
		}
		// Validate confidence
		if exp.Confidence < 0 {
			exp.Confidence = 0
		}
		if exp.Confidence > 1 {
			exp.Confidence = 1
		}
		// Limit tags
		if len(exp.Tags) > domain.MaxExperienceTags {
			exp.Tags = exp.Tags[:domain.MaxExperienceTags]
		}
		// Clean up tags
		for i, tag := range exp.Tags {
			exp.Tags[i] = strings.TrimSpace(strings.ToLower(tag))
		}

		result = append(result, exp)
	}

	return result, nil
}

// validateWriteRequest validates an ExperienceWriteRequest.
func validateWriteRequest(req *domain.ExperienceWriteRequest) error {
	if req.UserID == "" {
		return &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if req.Content == "" {
		return &domain.ValidationError{Field: "content", Message: "required"}
	}
	if utf8.RuneCountInString(req.Content) > domain.MaxExperienceContent {
		return &domain.ValidationError{Field: "content", Message: "too long (max 10000 characters)"}
	}
	if req.Outcome != "" && !domain.IsValidOutcome(req.Outcome) {
		return &domain.ValidationError{Field: "outcome", Message: "invalid outcome value"}
	}
	if req.Confidence < domain.MinExperienceConfidence || req.Confidence > domain.MaxExperienceConfidence {
		req.Confidence = 1.0 // Default to full confidence
	}
	if len(req.Tags) > domain.MaxExperienceTags {
		return &domain.ValidationError{Field: "tags", Message: "too many (max 20)"}
	}
	return nil
}

// truncateString truncates a string to maxLen runes.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// HasEmbedder returns true if an embedder is configured.
func (s *ExperienceService) HasEmbedder() bool {
	return s.embedder != nil
}

// HasLLM returns true if an LLM client is configured.
func (s *ExperienceService) HasLLM() bool {
	return s.llm != nil
}

// Health checks if the experience service is healthy.
func (s *ExperienceService) Health(ctx context.Context) error {
	return s.store.Health(ctx)
}

// Close releases resources used by the experience service.
func (s *ExperienceService) Close() error {
	return s.store.Close()
}

// ExperienceIntegration provides integration with ConversationSummarizerService.
// It can be called after a summary is generated to automatically extract experiences.
type ExperienceIntegration struct {
	expService *ExperienceService
}

// NewExperienceIntegration creates a new ExperienceIntegration.
func NewExperienceIntegration(expService *ExperienceService) *ExperienceIntegration {
	return &ExperienceIntegration{expService: expService}
}

// OnSummaryCreated is called after a conversation summary is created.
// It extracts experiences from the summary and stores them.
func (i *ExperienceIntegration) OnSummaryCreated(ctx context.Context, summary *domain.ConversationSummary) error {
	if i.expService == nil {
		return nil
	}

	_, err := i.expService.ExtractFromSummary(ctx, summary)
	if err != nil {
		slog.Warn("failed to extract experiences from summary", "summary_id", summary.SummaryID, "err", err)
		// Don't fail the summary creation if experience extraction fails
		return nil
	}

	return nil
}
