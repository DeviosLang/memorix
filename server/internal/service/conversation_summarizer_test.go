package service

import (
	"context"
	"testing"

	"github.com/devioslang/memorix/server/internal/domain"
)

// summaryTestRepo is a mock implementation of repository.ConversationSummaryRepo for testing
type summaryTestRepo struct {
	summaries map[string]*domain.ConversationSummary
	byUser    map[string][]*domain.ConversationSummary
	bySession map[string]*domain.ConversationSummary
	count     int
	createErr error
	getErr    error
	deleteErr error
}

func newSummaryTestRepo() *summaryTestRepo {
	return &summaryTestRepo{
		summaries: make(map[string]*domain.ConversationSummary),
		byUser:    make(map[string][]*domain.ConversationSummary),
		bySession: make(map[string]*domain.ConversationSummary),
	}
}

func (m *summaryTestRepo) Create(ctx context.Context, summary *domain.ConversationSummary) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.summaries[summary.SummaryID] = summary
	m.byUser[summary.UserID] = append(m.byUser[summary.UserID], summary)
	if summary.SessionID != "" {
		m.bySession[summary.SessionID] = summary
	}
	m.count++
	return nil
}

func (m *summaryTestRepo) GetByID(ctx context.Context, summaryID string) (*domain.ConversationSummary, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if s, ok := m.summaries[summaryID]; ok {
		return s, nil
	}
	return nil, domain.ErrNotFound
}

func (m *summaryTestRepo) GetByUserID(ctx context.Context, userID string, limit int) ([]domain.ConversationSummary, error) {
	summaries := m.byUser[userID]
	if len(summaries) > limit {
		summaries = summaries[:limit]
	}
	result := make([]domain.ConversationSummary, len(summaries))
	for i, s := range summaries {
		result[i] = *s
	}
	return result, nil
}

func (m *summaryTestRepo) GetBySessionID(ctx context.Context, sessionID string) (*domain.ConversationSummary, error) {
	if s, ok := m.bySession[sessionID]; ok {
		return s, nil
	}
	return nil, domain.ErrNotFound
}

func (m *summaryTestRepo) List(ctx context.Context, f domain.ConversationSummaryFilter) ([]domain.ConversationSummary, int, error) {
	summaries := m.byUser[f.UserID]
	result := make([]domain.ConversationSummary, len(summaries))
	for i, s := range summaries {
		result[i] = *s
	}
	return result, len(summaries), nil
}

func (m *summaryTestRepo) Delete(ctx context.Context, summaryID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if s, ok := m.summaries[summaryID]; ok {
		delete(m.summaries, summaryID)
		m.count--
		userSummaries := m.byUser[s.UserID]
		for i, us := range userSummaries {
			if us.SummaryID == summaryID {
				m.byUser[s.UserID] = append(userSummaries[:i], userSummaries[i+1:]...)
				break
			}
		}
		if s.SessionID != "" {
			delete(m.bySession, s.SessionID)
		}
		return nil
	}
	return domain.ErrNotFound
}

func (m *summaryTestRepo) DeleteOldest(ctx context.Context, userID string, count int) (int64, error) {
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	userSummaries := m.byUser[userID]
	if len(userSummaries) <= count {
		for _, s := range userSummaries {
			delete(m.summaries, s.SummaryID)
			if s.SessionID != "" {
				delete(m.bySession, s.SessionID)
			}
			m.count--
		}
		deleted := int64(len(userSummaries))
		m.byUser[userID] = nil
		return deleted, nil
	}
	for i := 0; i < count; i++ {
		s := userSummaries[i]
		delete(m.summaries, s.SummaryID)
		if s.SessionID != "" {
			delete(m.bySession, s.SessionID)
		}
		m.count--
	}
	m.byUser[userID] = userSummaries[count:]
	return int64(count), nil
}

func (m *summaryTestRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	return len(m.byUser[userID]), nil
}

// summaryTestLLM is a mock LLM client for summarizer tests
type summaryTestLLM struct {
	response string
	err      error
}

func (m *summaryTestLLM) Complete(ctx context.Context, system, user string) (string, error) {
	return m.response, m.err
}

func (m *summaryTestLLM) CompleteJSON(ctx context.Context, system, user string) (string, error) {
	return m.response, m.err
}

func TestSummarize_Success(t *testing.T) {
	repo := newSummaryTestRepo()
	llm := &summaryTestLLM{
		response: `{
			"title": "Go channels discussion",
			"summary": "Discussed Go channels, buffered vs unbuffered differences.",
			"key_topics": ["go", "concurrency", "channels"],
			"user_intent": "Learn Go channel patterns"
		}`,
	}
	svc := NewConversationSummarizerService(repo, llm, 20)

	req := SummarizeRequest{
		UserID:    "user-123",
		SessionID: "session-456",
		Messages: []domain.SummaryMessage{
			{Role: "user", Content: "What are Go channels?"},
			{Role: "assistant", Content: "Go channels are a concurrency primitive..."},
		},
	}

	result, err := svc.Summarize(context.Background(), req)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if result.Status != "complete" {
		t.Errorf("expected status 'complete', got '%s'", result.Status)
	}
	if result.SummaryID == "" {
		t.Error("expected summary_id to be set")
	}
	if result.Skipped {
		t.Error("expected skipped to be false")
	}
	if result.Summary == nil {
		t.Fatal("expected summary to be set")
	}
	if result.Summary.Title != "Go channels discussion" {
		t.Errorf("expected title 'Go channels discussion', got '%s'", result.Summary.Title)
	}
}

func TestSummarize_SkipExisting(t *testing.T) {
	repo := newSummaryTestRepo()
	existing := &domain.ConversationSummary{
		SummaryID:  "existing-id",
		UserID:     "user-123",
		SessionID:  "session-456",
		Title:      "Existing summary",
		Summary:    "This already exists",
		KeyTopics:  []string{"existing"},
		UserIntent: "Already summarized",
	}
	repo.Create(context.Background(), existing)

	llm := &summaryTestLLM{
		response: `{"title": "New", "summary": "New summary", "key_topics": [], "user_intent": "New"}`,
	}
	svc := NewConversationSummarizerService(repo, llm, 20)

	req := SummarizeRequest{
		UserID:    "user-123",
		SessionID: "session-456",
		Messages: []domain.SummaryMessage{
			{Role: "user", Content: "New message"},
		},
	}

	result, err := svc.Summarize(context.Background(), req)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if !result.Skipped {
		t.Error("expected skipped to be true")
	}
	if result.SummaryID != "existing-id" {
		t.Errorf("expected existing summary_id, got '%s'", result.SummaryID)
	}
	if repo.count != 1 {
		t.Errorf("expected 1 summary, got %d", repo.count)
	}
}

func TestSummarize_ValidationError(t *testing.T) {
	repo := newSummaryTestRepo()
	llm := &summaryTestLLM{}
	svc := NewConversationSummarizerService(repo, llm, 20)

	// Missing user_id
	req := SummarizeRequest{
		SessionID: "session-456",
		Messages: []domain.SummaryMessage{
			{Role: "user", Content: "Test"},
		},
	}

	_, err := svc.Summarize(context.Background(), req)
	if err == nil {
		t.Error("expected error for missing user_id")
	}

	// Empty messages
	req2 := SummarizeRequest{
		UserID: "user-123",
	}
	_, err = svc.Summarize(context.Background(), req2)
	if err == nil {
		t.Error("expected error for missing messages")
	}
}

func TestSummarize_SlidingWindow(t *testing.T) {
	repo := newSummaryTestRepo()
	llm := &summaryTestLLM{
		response: `{
			"title": "Test",
			"summary": "Test summary",
			"key_topics": ["test"],
			"user_intent": "Testing"
		}`,
	}
	svc := NewConversationSummarizerService(repo, llm, 3) // Small limit for testing

	// Create 3 summaries (at capacity)
	for i := 0; i < 3; i++ {
		req := SummarizeRequest{
			UserID:    "user-123",
			SessionID: string(rune('a' + i)),
			Messages: []domain.SummaryMessage{
				{Role: "user", Content: "Test"},
			},
		}
		_, err := svc.Summarize(context.Background(), req)
		if err != nil {
			t.Fatalf("Summarize failed: %v", err)
		}
	}

	// Now create one more - should trigger deletion of oldest
	req := SummarizeRequest{
		UserID:    "user-123",
		SessionID: "new-session",
		Messages: []domain.SummaryMessage{
			{Role: "user", Content: "Test"},
		},
	}
	result, err := svc.Summarize(context.Background(), req)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if result.Status != "complete" {
		t.Errorf("expected status 'complete', got '%s'", result.Status)
	}

	// Should still have 3 summaries (sliding window enforced)
	count, _ := repo.CountByUserID(context.Background(), "user-123")
	if count != 3 {
		t.Errorf("expected 3 summaries, got %d", count)
	}
}

func TestSummarize_NoLLM(t *testing.T) {
	repo := newSummaryTestRepo()
	svc := NewConversationSummarizerService(repo, nil, 20) // No LLM client

	req := SummarizeRequest{
		UserID:    "user-123",
		SessionID: "session-456",
		Messages: []domain.SummaryMessage{
			{Role: "user", Content: "Test"},
		},
	}

	result, err := svc.Summarize(context.Background(), req)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", result.Status)
	}
	if result.Error != "LLM client not configured" {
		t.Errorf("expected LLM error, got '%s'", result.Error)
	}
}

func TestGetSummary(t *testing.T) {
	repo := newSummaryTestRepo()
	llm := &summaryTestLLM{}
	svc := NewConversationSummarizerService(repo, llm, 20)

	summary := &domain.ConversationSummary{
		SummaryID:  "test-id",
		UserID:     "user-123",
		Title:      "Test",
		Summary:    "Test summary",
		KeyTopics:  []string{"test"},
		UserIntent: "Testing",
	}
	repo.Create(context.Background(), summary)

	result, err := svc.GetSummary(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if result.SummaryID != "test-id" {
		t.Errorf("expected summary_id 'test-id', got '%s'", result.SummaryID)
	}
}

func TestGetSummariesByUser(t *testing.T) {
	repo := newSummaryTestRepo()
	llm := &summaryTestLLM{}
	svc := NewConversationSummarizerService(repo, llm, 20)

	for i := 0; i < 5; i++ {
		summary := &domain.ConversationSummary{
			SummaryID:  string(rune('a' + i)),
			UserID:     "user-123",
			Title:      "Test",
			Summary:    "Test summary",
			KeyTopics:  []string{"test"},
			UserIntent: "Testing",
		}
		repo.Create(context.Background(), summary)
	}

	result, err := svc.GetSummariesByUser(context.Background(), "user-123", 3)
	if err != nil {
		t.Fatalf("GetSummariesByUser failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 summaries, got %d", len(result))
	}
}

func TestDeleteSummary(t *testing.T) {
	repo := newSummaryTestRepo()
	llm := &summaryTestLLM{}
	svc := NewConversationSummarizerService(repo, llm, 20)

	summary := &domain.ConversationSummary{
		SummaryID:  "test-id",
		UserID:     "user-123",
		Title:      "Test",
		Summary:    "Test summary",
		KeyTopics:  []string{"test"},
		UserIntent: "Testing",
	}
	repo.Create(context.Background(), summary)

	err := svc.DeleteSummary(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("DeleteSummary failed: %v", err)
	}

	_, err = svc.GetSummary(context.Background(), "test-id")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestSummarizerHasLLM(t *testing.T) {
	repo := newSummaryTestRepo()

	svc := NewConversationSummarizerService(repo, &summaryTestLLM{}, 20)
	if !svc.HasLLM() {
		t.Error("expected HasLLM to return true")
	}

	svc2 := NewConversationSummarizerService(repo, nil, 20)
	if svc2.HasLLM() {
		t.Error("expected HasLLM to return false")
	}
}
