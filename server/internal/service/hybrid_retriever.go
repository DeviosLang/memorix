package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/devioslang/memorix/server/internal/tokenizer"
	"github.com/devioslang/memorix/server/internal/vectorstore"
)

// HybridRetriever implements a multi-layer retrieval strategy that combines
// results from User Profile Store, Conversation Summary Pool, and Vector Store.
//
// Retrieval Order (priority):
// 1. User Profile Store - Exact match on key/category
// 2. Conversation Summary Pool - Keyword + time range
// 3. Vector Store (Experience) - Semantic similarity
//
// Scoring: FinalScore = w1*ExactMatch + w2*Semantic + w3*TimeDecay + w4*Importance
type HybridRetriever struct {
	userProfileRepo  repository.UserProfileFactRepo
	summaryRepo      repository.ConversationSummaryRepo
	experienceStore  vectorstore.VectorStore
	config           domain.HybridRetrieverConfig
	tokenizer        tokenizer.Tokenizer
	embedder         Embedder
}

// Embedder interface for generating query embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// NewHybridRetriever creates a new HybridRetriever instance.
func NewHybridRetriever(
	userProfileRepo repository.UserProfileFactRepo,
	summaryRepo repository.ConversationSummaryRepo,
	experienceStore vectorstore.VectorStore,
	embedder Embedder,
	config domain.HybridRetrieverConfig,
) *HybridRetriever {
	if config.DefaultTopK <= 0 {
		config.DefaultTopK = 10
	}
	if config.DefaultMaxTokens <= 0 {
		config.DefaultMaxTokens = 2000
	}
	if config.DefaultTimeDecayRate <= 0 {
		config.DefaultTimeDecayRate = 0.001
	}
	if config.SummaryTimeRangeHours <= 0 {
		config.SummaryTimeRangeHours = 168
	}
	if config.DefaultWeights.W1ExactMatch+config.DefaultWeights.W2Semantic+
		config.DefaultWeights.W3TimeDecay+config.DefaultWeights.W4Importance == 0 {
		config.DefaultWeights = domain.DefaultHybridRetrieverConfig().DefaultWeights
	}

	return &HybridRetriever{
		userProfileRepo:  userProfileRepo,
		summaryRepo:      summaryRepo,
		experienceStore:  experienceStore,
		embedder:         embedder,
		config:           config,
		tokenizer:        tokenizer.NewDefault(),
	}
}

// Retrieve performs hybrid retrieval across all memory layers.
// The retrieval strategy follows a priority order:
// 1. User Profile Store - Exact match for factual queries
// 2. Conversation Summary Pool - Keyword match for recent context
// 3. Vector Store - Semantic similarity for related experiences
//
// Results are merged, deduplicated, scored, and trimmed to fit the token budget.
func (r *HybridRetriever) Retrieve(
	ctx context.Context,
	userID string,
	query string,
	retrievalCtx *domain.HybridRetrievalContext,
) (*domain.HybridRetrievalResult, error) {
	startTime := time.Now()

	// Apply defaults to retrieval context
	if retrievalCtx == nil {
		retrievalCtx = &domain.HybridRetrievalContext{}
	}
	topK := retrievalCtx.TopK
	if topK <= 0 {
		topK = r.config.DefaultTopK
	}
	maxTokens := retrievalCtx.MaxTokens
	if maxTokens <= 0 {
		maxTokens = r.config.DefaultMaxTokens
	}
	minScore := retrievalCtx.MinScore
	if minScore == 0 {
		minScore = r.config.DefaultMinScore
	}
	timeDecayRate := retrievalCtx.TimeDecayRate
	if timeDecayRate <= 0 {
		timeDecayRate = r.config.DefaultTimeDecayRate
	}

	// Get scoring weights
	weights := r.config.DefaultWeights
	if retrievalCtx.CustomWeights != nil {
		weights = *retrievalCtx.CustomWeights
	}

	// Detect query intent
	queryIntent := r.detectQueryIntent(query)
	keywords := r.extractKeywords(query)
	detectedCategories := r.detectCategories(query)

	slog.Info("hybrid retrieval started",
		"user_id", userID,
		"query", query,
		"intent", queryIntent,
		"keywords", keywords,
		"top_k", topK,
		"max_tokens", maxTokens,
	)

	// Initialize result tracking
	var allResults []domain.HybridRetrieverResult
	var layerStats []domain.LayerRetrievalStats
	trace := domain.RetrievalTrace{
		QueryIntent:         queryIntent,
		ExtractedKeywords:   keywords,
		DetectedCategories:  detectedCategories,
		DeduplicationStats:  domain.DeduplicationStats{},
	}

	// Layer 1: User Profile Store (exact match)
	if r.shouldSearchLayer(domain.RetrievalLayerUserProfile, retrievalCtx.IncludeLayers) {
		profileResults, profileStats := r.retrieveFromUserProfile(
			ctx, userID, query, detectedCategories, weights, timeDecayRate,
		)
		allResults = append(allResults, profileResults...)
		layerStats = append(layerStats, profileStats)
	}

	// Layer 2: Conversation Summary Pool (keyword + time range)
	if r.shouldSearchLayer(domain.RetrievalLayerConversationSummary, retrievalCtx.IncludeLayers) {
		summaryResults, summaryStats := r.retrieveFromConversationSummary(
			ctx, userID, query, keywords, retrievalCtx.TimeRange, weights, timeDecayRate, topK,
		)
		allResults = append(allResults, summaryResults...)
		layerStats = append(layerStats, summaryStats)
	}

	// Layer 3: Vector Store (semantic similarity)
	if r.shouldSearchLayer(domain.RetrievalLayerExperience, retrievalCtx.IncludeLayers) && r.experienceStore != nil {
		expResults, expStats := r.retrieveFromExperience(
			ctx, userID, query, retrievalCtx, weights, timeDecayRate, topK, minScore,
		)
		allResults = append(allResults, expResults...)
		layerStats = append(layerStats, expStats)
	}

	// Deduplicate and sort
	trace.DeduplicationStats.TotalCandidates = len(allResults)
	dedupedResults := r.deduplicateResults(allResults)
	trace.DeduplicationStats.DuplicatesRemoved = len(allResults) - len(dedupedResults)
	trace.DeduplicationStats.DeduplicationMethod = "content_similarity"

	// Sort by final score (descending)
	r.sortResultsByScore(dedupedResults)

	// Trim to token budget
	totalTokens := r.calculateTotalTokens(dedupedResults)
	truncated := false
	if totalTokens > maxTokens {
		dedupedResults = r.trimToTokenBudget(dedupedResults, maxTokens)
		totalTokens = r.calculateTotalTokens(dedupedResults)
		truncated = true
	}

	latency := time.Since(startTime).Milliseconds()

	slog.Info("hybrid retrieval completed",
		"user_id", userID,
		"query", query,
		"total_candidates", trace.DeduplicationStats.TotalCandidates,
		"duplicates_removed", trace.DeduplicationStats.DuplicatesRemoved,
		"final_results", len(dedupedResults),
		"total_tokens", totalTokens,
		"truncated", truncated,
		"latency_ms", latency,
	)

	return &domain.HybridRetrievalResult{
		Results:          dedupedResults,
		TotalTokens:      totalTokens,
		MaxTokens:        maxTokens,
		Truncated:        truncated,
		LayerStatistics:  layerStats,
		RetrievalTrace:   trace,
		Query:            query,
		UserID:           userID,
		LatencyMs:        latency,
	}, nil
}

// shouldSearchLayer checks if a layer should be searched.
func (r *HybridRetriever) shouldSearchLayer(layer domain.RetrievalSourceLayer, includeLayers []domain.RetrievalSourceLayer) bool {
	if len(includeLayers) == 0 {
		return true
	}
	for _, l := range includeLayers {
		if l == layer {
			return true
		}
	}
	return false
}

// retrieveFromUserProfile retrieves from User Profile Store.
// Uses exact matching on key and category.
func (r *HybridRetriever) retrieveFromUserProfile(
	ctx context.Context,
	userID string,
	query string,
	detectedCategories []domain.FactCategory,
	weights domain.ScoringWeights,
	timeDecayRate float64,
) ([]domain.HybridRetrieverResult, domain.LayerRetrievalStats) {
	startTime := time.Now()
	var results []domain.HybridRetrieverResult

	// Get all facts for user
	facts, err := r.userProfileRepo.GetByUserID(ctx, userID)
	if err != nil {
		slog.Warn("failed to retrieve user profile facts", "user_id", userID, "err", err)
		return results, domain.LayerRetrievalStats{
			Layer:          domain.RetrievalLayerUserProfile,
			CandidatesFound: 0,
			Error:          err.Error(),
			LatencyMs:      time.Since(startTime).Milliseconds(),
		}
	}

	// Score each fact
	queryLower := strings.ToLower(query)
	for _, fact := range facts {
		// Calculate exact match score
		exactMatchScore := r.calculateExactMatchScore(queryLower, fact.Key, fact.Value, fact.Category)

		// Skip if below threshold
		if exactMatchScore < 0.5 {
			continue
		}

		// Calculate time decay score
		timeDecayScore := r.calculateTimeDecayScore(fact.CreatedAt, timeDecayRate)

		// Calculate importance score (use confidence)
		importanceScore := fact.Confidence

		// Calculate final score
		finalScore := weights.W1ExactMatch*exactMatchScore +
			weights.W2Semantic*1.0 + // Exact match means high semantic relevance
			weights.W3TimeDecay*timeDecayScore +
			weights.W4Importance*importanceScore

		content := fmt.Sprintf("[%s] %s: %s", fact.Category, fact.Key, fact.Value)
		result := domain.HybridRetrieverResult{
			ID:          fact.FactID,
			Content:     content,
			SourceLayer: domain.RetrievalLayerUserProfile,
			SourceType:  "fact",
			ScoreBreakdown: domain.ScoreBreakdown{
				ExactMatchScore: exactMatchScore,
				SemanticScore:   1.0,
				TimeDecayScore:  timeDecayScore,
				ImportanceScore: importanceScore,
				Weights:         weights,
			},
			FinalScore: finalScore,
			TokenCount: r.tokenizer.CountTokens(content),
			CreatedAt:  fact.CreatedAt,
			Metadata: map[string]interface{}{
				"category":   string(fact.Category),
				"key":        fact.Key,
				"value":      fact.Value,
				"confidence": fact.Confidence,
				"source":     string(fact.Source),
			},
		}
		results = append(results, result)
	}

	latency := time.Since(startTime).Milliseconds()
	slog.Debug("user profile retrieval completed",
		"user_id", userID,
		"candidates", len(results),
		"latency_ms", latency,
	)

	return results, domain.LayerRetrievalStats{
		Layer:            domain.RetrievalLayerUserProfile,
		CandidatesFound:  len(results),
		CandidatesReturned: len(results),
		AvgScore:         r.calculateAverageScore(results),
		LatencyMs:        latency,
	}
}

// retrieveFromConversationSummary retrieves from Conversation Summary Pool.
// Uses keyword matching and time range filtering.
func (r *HybridRetriever) retrieveFromConversationSummary(
	ctx context.Context,
	userID string,
	query string,
	keywords []string,
	timeRange *domain.TimeRange,
	weights domain.ScoringWeights,
	timeDecayRate float64,
	topK int,
) ([]domain.HybridRetrieverResult, domain.LayerRetrievalStats) {
	startTime := time.Now()
	var results []domain.HybridRetrieverResult

	// Determine time range
	var startTimeFilter, endTimeFilter time.Time
	if timeRange != nil {
		startTimeFilter = timeRange.Start
		endTimeFilter = timeRange.End
	} else {
		// Default to last N hours
		endTimeFilter = time.Now()
		startTimeFilter = endTimeFilter.Add(-time.Duration(r.config.SummaryTimeRangeHours) * time.Hour)
	}

	// Get summaries for user
	summaries, err := r.summaryRepo.GetByUserID(ctx, userID, topK*2)
	if err != nil {
		slog.Warn("failed to retrieve conversation summaries", "user_id", userID, "err", err)
		return results, domain.LayerRetrievalStats{
			Layer:          domain.RetrievalLayerConversationSummary,
			CandidatesFound: 0,
			Error:          err.Error(),
			LatencyMs:      time.Since(startTime).Milliseconds(),
		}
	}

	// Score each summary
	queryLower := strings.ToLower(query)
	for _, summary := range summaries {
		// Filter by time range
		if summary.CreatedAt.Before(startTimeFilter) || summary.CreatedAt.After(endTimeFilter) {
			continue
		}

		// Calculate keyword match score
		semanticScore := r.calculateKeywordMatchScore(queryLower, keywords, summary)

		// Skip if below threshold
		if semanticScore < 0.2 {
			continue
		}

		// Calculate time decay score
		timeDecayScore := r.calculateTimeDecayScore(summary.CreatedAt, timeDecayRate)

		// Calculate importance score based on key topics count
		importanceScore := float64(len(summary.KeyTopics)) / 5.0
		if importanceScore > 1.0 {
			importanceScore = 1.0
		}

		// Calculate final score
		finalScore := weights.W1ExactMatch*0.0 + // No exact match for summaries
			weights.W2Semantic*semanticScore +
			weights.W3TimeDecay*timeDecayScore +
			weights.W4Importance*importanceScore

		content := fmt.Sprintf("Title: %s\nSummary: %s\nTopics: %v\nUser Intent: %s",
			summary.Title, summary.Summary, summary.KeyTopics, summary.UserIntent)

		result := domain.HybridRetrieverResult{
			ID:          summary.SummaryID,
			Content:     content,
			SourceLayer: domain.RetrievalLayerConversationSummary,
			SourceType:  "summary",
			ScoreBreakdown: domain.ScoreBreakdown{
				ExactMatchScore: 0.0,
				SemanticScore:   semanticScore,
				TimeDecayScore:  timeDecayScore,
				ImportanceScore: importanceScore,
				Weights:         weights,
			},
			FinalScore: finalScore,
			TokenCount: r.tokenizer.CountTokens(content),
			CreatedAt:  summary.CreatedAt,
			Metadata: map[string]interface{}{
				"title":       summary.Title,
				"key_topics":  summary.KeyTopics,
				"user_intent": summary.UserIntent,
				"session_id":  summary.SessionID,
			},
		}
		results = append(results, result)
	}

	latency := time.Since(startTime).Milliseconds()
	slog.Debug("conversation summary retrieval completed",
		"user_id", userID,
		"candidates", len(results),
		"latency_ms", latency,
	)

	return results, domain.LayerRetrievalStats{
		Layer:            domain.RetrievalLayerConversationSummary,
		CandidatesFound:  len(results),
		CandidatesReturned: len(results),
		AvgScore:         r.calculateAverageScore(results),
		LatencyMs:        latency,
	}
}

// retrieveFromExperience retrieves from Vector Store (Experience).
// Uses semantic similarity via vector search.
func (r *HybridRetriever) retrieveFromExperience(
	ctx context.Context,
	userID string,
	query string,
	retrievalCtx *domain.HybridRetrievalContext,
	weights domain.ScoringWeights,
	timeDecayRate float64,
	topK int,
	minScore float64,
) ([]domain.HybridRetrieverResult, domain.LayerRetrievalStats) {
	startTime := time.Now()
	var results []domain.HybridRetrieverResult

	// Generate query embedding
	if r.embedder == nil {
		return results, domain.LayerRetrievalStats{
			Layer:          domain.RetrievalLayerExperience,
			CandidatesFound: 0,
			Error:          "embedder not configured",
			LatencyMs:      time.Since(startTime).Milliseconds(),
		}
	}

	queryVec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		slog.Warn("failed to embed query", "err", err)
		return results, domain.LayerRetrievalStats{
			Layer:          domain.RetrievalLayerExperience,
			CandidatesFound: 0,
			Error:          err.Error(),
			LatencyMs:      time.Since(startTime).Milliseconds(),
		}
	}

	// Build filter
	filter := domain.ExperienceFilter{
		UserID:   userID,
		Query:    query,
		MinScore: minScore,
		Limit:    topK,
	}
	if retrievalCtx != nil && len(retrievalCtx.Topics) > 0 {
		filter.Topic = retrievalCtx.Topics[0] // Use first topic
	}

	// Perform vector search
	searchResult, err := r.experienceStore.Search(ctx, userID, queryVec, filter)
	if err != nil {
		slog.Warn("failed to search experiences", "user_id", userID, "err", err)
		return results, domain.LayerRetrievalStats{
			Layer:          domain.RetrievalLayerExperience,
			CandidatesFound: 0,
			Error:          err.Error(),
			LatencyMs:      time.Since(startTime).Milliseconds(),
		}
	}

	// Score each experience
	for _, exp := range searchResult.Experiences {
		// Get semantic score from vector search result
		semanticScore := 0.5
		if exp.Score != nil {
			semanticScore = *exp.Score
		}

		// Calculate time decay score
		timeDecayScore := r.calculateTimeDecayScore(exp.CreatedAt, timeDecayRate)

		// Calculate importance score (use confidence from metadata)
		importanceScore := exp.Metadata.Confidence
		if importanceScore <= 0 {
			importanceScore = 0.5
		}

		// Calculate final score
		finalScore := weights.W1ExactMatch*0.0 + // No exact match for experiences
			weights.W2Semantic*semanticScore +
			weights.W3TimeDecay*timeDecayScore +
			weights.W4Importance*importanceScore

		result := domain.HybridRetrieverResult{
			ID:          exp.ExperienceID,
			Content:     exp.Content,
			SourceLayer: domain.RetrievalLayerExperience,
			SourceType:  "experience",
			ScoreBreakdown: domain.ScoreBreakdown{
				ExactMatchScore: 0.0,
				SemanticScore:   semanticScore,
				TimeDecayScore:  timeDecayScore,
				ImportanceScore: importanceScore,
				Weights:         weights,
			},
			FinalScore: finalScore,
			TokenCount: r.tokenizer.CountTokens(exp.Content),
			CreatedAt:  exp.CreatedAt,
			Metadata: map[string]interface{}{
				"topic":      exp.Metadata.Topic,
				"outcome":    string(exp.Metadata.Outcome),
				"confidence": exp.Metadata.Confidence,
				"tags":       exp.Metadata.Tags,
				"session_id": exp.Metadata.SessionID,
			},
		}
		results = append(results, result)
	}

	latency := time.Since(startTime).Milliseconds()
	slog.Debug("experience retrieval completed",
		"user_id", userID,
		"candidates", len(results),
		"latency_ms", latency,
	)

	return results, domain.LayerRetrievalStats{
		Layer:            domain.RetrievalLayerExperience,
		CandidatesFound:  len(searchResult.Experiences),
		CandidatesReturned: len(results),
		AvgScore:         r.calculateAverageScore(results),
		LatencyMs:        latency,
	}
}

// detectQueryIntent determines the intent of the query.
func (r *HybridRetriever) detectQueryIntent(query string) string {
	queryLower := strings.ToLower(query)

	// Check for exact fact queries (e.g., "What is my name?", "My preferences")
	exactPatterns := []string{
		"what is my", "what's my", "what are my",
		"my name", "my email", "my phone", "my address",
		"my preference", "my goal", "my skill",
		"i am", "i'm", "i work", "i live",
		"我叫", "我的名字", "我的邮箱", "我的电话",
		"我的偏好", "我的目标", "我的技能",
	}

	for _, pattern := range exactPatterns {
		if strings.Contains(queryLower, pattern) {
			return "exact_fact"
		}
	}

	// Check for semantic/experience queries
	semanticPatterns := []string{
		"discussed", "talked about", "mentioned", "we discussed",
		"before", "previous", "earlier", "last time",
		"solution", "approach", "tried", "attempted",
		"讨论过", "之前", "上次", "解决方案",
	}

	for _, pattern := range semanticPatterns {
		if strings.Contains(queryLower, pattern) {
			return "semantic"
		}
	}

	return "hybrid"
}

// extractKeywords extracts keywords from the query.
func (r *HybridRetriever) extractKeywords(query string) []string {
	// Simple keyword extraction
	// Remove common stop words
	stopWords := map[string]bool{
		"what": true, "is": true, "my": true, "the": true, "a": true, "an": true,
		"i": true, "you": true, "we": true, "they": true, "it": true,
		"do": true, "did": true, "does": true, "can": true, "could": true,
		"would": true, "should": true, "will": true, "have": true, "has": true,
		"about": true, "for": true, "with": true, "from": true, "to": true,
		"的": true, "是": true, "在": true, "了": true, "和": true,
		"我": true, "你": true, "他": true, "她": true, "它": true,
	}

	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) >= 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// detectCategories detects fact categories from the query.
func (r *HybridRetriever) detectCategories(query string) []domain.FactCategory {
	queryLower := strings.ToLower(query)
	var categories []domain.FactCategory

	// Check for personal category patterns
	personalPatterns := []string{"name", "email", "phone", "address", "age", "birthday", "location", "名字", "邮箱", "电话", "地址", "年龄", "生日", "位置"}
	for _, pattern := range personalPatterns {
		if strings.Contains(queryLower, pattern) {
			categories = append(categories, domain.CategoryPersonal)
			break
		}
	}

	// Check for preference category patterns
	preferencePatterns := []string{"prefer", "like", "favorite", "want", "偏好", "喜欢", "最爱", "想要"}
	for _, pattern := range preferencePatterns {
		if strings.Contains(queryLower, pattern) {
			categories = append(categories, domain.CategoryPreference)
			break
		}
	}

	// Check for goal category patterns
	goalPatterns := []string{"goal", "objective", "target", "aim", "plan", "目标", "计划", "目的"}
	for _, pattern := range goalPatterns {
		if strings.Contains(queryLower, pattern) {
			categories = append(categories, domain.CategoryGoal)
			break
		}
	}

	// Check for skill category patterns
	skillPatterns := []string{"skill", "ability", "expertise", "know", "can do", "技能", "能力", "专长", "会"}
	for _, pattern := range skillPatterns {
		if strings.Contains(queryLower, pattern) {
			categories = append(categories, domain.CategorySkill)
			break
		}
	}

	// If no categories detected, include all
	if len(categories) == 0 {
		categories = []domain.FactCategory{
			domain.CategoryPersonal,
			domain.CategoryPreference,
			domain.CategoryGoal,
			domain.CategorySkill,
		}
	}

	return categories
}

// calculateExactMatchScore calculates the exact match score for a user profile fact.
func (r *HybridRetriever) calculateExactMatchScore(queryLower string, key, value string, category domain.FactCategory) float64 {
	keyLower := strings.ToLower(key)
	valueLower := strings.ToLower(value)
	categoryLower := strings.ToLower(string(category))

	// Direct match on key (highest score)
	if strings.Contains(queryLower, keyLower) || strings.Contains(keyLower, queryLower) {
		return 1.0
	}

	// Match on value
	if strings.Contains(queryLower, valueLower) {
		return 0.8
	}

	// Match on category keywords
	if strings.Contains(queryLower, categoryLower) {
		return 0.7
	}

	// Check if any word in query matches key
	queryWords := strings.Fields(queryLower)
	for _, word := range queryWords {
		if len(word) >= 3 && strings.Contains(keyLower, word) {
			return 0.6
		}
	}

	return 0.0
}

// calculateKeywordMatchScore calculates the keyword match score for a conversation summary.
func (r *HybridRetriever) calculateKeywordMatchScore(queryLower string, keywords []string, summary domain.ConversationSummary) float64 {
	// Combine all summary text
	summaryText := strings.ToLower(strings.Join([]string{
		summary.Title, summary.Summary, summary.UserIntent,
		strings.Join(summary.KeyTopics, " "),
	}, " "))

	// Count keyword matches
	matchCount := 0
	for _, keyword := range keywords {
		if strings.Contains(summaryText, keyword) {
			matchCount++
		}
	}

	// Calculate score based on match ratio
	if len(keywords) == 0 {
		return 0.0
	}

	score := float64(matchCount) / float64(len(keywords))

	// Boost for title match
	if strings.Contains(strings.ToLower(summary.Title), queryLower) {
		score += 0.3
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// calculateTimeDecayScore calculates the time decay score based on age.
func (r *HybridRetriever) calculateTimeDecayScore(createdAt time.Time, decayRate float64) float64 {
	hoursSinceCreation := time.Since(createdAt).Hours()
	score := 1.0 / (1.0 + decayRate*hoursSinceCreation)

	// Ensure score is between 0 and 1
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// deduplicateResults removes duplicate results based on content similarity.
func (r *HybridRetriever) deduplicateResults(results []domain.HybridRetrieverResult) []domain.HybridRetrieverResult {
	if len(results) <= 1 {
		return results
	}

	// Use a map to track seen content hashes
	seen := make(map[string]bool)
	var deduped []domain.HybridRetrieverResult

	for _, result := range results {
		// Create a simple hash based on content
		hash := r.contentHash(result.Content)
		if !seen[hash] {
			seen[hash] = true
			deduped = append(deduped, result)
		}
	}

	return deduped
}

// contentHash creates a simple hash of content for deduplication.
func (r *HybridRetriever) contentHash(content string) string {
	// Normalize content: lowercase, remove extra spaces
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = strings.Join(strings.Fields(normalized), " ")

	// For deduplication, use first 100 chars as a simple hash
	if len(normalized) > 100 {
		normalized = normalized[:100]
	}

	return normalized
}

// sortResultsByScore sorts results by final score in descending order.
func (r *HybridRetriever) sortResultsByScore(results []domain.HybridRetrieverResult) {
	// Use simple insertion sort for small arrays
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].FinalScore > results[j-1].FinalScore; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

// calculateTotalTokens calculates the total token count of all results.
func (r *HybridRetriever) calculateTotalTokens(results []domain.HybridRetrieverResult) int {
	total := 0
	for _, result := range results {
		total += result.TokenCount
	}
	return total
}

// trimToTokenBudget trims results to fit within the token budget.
// Keeps highest-scoring results first.
func (r *HybridRetriever) trimToTokenBudget(results []domain.HybridRetrieverResult, maxTokens int) []domain.HybridRetrieverResult {
	if len(results) == 0 {
		return results
	}

	var trimmed []domain.HybridRetrieverResult
	totalTokens := 0

	for _, result := range results {
		if totalTokens+result.TokenCount <= maxTokens {
			trimmed = append(trimmed, result)
			totalTokens += result.TokenCount
		}
	}

	return trimmed
}

// calculateAverageScore calculates the average final score of results.
func (r *HybridRetriever) calculateAverageScore(results []domain.HybridRetrieverResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	total := 0.0
	for _, result := range results {
		total += result.FinalScore
	}

	return total / float64(len(results))
}

// WithTokenizer sets a custom tokenizer for the retriever.
func (r *HybridRetriever) WithTokenizer(t tokenizer.Tokenizer) *HybridRetriever {
	r.tokenizer = t
	return r
}

// GetConfig returns the current configuration.
func (r *HybridRetriever) GetConfig() domain.HybridRetrieverConfig {
	return r.config
}
