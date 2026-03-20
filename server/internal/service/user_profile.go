package service

import (
	"context"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/google/uuid"
)

// UserProfileService manages user profile facts.
type UserProfileService struct {
	facts           repository.UserProfileFactRepo
	maxFactsPerUser int
}

// NewUserProfileService creates a new UserProfileService.
func NewUserProfileService(facts repository.UserProfileFactRepo, maxFactsPerUser int) *UserProfileService {
	if maxFactsPerUser <= 0 {
		maxFactsPerUser = domain.DefaultMaxFactsPerUser
	}
	return &UserProfileService{
		facts:           facts,
		maxFactsPerUser: maxFactsPerUser,
	}
}

// CreateFactInput contains the input for creating a new fact.
type CreateFactInput struct {
	UserID     string              `json:"user_id"`
	Category   domain.FactCategory `json:"category"`
	Key        string              `json:"key"`
	Value      string              `json:"value"`
	Source     domain.FactSource   `json:"source"`
	Confidence float64             `json:"confidence"`
}

// UpdateFactInput contains the input for updating a fact.
type UpdateFactInput struct {
	Category   *domain.FactCategory `json:"category,omitempty"`
	Key        *string              `json:"key,omitempty"`
	Value      *string              `json:"value,omitempty"`
	Source     *domain.FactSource   `json:"source,omitempty"`
	Confidence *float64             `json:"confidence,omitempty"`
}

// CreateFact creates a new user profile fact.
// It enforces the capacity limit by cleaning up old low-confidence facts if needed.
func (s *UserProfileService) CreateFact(ctx context.Context, input CreateFactInput) (*domain.UserProfileFact, error) {
	if err := validateCreateFactInput(input); err != nil {
		return nil, err
	}

	// Check capacity and cleanup if needed
	count, err := s.facts.CountByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	if count >= s.maxFactsPerUser {
		// Need to make room - delete oldest low-confidence facts
		// Delete one fact to make room for the new one
		deleted, err := s.facts.DeleteOldestLowConfidence(ctx, input.UserID, 1)
		if err != nil {
			return nil, err
		}
		if deleted == 0 {
			// No facts were deleted - this shouldn't happen, but handle gracefully
			// by allowing the insert anyway (will exceed limit slightly)
		}
	}

	now := time.Now()
	fact := &domain.UserProfileFact{
		FactID:         uuid.New().String(),
		UserID:         input.UserID,
		Category:       input.Category,
		Key:            input.Key,
		Value:          input.Value,
		Source:         input.Source,
		Confidence:     input.Confidence,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
	}

	if err := s.facts.Create(ctx, fact); err != nil {
		return nil, err
	}

	return fact, nil
}

// GetFact retrieves a single fact by ID.
func (s *UserProfileService) GetFact(ctx context.Context, factID string) (*domain.UserProfileFact, error) {
	fact, err := s.facts.GetByID(ctx, factID)
	if err != nil {
		return nil, err
	}

	// Update last accessed timestamp
	_ = s.facts.TouchLastAccessed(ctx, factID)

	return fact, nil
}

// GetFactsByUser retrieves all facts for a user.
func (s *UserProfileService) GetFactsByUser(ctx context.Context, userID string) ([]domain.UserProfileFact, error) {
	return s.facts.GetByUserID(ctx, userID)
}

// GetFactsByCategory retrieves facts for a user filtered by category.
func (s *UserProfileService) GetFactsByCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error) {
	if !domain.IsValidCategory(category) {
		return nil, &domain.ValidationError{Field: "category", Message: "invalid category"}
	}
	return s.facts.GetByUserIDAndCategory(ctx, userID, category)
}

// ListFacts retrieves facts with filtering and pagination.
func (s *UserProfileService) ListFacts(ctx context.Context, filter domain.UserProfileFactFilter) ([]domain.UserProfileFact, int, error) {
	if filter.UserID == "" {
		return nil, 0, &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if filter.Category != "" && !domain.IsValidCategory(filter.Category) {
		return nil, 0, &domain.ValidationError{Field: "category", Message: "invalid category"}
	}
	if filter.Source != "" && !domain.IsValidSource(filter.Source) {
		return nil, 0, &domain.ValidationError{Field: "source", Message: "invalid source"}
	}
	return s.facts.List(ctx, filter)
}

// UpdateFact updates an existing fact.
func (s *UserProfileService) UpdateFact(ctx context.Context, factID string, input UpdateFactInput) (*domain.UserProfileFact, error) {
	// Get existing fact
	fact, err := s.facts.GetByID(ctx, factID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if input.Category != nil {
		if !domain.IsValidCategory(*input.Category) {
			return nil, &domain.ValidationError{Field: "category", Message: "invalid category"}
		}
		fact.Category = *input.Category
	}
	if input.Key != nil {
		if *input.Key == "" {
			return nil, &domain.ValidationError{Field: "key", Message: "required"}
		}
		if len(*input.Key) > 100 {
			return nil, &domain.ValidationError{Field: "key", Message: "too long (max 100)"}
		}
		fact.Key = *input.Key
	}
	if input.Value != nil {
		if *input.Value == "" {
			return nil, &domain.ValidationError{Field: "value", Message: "required"}
		}
		if len(*input.Value) > 10000 {
			return nil, &domain.ValidationError{Field: "value", Message: "too long (max 10000)"}
		}
		fact.Value = *input.Value
	}
	if input.Source != nil {
		if !domain.IsValidSource(*input.Source) {
			return nil, &domain.ValidationError{Field: "source", Message: "invalid source"}
		}
		fact.Source = *input.Source
	}
	if input.Confidence != nil {
		if *input.Confidence < domain.MinConfidence || *input.Confidence > domain.MaxConfidence {
			return nil, &domain.ValidationError{Field: "confidence", Message: "must be between 0 and 1"}
		}
		fact.Confidence = *input.Confidence
	}

	fact.UpdatedAt = time.Now()

	if err := s.facts.Update(ctx, fact); err != nil {
		return nil, err
	}

	return fact, nil
}

// DeleteFact deletes a fact by ID.
func (s *UserProfileService) DeleteFact(ctx context.Context, factID string) error {
	return s.facts.Delete(ctx, factID)
}

// DeleteFactsByUser deletes all facts for a user.
func (s *UserProfileService) DeleteFactsByUser(ctx context.Context, userID string) error {
	return s.facts.DeleteByUserID(ctx, userID)
}

// CountFactsByUser returns the number of facts for a user.
func (s *UserProfileService) CountFactsByUser(ctx context.Context, userID string) (int, error) {
	return s.facts.CountByUserID(ctx, userID)
}

func validateCreateFactInput(input CreateFactInput) error {
	if input.UserID == "" {
		return &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if !domain.IsValidCategory(input.Category) {
		return &domain.ValidationError{Field: "category", Message: "invalid category"}
	}
	if input.Key == "" {
		return &domain.ValidationError{Field: "key", Message: "required"}
	}
	if len(input.Key) > 100 {
		return &domain.ValidationError{Field: "key", Message: "too long (max 100)"}
	}
	if input.Value == "" {
		return &domain.ValidationError{Field: "value", Message: "required"}
	}
	if len(input.Value) > 10000 {
		return &domain.ValidationError{Field: "value", Message: "too long (max 10000)"}
	}
	if !domain.IsValidSource(input.Source) {
		return &domain.ValidationError{Field: "source", Message: "invalid source"}
	}
	if input.Confidence < domain.MinConfidence || input.Confidence > domain.MaxConfidence {
		return &domain.ValidationError{Field: "confidence", Message: "must be between 0 and 1"}
	}
	return nil
}
