package domain

import (
	"time"
)

// FactCategory represents the category of a user profile fact.
type FactCategory string

const (
	CategoryPersonal   FactCategory = "personal"
	CategoryPreference FactCategory = "preference"
	CategoryGoal       FactCategory = "goal"
	CategorySkill      FactCategory = "skill"
)

// FactSource represents how a fact was obtained.
type FactSource string

const (
	SourceExplicit FactSource = "explicit" // User explicitly provided
	SourceInferred FactSource = "inferred" // Model inferred from conversation
)

// UserProfileFact represents a structured long-term fact about a user.
// Based on ChatGPT's third-layer "user memory" design from reverse engineering analysis.
type UserProfileFact struct {
	FactID     string       `json:"fact_id"`
	UserID     string       `json:"user_id"`
	Category   FactCategory `json:"category"`
	Key        string       `json:"key"`
	Value      string       `json:"value"`
	Source     FactSource   `json:"source"`
	Confidence float64      `json:"confidence"` // 0-1, confidence level for inferred facts

	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

// UserProfileFactFilter encapsulates query parameters for fact queries.
type UserProfileFactFilter struct {
	UserID   string
	Category FactCategory
	Key      string
	Source   FactSource
	Limit    int
	Offset   int
}

// ValidFactCategories returns all valid fact categories.
func ValidFactCategories() []FactCategory {
	return []FactCategory{
		CategoryPersonal,
		CategoryPreference,
		CategoryGoal,
		CategorySkill,
	}
}

// IsValidCategory checks if a category is valid.
func IsValidCategory(c FactCategory) bool {
	for _, valid := range ValidFactCategories() {
		if c == valid {
			return true
		}
	}
	return false
}

// ValidFactSources returns all valid fact sources.
func ValidFactSources() []FactSource {
	return []FactSource{
		SourceExplicit,
		SourceInferred,
	}
}

// IsValidSource checks if a source is valid.
func IsValidSource(s FactSource) bool {
	for _, valid := range ValidFactSources() {
		if s == valid {
			return true
		}
	}
	return false
}

// Default capacity limits
const (
	DefaultMaxFactsPerUser = 200
	MinConfidence          = 0.0
	MaxConfidence          = 1.0
)
