package service

import (
	"context"
	"errors"
	"testing"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/llm"
)

// mockFactRepo implements repository.UserProfileFactRepo for testing.
type mockFactRepo struct {
	facts        map[string]*domain.UserProfileFact
	getByKeyFunc func(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error)
	searchFunc   func(ctx context.Context, userID string, value string, limit int) ([]domain.UserProfileFact, error)
	createFunc   func(ctx context.Context, fact *domain.UserProfileFact) error
	updateFunc   func(ctx context.Context, fact *domain.UserProfileFact) error
}

func newMockFactRepo() *mockFactRepo {
	return &mockFactRepo{
		facts: make(map[string]*domain.UserProfileFact),
	}
}

func (m *mockFactRepo) Create(ctx context.Context, fact *domain.UserProfileFact) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, fact)
	}
	m.facts[fact.FactID] = fact
	return nil
}

func (m *mockFactRepo) GetByID(ctx context.Context, factID string) (*domain.UserProfileFact, error) {
	fact, ok := m.facts[factID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return fact, nil
}

func (m *mockFactRepo) GetByUserID(ctx context.Context, userID string) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.facts {
		if f.UserID == userID {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockFactRepo) GetByUserIDAndCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.facts {
		if f.UserID == userID && f.Category == category {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockFactRepo) List(ctx context.Context, f domain.UserProfileFactFilter) ([]domain.UserProfileFact, int, error) {
	var result []domain.UserProfileFact
	for _, fact := range m.facts {
		if f.UserID != "" && fact.UserID != f.UserID {
			continue
		}
		result = append(result, *fact)
	}
	return result, len(result), nil
}

func (m *mockFactRepo) Update(ctx context.Context, fact *domain.UserProfileFact) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, fact)
	}
	m.facts[fact.FactID] = fact
	return nil
}

func (m *mockFactRepo) Delete(ctx context.Context, factID string) error {
	delete(m.facts, factID)
	return nil
}

func (m *mockFactRepo) DeleteByUserID(ctx context.Context, userID string) error {
	for id, f := range m.facts {
		if f.UserID == userID {
			delete(m.facts, id)
		}
	}
	return nil
}

func (m *mockFactRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	count := 0
	for _, f := range m.facts {
		if f.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockFactRepo) DeleteOldestLowConfidence(ctx context.Context, userID string, count int) (int64, error) {
	return 0, nil
}

func (m *mockFactRepo) TouchLastAccessed(ctx context.Context, factID string) error {
	return nil
}

func (m *mockFactRepo) GetByKey(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error) {
	if m.getByKeyFunc != nil {
		return m.getByKeyFunc(ctx, userID, category, key)
	}
	for _, f := range m.facts {
		if f.UserID == userID && f.Category == category && f.Key == key {
			return f, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockFactRepo) SearchByValue(ctx context.Context, userID string, value string, limit int) ([]domain.UserProfileFact, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, userID, value, limit)
	}
	return nil, nil
}

// mockLLMClient implements llm.Client for testing.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(ctx context.Context, system, user string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) CompleteJSON(ctx context.Context, system, user string) (string, error) {
	return m.response, m.err
}

func TestDetectExplicitTriggers(t *testing.T) {
	tests := []struct {
		name            string
		messages        []IngestMessage
		expectedFacts   int
		expectedKey     string
		expectedValue   string
		expectedCat     domain.FactCategory
	}{
		{
			name: "Chinese explicit name trigger",
			messages: []IngestMessage{
				{Role: "user", Content: "记住我叫张三"},
			},
			expectedFacts: 1,
			expectedKey:   "name",
			expectedValue: "张三",
			expectedCat:   domain.CategoryPersonal,
		},
		{
			name: "Chinese 'my name is' pattern",
			messages: []IngestMessage{
				{Role: "user", Content: "我的名字是李四"},
			},
			expectedFacts: 1,
			expectedKey:   "name",
			expectedValue: "李四",
			expectedCat:   domain.CategoryPersonal,
		},
		{
			name: "English 'remember my name' trigger",
			messages: []IngestMessage{
				{Role: "user", Content: "Remember my name is John Smith"},
			},
			expectedFacts: 1,
			expectedKey:   "name",
			expectedValue: "John Smith",
			expectedCat:   domain.CategoryPersonal,
		},
		{
			name: "Chinese preference trigger",
			messages: []IngestMessage{
				{Role: "user", Content: "记住我喜欢吃披萨"},
			},
			expectedFacts: 1,
			expectedKey:   "likes",
			expectedValue: "吃披萨",
			expectedCat:   domain.CategoryPreference,
		},
		{
			name: "Chinese skill trigger",
			messages: []IngestMessage{
				{Role: "user", Content: "我会Python和Go"},
			},
			expectedFacts: 1,
			expectedKey:   "skill",
			expectedValue: "Python和Go",
			expectedCat:   domain.CategorySkill,
		},
		{
			name: "Chinese occupation trigger",
			messages: []IngestMessage{
				{Role: "user", Content: "我是后端工程师"},
			},
			expectedFacts: 1,
			expectedKey:   "occupation",
			expectedValue: "后端工程师",
			expectedCat:   domain.CategoryPersonal,
		},
		{
			name: "No explicit trigger - regular message",
			messages: []IngestMessage{
				{Role: "user", Content: "今天天气怎么样？"},
			},
			expectedFacts: 0,
		},
		{
			name: "Assistant message ignored",
			messages: []IngestMessage{
				{Role: "assistant", Content: "记住你的名字是测试用户"},
			},
			expectedFacts: 0,
		},
		{
			name: "Multiple user messages with triggers",
			messages: []IngestMessage{
				{Role: "user", Content: "记住我叫王五"},
				{Role: "assistant", Content: "好的，我记住了。"},
				{Role: "user", Content: "记住我喜欢编程"},
			},
			expectedFacts:   2,
			expectedKey:     "name", // First fact
			expectedValue:   "王五",
			expectedCat:     domain.CategoryPersonal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockFactRepo()
			userProf := NewUserProfileService(repo, 200)
			extractor := NewExtractorService(repo, nil, userProf)

			facts := extractor.detectExplicitTriggers(tt.messages)

			if len(facts) != tt.expectedFacts {
				t.Errorf("expected %d facts, got %d", tt.expectedFacts, len(facts))
				return
			}

			if tt.expectedFacts > 0 {
				if facts[0].Key != tt.expectedKey {
					t.Errorf("expected key %q, got %q", tt.expectedKey, facts[0].Key)
				}
				if facts[0].Value != tt.expectedValue {
					t.Errorf("expected value %q, got %q", tt.expectedValue, facts[0].Value)
				}
				if facts[0].Category != tt.expectedCat {
					t.Errorf("expected category %q, got %q", tt.expectedCat, facts[0].Category)
				}
				if facts[0].Confidence != 1.0 {
					t.Errorf("expected confidence 1.0 for explicit facts, got %f", facts[0].Confidence)
				}
				if facts[0].Source != domain.SourceExplicit {
					t.Errorf("expected source explicit, got %s", facts[0].Source)
				}
			}
		})
	}
}

func TestIsSimilarValue(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "Exact match",
			a:        "张三",
			b:        "张三",
			expected: true,
		},
		{
			name:     "Case insensitive match",
			a:        "Python",
			b:        "python",
			expected: true,
		},
		{
			name:     "High overlap - containment match",
			a:        "Python programming language",
			b:        "Python programming",
			expected: true, // "Python programming" is contained in longer string
		},
		{
			name:     "Low overlap - not similar",
			a:        "Python",
			b:        "Python programming",
			expected: false, // 6/19 = 0.31 ratio, below 0.9 threshold
		},
		{
			name:     "Different values",
			a:        "张三",
			b:        "李四",
			expected: false,
		},
		{
			name:     "Completely different",
			a:        "I like pizza",
			b:        "I hate rain",
			expected: false,
		},
		{
			name:     "Whitespace difference",
			a:        "Python ",
			b:        " Python",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockFactRepo()
			userProf := NewUserProfileService(repo, 200)
			extractor := NewExtractorService(repo, nil, userProf)

			result := extractor.isSimilarValue(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("isSimilarValue(%q, %q) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestMergeFacts(t *testing.T) {
	explicit := []ExtractedFact{
		{Category: domain.CategoryPersonal, Key: "name", Value: "张三", Confidence: 1.0, Source: domain.SourceExplicit},
		{Category: domain.CategoryPreference, Key: "likes", Value: "编程", Confidence: 1.0, Source: domain.SourceExplicit},
	}

	implicit := []ExtractedFact{
		{Category: domain.CategoryPersonal, Key: "name", Value: "李四", Confidence: 0.8, Source: domain.SourceInferred},
		{Category: domain.CategorySkill, Key: "language", Value: "Python", Confidence: 0.9, Source: domain.SourceInferred},
	}

	repo := newMockFactRepo()
	userProf := NewUserProfileService(repo, 200)
	extractor := NewExtractorService(repo, nil, userProf)

	merged := extractor.mergeFacts(explicit, implicit)

	// Should have 3 facts: 2 explicit + 1 implicit (name from explicit takes precedence)
	if len(merged) != 3 {
		t.Errorf("expected 3 merged facts, got %d", len(merged))
	}

	// Check that explicit name fact is preserved
	for _, f := range merged {
		if f.Key == "name" {
			if f.Value != "张三" {
				t.Errorf("expected explicit name '张三' to take precedence, got %q", f.Value)
			}
			if f.Source != domain.SourceExplicit {
				t.Errorf("expected explicit source, got %s", f.Source)
			}
		}
	}
}

func TestDedupAndWrite_SkipExactMatch(t *testing.T) {
	repo := newMockFactRepo()
	// Pre-existing fact with same key
	repo.facts["existing"] = &domain.UserProfileFact{
		FactID:   "existing",
		UserID:   "user1",
		Category: domain.CategoryPersonal,
		Key:      "name",
		Value:    "张三",
		Source:   domain.SourceExplicit,
	}

	userProf := NewUserProfileService(repo, 200)
	extractor := NewExtractorService(repo, nil, userProf)

	facts := []ExtractedFact{
		{Category: domain.CategoryPersonal, Key: "name", Value: "张三", Confidence: 1.0, Source: domain.SourceExplicit},
	}

	result, err := extractor.dedupAndWrite(context.Background(), "user1", facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FactsSkipped != 1 {
		t.Errorf("expected 1 skipped fact, got %d", result.FactsSkipped)
	}
	if result.FactsAdded != 0 {
		t.Errorf("expected 0 added facts, got %d", result.FactsAdded)
	}
}

func TestDedupAndWrite_UpdateExisting(t *testing.T) {
	repo := newMockFactRepo()
	// Pre-existing fact with same key but different value
	repo.facts["existing"] = &domain.UserProfileFact{
		FactID:   "existing",
		UserID:   "user1",
		Category: domain.CategoryPersonal,
		Key:      "name",
		Value:    "张三",
		Source:   domain.SourceExplicit,
	}

	userProf := NewUserProfileService(repo, 200)
	extractor := NewExtractorService(repo, nil, userProf)

	facts := []ExtractedFact{
		{Category: domain.CategoryPersonal, Key: "name", Value: "李四", Confidence: 1.0, Source: domain.SourceExplicit},
	}

	result, err := extractor.dedupAndWrite(context.Background(), "user1", facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FactsUpdated != 1 {
		t.Errorf("expected 1 updated fact, got %d", result.FactsUpdated)
	}
	if result.FactsAdded != 0 {
		t.Errorf("expected 0 added facts, got %d", result.FactsAdded)
	}

	// Verify the value was updated
	updated := repo.facts["existing"]
	if updated.Value != "李四" {
		t.Errorf("expected updated value '李四', got %q", updated.Value)
	}
}

func TestDedupAndWrite_CreateNew(t *testing.T) {
	repo := newMockFactRepo()
	userProf := NewUserProfileService(repo, 200)
	extractor := NewExtractorService(repo, nil, userProf)

	facts := []ExtractedFact{
		{Category: domain.CategoryPersonal, Key: "name", Value: "王五", Confidence: 1.0, Source: domain.SourceExplicit},
		{Category: domain.CategorySkill, Key: "language", Value: "Go", Confidence: 0.9, Source: domain.SourceInferred},
	}

	result, err := extractor.dedupAndWrite(context.Background(), "user1", facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FactsAdded != 2 {
		t.Errorf("expected 2 added facts, got %d", result.FactsAdded)
	}

	// Verify facts were created
	count, _ := repo.CountByUserID(context.Background(), "user1")
	if count != 2 {
		t.Errorf("expected 2 facts in repo, got %d", count)
	}
}

func TestExtract_WithoutLLM(t *testing.T) {
	repo := newMockFactRepo()
	userProf := NewUserProfileService(repo, 200)
	// No LLM client
	extractor := NewExtractorService(repo, nil, userProf)

	req := ExtractRequest{
		Messages: []IngestMessage{
			{Role: "user", Content: "记住我叫张三"},
		},
		UserID: "user1",
	}

	result, err := extractor.Extract(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "complete" {
		t.Errorf("expected status complete, got %s", result.Status)
	}
	if result.FactsAdded != 1 {
		t.Errorf("expected 1 added fact, got %d", result.FactsAdded)
	}

	// Verify the fact was created with correct values
	fact, err := repo.GetByKey(context.Background(), "user1", domain.CategoryPersonal, "name")
	if err != nil {
		t.Fatalf("failed to get created fact: %v", err)
	}
	if fact.Value != "张三" {
		t.Errorf("expected value '张三', got %q", fact.Value)
	}
	if fact.Source != domain.SourceExplicit {
		t.Errorf("expected source explicit, got %s", fact.Source)
	}
}

func TestExtract_WithLLM(t *testing.T) {
	repo := newMockFactRepo()
	userProf := NewUserProfileService(repo, 200)

	// Mock LLM response
	llmClient := &mockLLMClient{
		response: `{"facts": [{"category": "personal", "key": "occupation", "value": "后端工程师", "confidence": 1.0}, {"category": "skill", "key": "programming_language", "value": "Python 和 Go", "confidence": 1.0}]}`,
	}

	extractor := NewExtractorService(repo, llmClient, userProf)

	req := ExtractRequest{
		Messages: []IngestMessage{
			{Role: "user", Content: "我是后端工程师，主要用 Python 和 Go。"},
		},
		UserID: "user1",
	}

	result, err := extractor.Extract(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "complete" {
		t.Errorf("expected status complete, got %s", result.Status)
	}
	if result.FactsAdded != 2 {
		t.Errorf("expected 2 added facts, got %d", result.FactsAdded)
	}
}

func TestExtract_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     ExtractRequest
		errMsg  string
	}{
		{
			name: "Missing user_id",
			req: ExtractRequest{
				Messages: []IngestMessage{{Role: "user", Content: "test"}},
			},
			errMsg: "user_id",
		},
		{
			name: "Missing messages",
			req: ExtractRequest{
				UserID: "user1",
			},
			errMsg: "messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockFactRepo()
			userProf := NewUserProfileService(repo, 200)
			extractor := NewExtractorService(repo, nil, userProf)

			_, err := extractor.Extract(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}

			var valErr *domain.ValidationError
			if errors.As(err, &valErr) {
				if valErr.Field != tt.errMsg {
					t.Errorf("expected field %q, got %q", tt.errMsg, valErr.Field)
				}
			} else {
				t.Errorf("expected ValidationError, got %T", err)
			}
		})
	}
}

func TestNormalizeFactKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Name", "name"},
		{"User Name", "user_name"},
		{"user-name", "user_name"},
		{"User  Name", "user_name"},
		{"  name  ", "name"},
		{"Programming Language", "programming_language"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeFactKey(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeFactKey(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractFromContent(t *testing.T) {
	repo := newMockFactRepo()
	userProf := NewUserProfileService(repo, 200)
	extractor := NewExtractorService(repo, nil, userProf)

	result, err := extractor.ExtractFromContent(context.Background(), "user1", "记住我叫测试用户")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FactsAdded != 1 {
		t.Errorf("expected 1 added fact, got %d", result.FactsAdded)
	}

	fact, err := repo.GetByKey(context.Background(), "user1", domain.CategoryPersonal, "name")
	if err != nil {
		t.Fatalf("failed to get created fact: %v", err)
	}
	if fact.Value != "测试用户" {
		t.Errorf("expected value '测试用户', got %q", fact.Value)
	}
}

func TestExtractFromContent_EmptyContent(t *testing.T) {
	repo := newMockFactRepo()
	userProf := NewUserProfileService(repo, 200)
	extractor := NewExtractorService(repo, nil, userProf)

	_, err := extractor.ExtractFromContent(context.Background(), "user1", "")
	if err == nil {
		t.Fatal("expected validation error for empty content")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

// Test StripMarkdownFences from llm package
func TestStripMarkdownFences(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "```json\n{\"facts\": []}\n```",
			expected: "{\"facts\": []}",
		},
		{
			input:    "```\n{\"facts\": []}\n```",
			expected: "{\"facts\": []}",
		},
		{
			input:    "{\"facts\": []}",
			expected: "{\"facts\": []}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := llm.StripMarkdownFences(tt.input)
			if result != tt.expected {
				t.Errorf("StripMarkdownFences(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
