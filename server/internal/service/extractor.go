package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/llm"
	"github.com/devioslang/memorix/server/internal/repository"
)

// LLMClient is an interface for LLM completion services.
// The llm.Client struct satisfies this interface.
type LLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
	CompleteJSON(ctx context.Context, system, user string) (string, error)
}

// ExtractorService extracts structured facts from conversation content.
// It implements the LLM-driven Memory Extractor described in Issue #5.
type ExtractorService struct {
	facts     repository.UserProfileFactRepo
	llm       LLMClient
	userProf  *UserProfileService
}

// NewExtractorService creates a new ExtractorService.
func NewExtractorService(
	facts repository.UserProfileFactRepo,
	llmClient LLMClient,
	userProf *UserProfileService,
) *ExtractorService {
	return &ExtractorService{
		facts:    facts,
		llm:      llmClient,
		userProf: userProf,
	}
}

// ExtractedFact represents a single extracted fact from conversation.
type ExtractedFact struct {
	Category   domain.FactCategory `json:"category"`
	Key        string              `json:"key"`
	Value      string              `json:"value"`
	Confidence float64             `json:"confidence"`
	Source     domain.FactSource   `json:"source"` // explicit or inferred
}

// ExtractRequest contains the input for the extraction pipeline.
type ExtractRequest struct {
	Messages  []IngestMessage `json:"messages"`
	UserID    string          `json:"user_id"`
	SessionID string          `json:"session_id,omitempty"`
}

// ExtractResult contains the result of the extraction pipeline.
type ExtractResult struct {
	Status       string          `json:"status"` // complete | partial | failed
	FactsAdded   int             `json:"facts_added"`
	FactsUpdated int             `json:"facts_updated"`
	FactsSkipped int             `json:"facts_skipped"`
	FactIDs      []string        `json:"fact_ids,omitempty"`
	Warnings     int             `json:"warnings,omitempty"`
	Error        string          `json:"error,omitempty"`
}

// Extract runs the full extraction pipeline:
// 1. Detect explicit triggers in user messages
// 2. Run LLM-based extraction for remaining content
// 3. Deduplicate against existing facts
// 4. Write new/updated facts to the User Profile Store
func (s *ExtractorService) Extract(ctx context.Context, req ExtractRequest) (*ExtractResult, error) {
	slog.Info("extractor pipeline started", "user_id", req.UserID, "messages", len(req.Messages))

	if req.UserID == "" {
		return nil, &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if len(req.Messages) == 0 {
		return nil, &domain.ValidationError{Field: "messages", Message: "required"}
	}

	// If no LLM, we can only do explicit trigger detection
	if s.llm == nil {
		return s.extractExplicitOnly(ctx, req)
	}

	// Strip previously injected memory context from messages
	cleaned := stripInjectedContext(req.Messages)

	// Format conversation for LLM
	conversation := formatConversation(cleaned)
	if conversation == "" {
		return &ExtractResult{Status: "complete"}, nil
	}

	// Phase 1: Detect explicit triggers and extract facts from them
	explicitFacts := s.detectExplicitTriggers(cleaned)
	slog.Info("explicit triggers detected", "count", len(explicitFacts))

	// Phase 2: Run LLM-based extraction for implicit facts
	implicitFacts, err := s.extractImplicitFacts(ctx, conversation)
	if err != nil {
		slog.Error("implicit extraction failed", "err", err)
		// Continue with explicit facts only
		if len(explicitFacts) == 0 {
			return &ExtractResult{Status: "failed", Error: err.Error(), Warnings: 1}, nil
		}
	}
	slog.Info("implicit facts extracted", "count", len(implicitFacts))

	// Merge facts (explicit facts take precedence for same key)
	allFacts := s.mergeFacts(explicitFacts, implicitFacts)

	// Phase 3: Deduplicate and write facts
	result, err := s.dedupAndWrite(ctx, req.UserID, allFacts)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// extractExplicitOnly handles extraction when LLM is not available.
func (s *ExtractorService) extractExplicitOnly(ctx context.Context, req ExtractRequest) (*ExtractResult, error) {
	cleaned := stripInjectedContext(req.Messages)
	explicitFacts := s.detectExplicitTriggers(cleaned)

	if len(explicitFacts) == 0 {
		return &ExtractResult{Status: "complete"}, nil
	}

	return s.dedupAndWrite(ctx, req.UserID, explicitFacts)
}

// ExplicitTriggerPattern represents a pattern for detecting explicit memory commands.
type ExplicitTriggerPattern struct {
	Pattern     *regexp.Regexp
	Category    domain.FactCategory
	KeyExtractor func(matches []string) string
}

var explicitTriggerPatterns = []ExplicitTriggerPattern{
	// Chinese patterns - ORDER MATTERS: more specific patterns first
	// "记住我喜欢xxx" or "记住我爱xxx" - MUST be before generic "记住" patterns
	{
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我)?(喜欢|爱|偏好)\s*(.+)$`),
		Category: domain.CategoryPreference,
		KeyExtractor: func(m []string) string {
			return "likes"
		},
	},
	{
		// "记住我不喜欢xxx" or "记住我讨厌xxx"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我)?(不喜欢|讨厌|厌恶)\s*(.+)$`),
		Category: domain.CategoryPreference,
		KeyExtractor: func(m []string) string {
			return "dislikes"
		},
	},
	{
		// "记住我叫张三" or "记住我的名字是张三"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我|我的)?(?:名字|叫)?\s*(?:是)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "name"
		},
	},
	{
		// "记住我的邮箱是xxx" or "记住我的email是xxx"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我|我的)?(?:邮箱|email|e-mail)\s*(?:是)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "email"
		},
	},
	{
		// "记住我的电话是xxx" or "记住我的手机号是xxx"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我|我的)?(?:电话|手机|手机号)\s*(?:是)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "phone"
		},
	},
	{
		// "记住我是xxx工程师" or "记住我的职业是xxx"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我|我的)?(?:职业|工作|职位)\s*(?:是)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "occupation"
		},
	},
	{
		// "记住我的目标是xxx" or "记住我想xxx"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我|我的)?(?:目标|想|想要)\s*(?:是)?\s*(.+)$`),
		Category: domain.CategoryGoal,
		KeyExtractor: func(m []string) string {
			return "goal"
		},
	},
	{
		// "记住我会xxx" or "记住我擅长xxx" or "记住我的技能是xxx"
		Pattern:  regexp.MustCompile(`(?i)^记住\s*(?:我|我的)?(?:会|擅长|技能)\s*(.+)$`),
		Category: domain.CategorySkill,
		KeyExtractor: func(m []string) string {
			return "skill"
		},
	},
	// Generic "记住xxx" pattern (catch-all for other explicit commands)
	{
		Pattern:  regexp.MustCompile(`(?i)^记住\s+(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "fact"
		},
	},
	// English patterns
	{
		// "remember my name is xxx"
		Pattern:  regexp.MustCompile(`(?i)^remember\s+(?:my\s+)?name\s+(?:is\s+)?(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "name"
		},
	},
	{
		// "remember my email is xxx"
		Pattern:  regexp.MustCompile(`(?i)^remember\s+(?:my\s+)?(?:email|e-mail)\s+(?:is\s+)?(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "email"
		},
	},
	{
		// "remember I like xxx" or "remember I love xxx"
		Pattern:  regexp.MustCompile(`(?i)^remember\s+(?:I\s+)?(like|love|prefer)\s+(.+)$`),
		Category: domain.CategoryPreference,
		KeyExtractor: func(m []string) string {
			return "likes"
		},
	},
	{
		// "remember I dislike xxx" or "remember I hate xxx"
		Pattern:  regexp.MustCompile(`(?i)^remember\s+(?:I\s+)?(dislike|hate)\s+(.+)$`),
		Category: domain.CategoryPreference,
		KeyExtractor: func(m []string) string {
			return "dislikes"
		},
	},
	// "我的xxx是xxx" patterns (Chinese "my xxx is xxx")
	{
		Pattern:  regexp.MustCompile(`^(?:我|我的)(?:名字|姓名)\s*(?:是|=)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "name"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我|我的)(?:邮箱|email|e-mail)\s*(?:是|=)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "email"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我|我的)(?:职业|工作|职位)\s*(?:是|=)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "occupation"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我|我的)(?:公司|单位)\s*(?:是|=)?\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "company"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我|我的)(?:目标)\s*(?:是|=)?\s*(.+)$`),
		Category: domain.CategoryGoal,
		KeyExtractor: func(m []string) string {
			return "goal"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我)(?:会|擅长|精通)\s*(.+)$`),
		Category: domain.CategorySkill,
		KeyExtractor: func(m []string) string {
			return "skill"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我)(?:喜欢|爱)\s*(.+)$`),
		Category: domain.CategoryPreference,
		KeyExtractor: func(m []string) string {
			return "likes"
		},
	},
	{
		Pattern:  regexp.MustCompile(`^(?:我)(?:不喜欢|讨厌)\s*(.+)$`),
		Category: domain.CategoryPreference,
		KeyExtractor: func(m []string) string {
			return "dislikes"
		},
	},
	// "我是xxx" pattern for occupation/role
	{
		Pattern:  regexp.MustCompile(`^我是\s*(.+)$`),
		Category: domain.CategoryPersonal,
		KeyExtractor: func(m []string) string {
			return "occupation"
		},
	},
}

// detectExplicitTriggers scans user messages for explicit memory commands.
func (s *ExtractorService) detectExplicitTriggers(messages []IngestMessage) []ExtractedFact {
	var facts []ExtractedFact

	for _, msg := range messages {
		// Only process user messages
		if strings.ToLower(msg.Role) != "user" {
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		for _, pattern := range explicitTriggerPatterns {
			matches := pattern.Pattern.FindStringSubmatch(content)
			if matches == nil {
				continue
			}

			// Extract the value from matches
			var value string
			if len(matches) >= 2 {
				// For patterns with 2 capture groups (e.g., "remember I like xxx")
				if len(matches) >= 3 && matches[2] != "" {
					value = strings.TrimSpace(matches[2])
				} else {
					value = strings.TrimSpace(matches[len(matches)-1])
				}
			}

			if value == "" {
				continue
			}

			key := pattern.KeyExtractor(matches)

			facts = append(facts, ExtractedFact{
				Category:   pattern.Category,
				Key:        key,
				Value:      value,
				Confidence: 1.0, // Explicit facts have full confidence
				Source:     domain.SourceExplicit,
			})

			// Only match first pattern per message to avoid duplicates
			break
		}
	}

	return facts
}

// extractImplicitFacts uses LLM to extract structured facts from conversation.
func (s *ExtractorService) extractImplicitFacts(ctx context.Context, conversation string) ([]ExtractedFact, error) {
	currentDate := time.Now().Format("2006-01-02")

	systemPrompt := `You are an information extraction engine. Your task is to extract structured facts about the user from a conversation and return them as a JSON array.

## Categories

Use one of these categories for each fact:
- "personal": Personal information (name, age, location, occupation, contact info, family, etc.)
- "preference": User preferences (likes, dislikes, favorites, preferred styles, etc.)
- "goal": User goals and objectives (what they want to achieve, learn, or accomplish)
- "skill": User skills and expertise (what they know, can do, or are learning)

## Rules

1. Extract facts ONLY from the user's messages. Ignore assistant and system messages entirely.
2. Each fact must have: category, key (a short identifier like "name", "language_preference"), value (the actual fact), and confidence (0.0-1.0).
3. Keys should be in English snake_case format.
4. Preserve the user's original language in the value field.
5. Confidence should be:
   - 1.0 for explicit statements (e.g., "My name is John")
   - 0.8-0.9 for strong implicit signals (e.g., "I've been working with Python for 10 years" → skill)
   - 0.5-0.7 for weaker inferences (e.g., "I hate debugging" → preference)
6. Omit ephemeral information, greetings, task-specific details with no future value.
7. If no meaningful facts exist, return an empty array.

## Examples

Input:
User: Hi, I'm Sarah, a product manager at a tech startup.
Assistant: Hello Sarah! How can I help you today?
Output: {"facts": [{"category": "personal", "key": "name", "value": "Sarah", "confidence": 1.0}, {"category": "personal", "key": "occupation", "value": "product manager at a tech startup", "confidence": 1.0}]}

Input:
User: I'm trying to learn Rust, but it's been challenging. I've been a Python developer for 8 years.
Assistant: Rust has a steep learning curve. Your Python background will help with some concepts.
Output: {"facts": [{"category": "skill", "key": "learning", "value": "Rust", "confidence": 1.0}, {"category": "skill", "key": "programming_language", "value": "Python (8 years experience)", "confidence": 1.0}]}

Input:
User: I prefer dark mode for everything, especially in my IDE.
Assistant: Dark mode is easier on the eyes!
Output: {"facts": [{"category": "preference", "key": "ui_theme", "value": "dark mode", "confidence": 1.0}]}

Input:
User: 我是后端工程师，主要用 Python 和 Go。
Assistant: 您好！有什么可以帮助您的？
Output: {"facts": [{"category": "personal", "key": "occupation", "value": "后端工程师", "confidence": 1.0}, {"category": "skill", "key": "programming_language", "value": "Python 和 Go", "confidence": 1.0}]}

## Output Format

Return ONLY valid JSON. No markdown fences, no explanation.

{"facts": [{"category": "...", "key": "...", "value": "...", "confidence": 0.0}, ...]}`

	userPrompt := fmt.Sprintf("Extract structured facts about the user from this conversation. Today's date is %s.\n\n%s", currentDate, conversation)

	raw, err := s.llm.CompleteJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("extraction LLM call: %w", err)
	}

	type llmExtractedFact struct {
		Category   string  `json:"category"`
		Key        string  `json:"key"`
		Value      string  `json:"value"`
		Confidence float64 `json:"confidence"`
	}
	type extractResponse struct {
		Facts []llmExtractedFact `json:"facts"`
	}

	parsed, err := llm.ParseJSON[extractResponse](raw)
	if err != nil {
		// Retry once without JSON format requirement
		raw2, retryErr := s.llm.CompleteJSON(ctx, systemPrompt,
			"Your previous response was not valid JSON. Return ONLY the JSON object.\n\n"+userPrompt)
		if retryErr != nil {
			return nil, fmt.Errorf("extraction retry: %w", retryErr)
		}
		parsed, err = llm.ParseJSON[extractResponse](raw2)
		if err != nil {
			return nil, fmt.Errorf("extraction parse after retry: %w", err)
		}
	}

	// Convert to ExtractedFact and validate
	var facts []ExtractedFact
	for _, f := range parsed.Facts {
		// Validate and normalize category
		cat := domain.FactCategory(strings.ToLower(f.Category))
		if !domain.IsValidCategory(cat) {
			cat = domain.CategoryPersonal // Default to personal
		}

		// Validate confidence
		conf := f.Confidence
		if conf < 0 {
			conf = 0
		}
		if conf > 1 {
			conf = 1
		}

		// Clean up key and value
		key := strings.TrimSpace(strings.ToLower(f.Key))
		value := strings.TrimSpace(f.Value)

		if key == "" || value == "" {
			continue
		}

		facts = append(facts, ExtractedFact{
			Category:   cat,
			Key:        key,
			Value:      value,
			Confidence: conf,
			Source:     domain.SourceInferred,
		})
	}

	slog.Info("implicit facts extracted", "count", len(facts))
	return facts, nil
}

// mergeFacts combines explicit and implicit facts, with explicit taking precedence.
func (s *ExtractorService) mergeFacts(explicit, implicit []ExtractedFact) []ExtractedFact {
	// Build a map of explicit facts by (category, key)
	explicitMap := make(map[string]ExtractedFact)
	for _, f := range explicit {
		key := fmt.Sprintf("%s:%s", f.Category, f.Key)
		explicitMap[key] = f
	}

	// Add implicit facts that don't conflict with explicit ones
	var result []ExtractedFact
	for _, f := range explicit {
		result = append(result, f)
	}
	for _, f := range implicit {
		key := fmt.Sprintf("%s:%s", f.Category, f.Key)
		if _, exists := explicitMap[key]; !exists {
			result = append(result, f)
		}
	}

	return result
}

// dedupAndWrite checks for duplicates and writes facts to the User Profile Store.
// Deduplication rules:
// 1. Exact match on (user_id, category, key) → update existing fact
// 2. Similar value (text similarity) → skip to avoid duplicates
func (s *ExtractorService) dedupAndWrite(ctx context.Context, userID string, facts []ExtractedFact) (*ExtractResult, error) {
	result := &ExtractResult{Status: "complete"}
	var factIDs []string
	var warnings int

	for _, fact := range facts {
		// Check for exact match on (category, key)
		existing, err := s.facts.GetByKey(ctx, userID, fact.Category, fact.Key)
		if err != nil && err != domain.ErrNotFound {
			slog.Warn("failed to check existing fact", "err", err, "key", fact.Key)
			warnings++
			continue
		}

		if existing != nil {
			// Exact match found - update if value is different or confidence is higher
			if existing.Value == fact.Value {
				// Same value - just update timestamp
				result.FactsSkipped++
				slog.Debug("skipping duplicate fact", "key", fact.Key, "value", fact.Value)
				continue
			}

			// Check for similar values (basic dedup)
			if s.isSimilarValue(existing.Value, fact.Value) {
				result.FactsSkipped++
				slog.Debug("skipping similar fact", "key", fact.Key, "existing", existing.Value, "new", fact.Value)
				continue
			}

			// Update the existing fact
			updateInput := UpdateFactInput{
				Value:      &fact.Value,
				Confidence: &fact.Confidence,
				Source:     &fact.Source,
			}
			updated, err := s.userProf.UpdateFact(ctx, existing.FactID, updateInput)
			if err != nil {
				slog.Warn("failed to update fact", "err", err, "fact_id", existing.FactID)
				warnings++
				continue
			}
			result.FactsUpdated++
			factIDs = append(factIDs, updated.FactID)
			slog.Info("updated fact", "key", fact.Key, "value", fact.Value)
			continue
		}

		// Check for similar values in same category
		similar, err := s.facts.SearchByValue(ctx, userID, fact.Value, 5)
		shouldSkip := false
		if err != nil {
			slog.Warn("failed to search similar facts", "err", err)
			// Continue anyway
		} else if len(similar) > 0 {
			// Check similarity threshold
			for _, sim := range similar {
				if sim.Category == fact.Category && sim.Value == fact.Value {
					shouldSkip = true
					slog.Debug("skipping duplicate fact found via value search", "key", fact.Key)
					break
				}
				// Check if values are very similar (> 90% overlap)
				if sim.Category == fact.Category && s.isSimilarValue(sim.Value, fact.Value) {
					shouldSkip = true
					slog.Debug("skipping similar fact found via value search", "key", fact.Key)
					break
				}
			}
		}

		if shouldSkip {
			result.FactsSkipped++
			continue
		}

		// Create new fact
		createInput := CreateFactInput{
			UserID:     userID,
			Category:   fact.Category,
			Key:        fact.Key,
			Value:      fact.Value,
			Confidence: fact.Confidence,
			Source:     fact.Source,
		}
		created, err := s.userProf.CreateFact(ctx, createInput)
		if err != nil {
			slog.Warn("failed to create fact", "err", err, "key", fact.Key)
			warnings++
			continue
		}
		result.FactsAdded++
		factIDs = append(factIDs, created.FactID)
		slog.Info("created fact", "key", fact.Key, "value", fact.Value, "category", fact.Category)
	}

	result.FactIDs = factIDs
	result.Warnings = warnings

	if warnings > 0 && result.FactsAdded+result.FactsUpdated == 0 {
		result.Status = "failed"
	} else if warnings > 0 {
		result.Status = "partial"
	}

	return result, nil
}

// isSimilarValue checks if two values are similar (> 60% overlap by length).
// This is a simple implementation using string comparison.
// A more sophisticated implementation would use embeddings for semantic similarity.
func (s *ExtractorService) isSimilarValue(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return true
	}

	// Containment check with a ratio guard to avoid single-word matches being too broad.
	// Uses 0.6 threshold: "Python programming" / "Python programming language" = 0.67 (similar),
	// but "Python" / "Python programming" = 0.33 (not similar enough).
	if strings.Contains(a, b) || strings.Contains(b, a) {
		shorter, longer := a, b
		if len(b) < len(a) {
			shorter, longer = b, a
		}
		ratio := float64(len(shorter)) / float64(len(longer))
		return ratio >= 0.6
	}

	return false
}

// HasLLM returns true if an LLM client is configured.
func (s *ExtractorService) HasLLM() bool {
	return s.llm != nil
}

// ExtractFromContent extracts facts from a single content string.
// This is a convenience method for extracting facts without a full conversation.
func (s *ExtractorService) ExtractFromContent(ctx context.Context, userID, content string) (*ExtractResult, error) {
	if content == "" {
		return nil, &domain.ValidationError{Field: "content", Message: "required"}
	}

	return s.Extract(ctx, ExtractRequest{
		Messages: []IngestMessage{{Role: "user", Content: content}},
		UserID:   userID,
	})
}

// GetExplicitTriggerPatterns returns the list of explicit trigger patterns for testing.
func GetExplicitTriggerPatterns() []ExplicitTriggerPattern {
	return explicitTriggerPatterns
}

// NormalizeFactKey normalizes a fact key for consistent storage.
func NormalizeFactKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	// Replace spaces and special chars with underscores
	key = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(key, "_")
	// Remove leading/trailing underscores
	key = strings.Trim(key, "_")
	return key
}

// init registers the custom JSON marshaler for ExtractResult.
func init() {
	// Ensure JSON marshaling works correctly
	var _ json.Marshaler = (*ExtractResult)(nil)
}

// MarshalJSON ensures ExtractResult marshals correctly.
func (r ExtractResult) MarshalJSON() ([]byte, error) {
	type Alias ExtractResult
	return json.Marshal(struct {
		Alias
	}{
		Alias: Alias(r),
	})
}
