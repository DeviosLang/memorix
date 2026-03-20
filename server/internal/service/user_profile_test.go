package service

import (
	"context"
	"errors"
	"testing"

	"github.com/devioslang/memorix/server/internal/domain"
)

// mockUserProfileFactRepo is a mock implementation of repository.UserProfileFactRepo
type mockUserProfileFactRepo struct {
	facts       map[string]*domain.UserProfileFact
	countResult int
	countErr    error
}

func newMockUserProfileFactRepo() *mockUserProfileFactRepo {
	return &mockUserProfileFactRepo{
		facts: make(map[string]*domain.UserProfileFact),
	}
}

func (m *mockUserProfileFactRepo) Create(ctx context.Context, fact *domain.UserProfileFact) error {
	m.facts[fact.FactID] = fact
	return nil
}

func (m *mockUserProfileFactRepo) GetByID(ctx context.Context, factID string) (*domain.UserProfileFact, error) {
	fact, ok := m.facts[factID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return fact, nil
}

func (m *mockUserProfileFactRepo) GetByUserID(ctx context.Context, userID string) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.facts {
		if f.UserID == userID {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockUserProfileFactRepo) GetByUserIDAndCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.facts {
		if f.UserID == userID && f.Category == category {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockUserProfileFactRepo) List(ctx context.Context, f domain.UserProfileFactFilter) ([]domain.UserProfileFact, int, error) {
	var result []domain.UserProfileFact
	for _, fact := range m.facts {
		if f.UserID != "" && fact.UserID != f.UserID {
			continue
		}
		if f.Category != "" && fact.Category != f.Category {
			continue
		}
		if f.Key != "" && fact.Key != f.Key {
			continue
		}
		if f.Source != "" && fact.Source != f.Source {
			continue
		}
		result = append(result, *fact)
	}
	total := len(result)
	// Simple pagination
	start := f.Offset
	if start > total {
		start = total
	}
	end := start + f.Limit
	if end > total || f.Limit <= 0 {
		end = total
	}
	return result[start:end], total, nil
}

func (m *mockUserProfileFactRepo) Update(ctx context.Context, fact *domain.UserProfileFact) error {
	if _, ok := m.facts[fact.FactID]; !ok {
		return domain.ErrNotFound
	}
	m.facts[fact.FactID] = fact
	return nil
}

func (m *mockUserProfileFactRepo) Delete(ctx context.Context, factID string) error {
	if _, ok := m.facts[factID]; !ok {
		return domain.ErrNotFound
	}
	delete(m.facts, factID)
	return nil
}

func (m *mockUserProfileFactRepo) DeleteByUserID(ctx context.Context, userID string) error {
	for id, f := range m.facts {
		if f.UserID == userID {
			delete(m.facts, id)
		}
	}
	return nil
}

func (m *mockUserProfileFactRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	if m.countResult > 0 {
		return m.countResult, nil
	}
	count := 0
	for _, f := range m.facts {
		if f.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockUserProfileFactRepo) DeleteOldestLowConfidence(ctx context.Context, userID string, count int) (int64, error) {
	deleted := int64(0)
	// Simple implementation: just delete the first 'count' facts for this user
	for id, f := range m.facts {
		if f.UserID == userID && deleted < int64(count) {
			delete(m.facts, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *mockUserProfileFactRepo) TouchLastAccessed(ctx context.Context, factID string) error {
	return nil
}

func TestValidateCreateFactInput(t *testing.T) {
	tests := []struct {
		name        string
		input       CreateFactInput
		wantErr     bool
		wantField   string
		wantMessage string
	}{
		{
			name: "valid input with explicit source",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr: false,
		},
		{
			name: "valid input with inferred source",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPreference,
				Key:        "language",
				Value:      "Python",
				Source:     domain.SourceInferred,
				Confidence: 0.85,
			},
			wantErr: false,
		},
		{
			name: "missing user_id",
			input: CreateFactInput{
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "user_id",
			wantMessage: "required",
		},
		{
			name: "invalid category",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   "invalid",
				Key:        "name",
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "category",
			wantMessage: "invalid category",
		},
		{
			name: "missing key",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "key",
			wantMessage: "required",
		},
		{
			name: "missing value",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "value",
			wantMessage: "required",
		},
		{
			name: "invalid source",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "John Doe",
				Source:     "invalid",
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "source",
			wantMessage: "invalid source",
		},
		{
			name: "confidence below 0",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: -0.1,
			},
			wantErr:     true,
			wantField:   "confidence",
			wantMessage: "must be between 0 and 1",
		},
		{
			name: "confidence above 1",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.5,
			},
			wantErr:     true,
			wantField:   "confidence",
			wantMessage: "must be between 0 and 1",
		},
		{
			name: "key too long",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        string(make([]byte, 101)),
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "key",
			wantMessage: "too long (max 100)",
		},
		{
			name: "value too long",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      string(make([]byte, 10001)),
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			wantErr:     true,
			wantField:   "value",
			wantMessage: "too long (max 10000)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateFactInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateCreateFactInput() expected error, got nil")
					return
				}
				var ve *domain.ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("validateCreateFactInput() expected ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.wantField {
					t.Errorf("validateCreateFactInput() field = %q, want %q", ve.Field, tt.wantField)
				}
				if ve.Message != tt.wantMessage {
					t.Errorf("validateCreateFactInput() message = %q, want %q", ve.Message, tt.wantMessage)
				}
			} else {
				if err != nil {
					t.Errorf("validateCreateFactInput() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestUserProfileService_CreateFact(t *testing.T) {
	tests := []struct {
		name          string
		input         CreateFactInput
		existingCount int
		wantErr       bool
	}{
		{
			name: "create fact successfully",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "name",
				Value:      "John Doe",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			existingCount: 0,
			wantErr:       false,
		},
		{
			name: "create fact when at capacity triggers cleanup",
			input: CreateFactInput{
				UserID:     "user-123",
				Category:   domain.CategoryPersonal,
				Key:        "new_fact",
				Value:      "New Value",
				Source:     domain.SourceExplicit,
				Confidence: 1.0,
			},
			existingCount: 200,
			wantErr:       false,
		},
		{
			name: "invalid input returns error",
			input: CreateFactInput{
				UserID: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockUserProfileFactRepo()
			mockRepo.countResult = tt.existingCount
			svc := NewUserProfileService(mockRepo, 200)

			fact, err := svc.CreateFact(context.Background(), tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateFact() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("CreateFact() unexpected error: %v", err)
				return
			}
			if fact == nil {
				t.Errorf("CreateFact() returned nil fact")
				return
			}
			if fact.FactID == "" {
				t.Errorf("CreateFact() returned fact with empty ID")
			}
			if fact.UserID != tt.input.UserID {
				t.Errorf("CreateFact() UserID = %q, want %q", fact.UserID, tt.input.UserID)
			}
			if fact.Category != tt.input.Category {
				t.Errorf("CreateFact() Category = %q, want %q", fact.Category, tt.input.Category)
			}
		})
	}
}

func TestUserProfileService_GetFact(t *testing.T) {
	mockRepo := newMockUserProfileFactRepo()
	svc := NewUserProfileService(mockRepo, 200)

	// Create a fact first
	created, _ := svc.CreateFact(context.Background(), CreateFactInput{
		UserID:     "user-123",
		Category:   domain.CategoryPersonal,
		Key:        "name",
		Value:      "John Doe",
		Source:     domain.SourceExplicit,
		Confidence: 1.0,
	})

	// Test retrieval
	fact, err := svc.GetFact(context.Background(), created.FactID)
	if err != nil {
		t.Errorf("GetFact() unexpected error: %v", err)
		return
	}
	if fact.FactID != created.FactID {
		t.Errorf("GetFact() FactID = %q, want %q", fact.FactID, created.FactID)
	}

	// Test not found
	_, err = svc.GetFact(context.Background(), "non-existent")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetFact() expected ErrNotFound, got %v", err)
	}
}

func TestUserProfileService_GetFactsByCategory(t *testing.T) {
	mockRepo := newMockUserProfileFactRepo()
	svc := NewUserProfileService(mockRepo, 200)

	// Create facts in different categories
	svc.CreateFact(context.Background(), CreateFactInput{
		UserID:     "user-123",
		Category:   domain.CategoryPersonal,
		Key:        "name",
		Value:      "John Doe",
		Source:     domain.SourceExplicit,
		Confidence: 1.0,
	})
	svc.CreateFact(context.Background(), CreateFactInput{
		UserID:     "user-123",
		Category:   domain.CategoryPreference,
		Key:        "language",
		Value:      "Python",
		Source:     domain.SourceExplicit,
		Confidence: 1.0,
	})

	// Test category filter
	facts, err := svc.GetFactsByCategory(context.Background(), "user-123", domain.CategoryPersonal)
	if err != nil {
		t.Errorf("GetFactsByCategory() unexpected error: %v", err)
		return
	}
	if len(facts) != 1 {
		t.Errorf("GetFactsByCategory() returned %d facts, want 1", len(facts))
		return
	}
	if facts[0].Category != domain.CategoryPersonal {
		t.Errorf("GetFactsByCategory() Category = %q, want %q", facts[0].Category, domain.CategoryPersonal)
	}

	// Test invalid category
	_, err = svc.GetFactsByCategory(context.Background(), "user-123", "invalid")
	if err == nil {
		t.Errorf("GetFactsByCategory() expected error for invalid category")
	}
}

func TestUserProfileService_UpdateFact(t *testing.T) {
	mockRepo := newMockUserProfileFactRepo()
	svc := NewUserProfileService(mockRepo, 200)

	// Create a fact first
	created, _ := svc.CreateFact(context.Background(), CreateFactInput{
		UserID:     "user-123",
		Category:   domain.CategoryPersonal,
		Key:        "name",
		Value:      "John Doe",
		Source:     domain.SourceExplicit,
		Confidence: 1.0,
	})

	// Test update
	newValue := "Jane Doe"
	updated, err := svc.UpdateFact(context.Background(), created.FactID, UpdateFactInput{
		Value: &newValue,
	})
	if err != nil {
		t.Errorf("UpdateFact() unexpected error: %v", err)
		return
	}
	if updated.Value != newValue {
		t.Errorf("UpdateFact() Value = %q, want %q", updated.Value, newValue)
	}

	// Test update non-existent fact
	_, err = svc.UpdateFact(context.Background(), "non-existent", UpdateFactInput{
		Value: &newValue,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("UpdateFact() expected ErrNotFound, got %v", err)
	}
}

func TestUserProfileService_DeleteFact(t *testing.T) {
	mockRepo := newMockUserProfileFactRepo()
	svc := NewUserProfileService(mockRepo, 200)

	// Create a fact first
	created, _ := svc.CreateFact(context.Background(), CreateFactInput{
		UserID:     "user-123",
		Category:   domain.CategoryPersonal,
		Key:        "name",
		Value:      "John Doe",
		Source:     domain.SourceExplicit,
		Confidence: 1.0,
	})

	// Test delete
	err := svc.DeleteFact(context.Background(), created.FactID)
	if err != nil {
		t.Errorf("DeleteFact() unexpected error: %v", err)
		return
	}

	// Verify deleted
	_, err = svc.GetFact(context.Background(), created.FactID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetFact() expected ErrNotFound after delete, got %v", err)
	}

	// Test delete non-existent fact
	err = svc.DeleteFact(context.Background(), "non-existent")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("DeleteFact() expected ErrNotFound, got %v", err)
	}
}

func TestUserProfileService_ListFacts(t *testing.T) {
	mockRepo := newMockUserProfileFactRepo()
	svc := NewUserProfileService(mockRepo, 200)

	// Create multiple facts
	for i := 0; i < 5; i++ {
		svc.CreateFact(context.Background(), CreateFactInput{
			UserID:     "user-123",
			Category:   domain.CategoryPersonal,
			Key:        "key" + string(rune('a'+i)),
			Value:      "value",
			Source:     domain.SourceExplicit,
			Confidence: 1.0,
		})
	}

	// Test list
	facts, total, err := svc.ListFacts(context.Background(), domain.UserProfileFactFilter{
		UserID: "user-123",
		Limit:  10,
	})
	if err != nil {
		t.Errorf("ListFacts() unexpected error: %v", err)
		return
	}
	if total != 5 {
		t.Errorf("ListFacts() total = %d, want 5", total)
	}
	if len(facts) != 5 {
		t.Errorf("ListFacts() returned %d facts, want 5", len(facts))
	}

	// Test missing user_id
	_, _, err = svc.ListFacts(context.Background(), domain.UserProfileFactFilter{})
	if err == nil {
		t.Errorf("ListFacts() expected error for missing user_id")
	}
}

func TestUserProfileService_DefaultMaxFacts(t *testing.T) {
	mockRepo := newMockUserProfileFactRepo()
	// Test with default max facts
	svc := NewUserProfileService(mockRepo, 0) // 0 should use default
	if svc.maxFactsPerUser != domain.DefaultMaxFactsPerUser {
		t.Errorf("NewUserProfileService() maxFactsPerUser = %d, want %d", svc.maxFactsPerUser, domain.DefaultMaxFactsPerUser)
	}
}
