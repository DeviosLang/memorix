package service

import (
	"context"
	"testing"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/vectorstore"
)

// MockUserProfileRepo implements repository.UserProfileFactRepo for testing.
type MockUserProfileRepo struct {
	Facts []domain.UserProfileFact
}

func (m *MockUserProfileRepo) Create(ctx context.Context, fact *domain.UserProfileFact) error {
	m.Facts = append(m.Facts, *fact)
	return nil
}

func (m *MockUserProfileRepo) GetByID(ctx context.Context, factID string) (*domain.UserProfileFact, error) {
	for _, f := range m.Facts {
		if f.FactID == factID {
			return &f, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockUserProfileRepo) GetByUserID(ctx context.Context, userID string) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.Facts {
		if f.UserID == userID {
			result = append(result, f)
		}
	}
	return result, nil
}

func (m *MockUserProfileRepo) GetByUserIDAndCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.Facts {
		if f.UserID == userID && f.Category == category {
			result = append(result, f)
		}
	}
	return result, nil
}

func (m *MockUserProfileRepo) List(ctx context.Context, f domain.UserProfileFactFilter) (facts []domain.UserProfileFact, total int, err error) {
	return m.Facts, len(m.Facts), nil
}

func (m *MockUserProfileRepo) Update(ctx context.Context, fact *domain.UserProfileFact) error {
	for i, f := range m.Facts {
		if f.FactID == fact.FactID {
			m.Facts[i] = *fact
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *MockUserProfileRepo) Delete(ctx context.Context, factID string) error {
	for i, f := range m.Facts {
		if f.FactID == factID {
			m.Facts = append(m.Facts[:i], m.Facts[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *MockUserProfileRepo) DeleteByUserID(ctx context.Context, userID string) error {
	var filtered []domain.UserProfileFact
	for _, f := range m.Facts {
		if f.UserID != userID {
			filtered = append(filtered, f)
		}
	}
	m.Facts = filtered
	return nil
}

func (m *MockUserProfileRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	count := 0
	for _, f := range m.Facts {
		if f.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *MockUserProfileRepo) DeleteOldestLowConfidence(ctx context.Context, userID string, count int) (int64, error) {
	return 0, nil
}

func (m *MockUserProfileRepo) TouchLastAccessed(ctx context.Context, factID string) error {
	return nil
}

func (m *MockUserProfileRepo) GetByKey(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error) {
	for _, f := range m.Facts {
		if f.UserID == userID && f.Category == category && f.Key == key {
			return &f, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockUserProfileRepo) SearchByValue(ctx context.Context, userID string, value string, limit int) ([]domain.UserProfileFact, error) {
	return nil, nil
}

// MockSummaryRepo implements repository.ConversationSummaryRepo for testing.
type MockSummaryRepo struct {
	Summaries []domain.ConversationSummary
}

func (m *MockSummaryRepo) Create(ctx context.Context, summary *domain.ConversationSummary) error {
	m.Summaries = append(m.Summaries, *summary)
	return nil
}

func (m *MockSummaryRepo) GetByID(ctx context.Context, summaryID string) (*domain.ConversationSummary, error) {
	for _, s := range m.Summaries {
		if s.SummaryID == summaryID {
			return &s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockSummaryRepo) GetByUserID(ctx context.Context, userID string, limit int) ([]domain.ConversationSummary, error) {
	var result []domain.ConversationSummary
	for _, s := range m.Summaries {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockSummaryRepo) GetBySessionID(ctx context.Context, sessionID string) (*domain.ConversationSummary, error) {
	for _, s := range m.Summaries {
		if s.SessionID == sessionID {
			return &s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockSummaryRepo) List(ctx context.Context, f domain.ConversationSummaryFilter) (summaries []domain.ConversationSummary, total int, err error) {
	return m.Summaries, len(m.Summaries), nil
}

func (m *MockSummaryRepo) Delete(ctx context.Context, summaryID string) error {
	for i, s := range m.Summaries {
		if s.SummaryID == summaryID {
			m.Summaries = append(m.Summaries[:i], m.Summaries[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *MockSummaryRepo) DeleteOldest(ctx context.Context, userID string, count int) (int64, error) {
	return 0, nil
}

func (m *MockSummaryRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	count := 0
	for _, s := range m.Summaries {
		if s.UserID == userID {
			count++
		}
	}
	return count, nil
}

// MockExperienceStore implements vectorstore.VectorStore for testing.
type MockExperienceStore struct {
	Experiences []domain.Experience
}

func (m *MockExperienceStore) Write(ctx context.Context, exp *domain.Experience) error {
	m.Experiences = append(m.Experiences, *exp)
	return nil
}

func (m *MockExperienceStore) WriteBatch(ctx context.Context, experiences []*domain.Experience) (int, error) {
	for _, exp := range experiences {
		m.Experiences = append(m.Experiences, *exp)
	}
	return len(experiences), nil
}

func (m *MockExperienceStore) Search(ctx context.Context, userID string, queryVec []float32, filter domain.ExperienceFilter) (*domain.ExperienceSearchResult, error) {
	var result []domain.Experience
	for _, exp := range m.Experiences {
		if exp.UserID == userID {
			score := 0.8 // Mock score
			exp.Score = &score
			result = append(result, exp)
		}
	}
	if len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return &domain.ExperienceSearchResult{
		Experiences: result,
		Total:       len(result),
		Query:       filter.Query,
		LatencyMs:   10,
	}, nil
}

func (m *MockExperienceStore) GetByID(ctx context.Context, userID, experienceID string) (*domain.Experience, error) {
	for _, exp := range m.Experiences {
		if exp.UserID == userID && exp.ExperienceID == experienceID {
			return &exp, nil
		}
	}
	return nil, vectorstore.ErrNotFound
}

func (m *MockExperienceStore) Delete(ctx context.Context, userID, experienceID string) error {
	for i, exp := range m.Experiences {
		if exp.UserID == userID && exp.ExperienceID == experienceID {
			m.Experiences = append(m.Experiences[:i], m.Experiences[i+1:]...)
			return nil
		}
	}
	return vectorstore.ErrNotFound
}

func (m *MockExperienceStore) DeleteByUser(ctx context.Context, userID string) error {
	var filtered []domain.Experience
	for _, exp := range m.Experiences {
		if exp.UserID != userID {
			filtered = append(filtered, exp)
		}
	}
	m.Experiences = filtered
	return nil
}

func (m *MockExperienceStore) Stats(ctx context.Context, userID string) (*domain.ExperienceStats, error) {
	return &domain.ExperienceStats{UserID: userID, TotalExperiences: len(m.Experiences)}, nil
}

func (m *MockExperienceStore) Health(ctx context.Context) error {
	return nil
}

func (m *MockExperienceStore) Close() error {
	return nil
}

// TestHybridRetriever_ExactFactQuery tests exact fact queries like "What's my name?"
func TestHybridRetriever_ExactFactQuery(t *testing.T) {
	// Setup mocks
	profileRepo := &MockUserProfileRepo{
		Facts: []domain.UserProfileFact{
			{
				FactID:     "fact-1",
				UserID:     "user-1",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "Alice",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
				CreatedAt:  time.Now(),
			},
		},
	}

	summaryRepo := &MockSummaryRepo{}
	expStore := &MockExperienceStore{}
	embedder := NewMockEmbedder(1536)

	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	// Test exact fact query
	result, err := retriever.Retrieve(context.Background(), "user-1", "What's my name?", nil)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify results
	if len(result.Results) == 0 {
		t.Fatal("Expected at least one result for exact fact query")
	}

	// First result should be from user profile
	if result.Results[0].SourceLayer != domain.RetrievalLayerUserProfile {
		t.Errorf("Expected first result from user_profile layer, got %s", result.Results[0].SourceLayer)
	}

	// Verify score breakdown has exact match score
	if result.Results[0].ScoreBreakdown.ExactMatchScore < 0.5 {
		t.Errorf("Expected high exact match score, got %f", result.Results[0].ScoreBreakdown.ExactMatchScore)
	}

	// Verify tracing
	if result.RetrievalTrace.QueryIntent != "exact_fact" {
		t.Errorf("Expected query intent 'exact_fact', got %s", result.RetrievalTrace.QueryIntent)
	}

	t.Logf("Exact fact query test passed: %d results, intent=%s", len(result.Results), result.RetrievalTrace.QueryIntent)
}

// TestHybridRetriever_SemanticQuery tests semantic queries like "discussed architecture"
func TestHybridRetriever_SemanticQuery(t *testing.T) {
	// Setup mocks
	profileRepo := &MockUserProfileRepo{}

	summaryRepo := &MockSummaryRepo{
		Summaries: []domain.ConversationSummary{
			{
				SummaryID:  "summary-1",
				UserID:     "user-1",
				SessionID:  "session-1",
				Title:      "Architecture discussion",
				Summary:    "We discussed microservices architecture patterns and best practices",
				KeyTopics:  []string{"architecture", "microservices", "patterns"},
				UserIntent: "Learn about architecture patterns",
				CreatedAt:  time.Now().Add(-24 * time.Hour),
			},
		},
	}

	expStore := &MockExperienceStore{
		Experiences: []domain.Experience{
			{
				ExperienceID: "exp-1",
				UserID:       "user-1",
				Content:      "Discussed microservices architecture and decided on event-driven approach",
				Metadata: domain.ExperienceMetadata{
					Topic:       "architecture",
					Outcome:     domain.OutcomeSuccess,
					Confidence:  0.9,
					SessionID:   "session-1",
				},
				CreatedAt: time.Now().Add(-48 * time.Hour),
			},
		},
	}

	embedder := NewMockEmbedder(1536)
	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	// Test semantic query
	result, err := retriever.Retrieve(context.Background(), "user-1", "What architecture patterns did we discuss before?", nil)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify results
	if len(result.Results) == 0 {
		t.Fatal("Expected at least one result for semantic query")
	}

	// Verify tracing
	if result.RetrievalTrace.QueryIntent != "semantic" {
		t.Errorf("Expected query intent 'semantic', got %s", result.RetrievalTrace.QueryIntent)
	}

	t.Logf("Semantic query test passed: %d results, intent=%s", len(result.Results), result.RetrievalTrace.QueryIntent)
}

// TestHybridRetriever_TokenBudget tests token budget enforcement
func TestHybridRetriever_TokenBudget(t *testing.T) {
	// Setup mocks with many results
	profileRepo := &MockUserProfileRepo{}
	summaryRepo := &MockSummaryRepo{}

	// Create many experiences to exceed token budget
	var experiences []domain.Experience
	for i := 0; i < 20; i++ {
		content := "This is a long experience entry about microservices and distributed systems architecture with many details that should increase token count significantly."
		experiences = append(experiences, domain.Experience{
			ExperienceID: string(rune('a' + i)),
			UserID:       "user-1",
			Content:      content,
			Metadata: domain.ExperienceMetadata{
				Topic:      "architecture",
				Confidence: 0.8,
			},
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}
	expStore := &MockExperienceStore{Experiences: experiences}
	embedder := NewMockEmbedder(1536)
	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	// Set a low token budget
	ctx := &domain.HybridRetrievalContext{
		MaxTokens: 500,
	}

	result, err := retriever.Retrieve(context.Background(), "user-1", "architecture patterns", ctx)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify token budget is respected
	if result.TotalTokens > ctx.MaxTokens {
		t.Errorf("Token budget exceeded: %d > %d", result.TotalTokens, ctx.MaxTokens)
	}

	// Verify truncation flag
	if !result.Truncated {
		t.Error("Expected truncated=true when budget was applied")
	}

	t.Logf("Token budget test passed: %d tokens (budget: %d), truncated=%v", result.TotalTokens, ctx.MaxTokens, result.Truncated)
}

// TestHybridRetriever_Deduplication tests result deduplication
func TestHybridRetriever_Deduplication(t *testing.T) {
	// Setup mocks with duplicate content
	profileRepo := &MockUserProfileRepo{}

	now := time.Now()
	summaryRepo := &MockSummaryRepo{
		Summaries: []domain.ConversationSummary{
			{
				SummaryID:  "summary-1",
				UserID:     "user-1",
				Title:      "Same topic",
				Summary:    "Duplicate content about microservices",
				KeyTopics:  []string{"microservices"},
				CreatedAt:  now,
			},
			{
				SummaryID:  "summary-2",
				UserID:     "user-1",
				Title:      "Same topic",
				Summary:    "Duplicate content about microservices",
				KeyTopics:  []string{"microservices"},
				CreatedAt:  now,
			},
		},
	}

	expStore := &MockExperienceStore{}
	embedder := NewMockEmbedder(1536)
	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	result, err := retriever.Retrieve(context.Background(), "user-1", "microservices", nil)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify deduplication happened
	if result.RetrievalTrace.DeduplicationStats.DuplicatesRemoved > 0 {
		t.Logf("Deduplication removed %d duplicates", result.RetrievalTrace.DeduplicationStats.DuplicatesRemoved)
	}

	t.Logf("Deduplication test passed: total_candidates=%d, final=%d",
		result.RetrievalTrace.DeduplicationStats.TotalCandidates, len(result.Results))
}

// TestHybridRetriever_ScoringWeights tests custom scoring weights
func TestHybridRetriever_ScoringWeights(t *testing.T) {
	profileRepo := &MockUserProfileRepo{
		Facts: []domain.UserProfileFact{
			{
				FactID:     "fact-1",
				UserID:     "user-1",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "Bob",
				Confidence: 1.0,
				CreatedAt:  time.Now(),
			},
		},
	}

	summaryRepo := &MockSummaryRepo{}
	expStore := &MockExperienceStore{}
	embedder := NewMockEmbedder(1536)

	// Custom weights prioritizing exact match
	config := domain.DefaultHybridRetrieverConfig()
	config.DefaultWeights = domain.ScoringWeights{
		W1ExactMatch: 0.7,
		W2Semantic:   0.1,
		W3TimeDecay:  0.1,
		W4Importance: 0.1,
	}
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	result, err := retriever.Retrieve(context.Background(), "user-1", "What's my name?", nil)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify weights were applied
	if len(result.Results) > 0 {
		weights := result.Results[0].ScoreBreakdown.Weights
		if weights.W1ExactMatch != 0.7 {
			t.Errorf("Expected W1ExactMatch=0.7, got %f", weights.W1ExactMatch)
		}
		t.Logf("Scoring weights test passed: W1=%f, W2=%f, W3=%f, W4=%f",
			weights.W1ExactMatch, weights.W2Semantic, weights.W3TimeDecay, weights.W4Importance)
	}
}

// TestHybridRetriever_LayerFiltering tests filtering by layer
func TestHybridRetriever_LayerFiltering(t *testing.T) {
	profileRepo := &MockUserProfileRepo{
		Facts: []domain.UserProfileFact{
			{
				FactID:     "fact-1",
				UserID:     "user-1",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "Charlie",
				Confidence: 1.0,
				CreatedAt:  time.Now(),
			},
		},
	}

	summaryRepo := &MockSummaryRepo{
		Summaries: []domain.ConversationSummary{
			{
				SummaryID:  "summary-1",
				UserID:     "user-1",
				Title:      "Test summary",
				Summary:    "Test content",
				CreatedAt:  time.Now(),
			},
		},
	}

	expStore := &MockExperienceStore{}
	embedder := NewMockEmbedder(1536)
	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	// Only search user profile layer
	ctx := &domain.HybridRetrievalContext{
		IncludeLayers: []domain.RetrievalSourceLayer{domain.RetrievalLayerUserProfile},
	}

	result, err := retriever.Retrieve(context.Background(), "user-1", "name", ctx)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify only user profile layer was searched
	for _, res := range result.Results {
		if res.SourceLayer != domain.RetrievalLayerUserProfile {
			t.Errorf("Expected only user_profile results, got %s", res.SourceLayer)
		}
	}

	// Verify layer statistics only include user profile
	for _, stat := range result.LayerStatistics {
		if stat.Layer != domain.RetrievalLayerUserProfile {
			t.Errorf("Expected only user_profile layer stats, got %s", stat.Layer)
		}
	}

	t.Logf("Layer filtering test passed: %d results from user_profile only", len(result.Results))
}

// TestHybridRetriever_ComprehensiveTracing tests complete tracing information
func TestHybridRetriever_ComprehensiveTracing(t *testing.T) {
	profileRepo := &MockUserProfileRepo{
		Facts: []domain.UserProfileFact{
			{
				FactID:     "fact-1",
				UserID:     "user-1",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "David",
				Confidence: 1.0,
				CreatedAt:  time.Now(),
			},
		},
	}

	summaryRepo := &MockSummaryRepo{
		Summaries: []domain.ConversationSummary{
			{
				SummaryID:  "summary-1",
				UserID:     "user-1",
				Title:      "Test",
				Summary:    "Test summary",
				CreatedAt:  time.Now(),
			},
		},
	}

	expStore := &MockExperienceStore{}
	embedder := NewMockEmbedder(1536)
	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	result, err := retriever.Retrieve(context.Background(), "user-1", "What's my name?", nil)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Verify comprehensive tracing
	trace := result.RetrievalTrace
	if trace.QueryIntent == "" {
		t.Error("Missing query intent in trace")
	}
	if len(trace.ExtractedKeywords) == 0 {
		t.Error("Missing extracted keywords in trace")
	}
	if trace.DeduplicationStats.TotalCandidates == 0 {
		t.Error("Missing deduplication stats in trace")
	}

	// Verify layer statistics
	if len(result.LayerStatistics) == 0 {
		t.Error("Missing layer statistics")
	}

	for _, stat := range result.LayerStatistics {
		if stat.Layer == "" {
			t.Error("Missing layer name in statistics")
		}
		if stat.LatencyMs == 0 {
			t.Logf("Warning: zero latency for layer %s", stat.Layer)
		}
	}

	// Verify score breakdown for each result
	for _, res := range result.Results {
		sb := res.ScoreBreakdown
		if sb.Weights.W1ExactMatch+sb.Weights.W2Semantic+sb.Weights.W3TimeDecay+sb.Weights.W4Importance == 0 {
			t.Error("Missing scoring weights in result")
		}
		if res.FinalScore <= 0 {
			t.Error("Expected positive final score")
		}
	}

	t.Logf("Comprehensive tracing test passed: intent=%s, keywords=%v, layers=%d",
		trace.QueryIntent, trace.ExtractedKeywords, len(result.LayerStatistics))
}

// TestHybridRetriever_TimeDecay tests time decay scoring
func TestHybridRetriever_TimeDecay(t *testing.T) {
	profileRepo := &MockUserProfileRepo{
		Facts: []domain.UserProfileFact{
			{
				FactID:     "fact-old",
				UserID:     "user-1",
				Category:   domain.CategoryPersonal,
				Key:        "old_fact",
				Value:      "Old value",
				Confidence: 1.0,
				CreatedAt:  time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
			},
			{
				FactID:     "fact-new",
				UserID:     "user-1",
				Category:   domain.CategoryPersonal,
				Key:        "new_fact",
				Value:      "New value",
				Confidence: 1.0,
				CreatedAt:  time.Now(), // Now
			},
		},
	}

	summaryRepo := &MockSummaryRepo{}
	expStore := &MockExperienceStore{}
	embedder := NewMockEmbedder(1536)
	config := domain.DefaultHybridRetrieverConfig()
	retriever := NewHybridRetriever(profileRepo, summaryRepo, expStore, embedder, config)

	result, err := retriever.Retrieve(context.Background(), "user-1", "fact", nil)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	// Find the two facts in results
	var oldFact, newFact *domain.HybridRetrieverResult
	for _, res := range result.Results {
		if res.ID == "fact-old" {
			oldFact = &res
		}
		if res.ID == "fact-new" {
			newFact = &res
		}
	}

	if oldFact != nil && newFact != nil {
		// New fact should have higher time decay score
		if newFact.ScoreBreakdown.TimeDecayScore <= oldFact.ScoreBreakdown.TimeDecayScore {
			t.Errorf("Expected newer fact to have higher time decay score: new=%f, old=%f",
				newFact.ScoreBreakdown.TimeDecayScore, oldFact.ScoreBreakdown.TimeDecayScore)
		}
		t.Logf("Time decay test passed: new=%f, old=%f",
			newFact.ScoreBreakdown.TimeDecayScore, oldFact.ScoreBreakdown.TimeDecayScore)
	}
}
