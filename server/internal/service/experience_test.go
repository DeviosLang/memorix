package service

import (
	"context"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/vectorstore"
)

// MockVectorStore implements vectorstore.VectorStore for testing.
type MockVectorStore struct {
	experiences map[string]*domain.Experience // keyed by experienceID
	userIndex   map[string][]string           // userID -> []experienceID
	writeCount  int
	searchCalls int
	lastLatency int64
}

func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{
		experiences: make(map[string]*domain.Experience),
		userIndex:   make(map[string][]string),
		lastLatency: 15, // Simulate 15ms latency
	}
}

func (m *MockVectorStore) Write(ctx context.Context, exp *domain.Experience) error {
	m.experiences[exp.ExperienceID] = exp
	m.userIndex[exp.UserID] = append(m.userIndex[exp.UserID], exp.ExperienceID)
	m.writeCount++
	return nil
}

func (m *MockVectorStore) WriteBatch(ctx context.Context, experiences []*domain.Experience) (int, error) {
	for _, exp := range experiences {
		m.experiences[exp.ExperienceID] = exp
		m.userIndex[exp.UserID] = append(m.userIndex[exp.UserID], exp.ExperienceID)
		m.writeCount++
	}
	return len(experiences), nil
}

func (m *MockVectorStore) Search(ctx context.Context, userID string, queryVec []float32, filter domain.ExperienceFilter) (*domain.ExperienceSearchResult, error) {
	m.searchCalls++
	var results []domain.Experience
	for _, id := range m.userIndex[userID] {
		if exp, ok := m.experiences[id]; ok {
			// Apply filters
			if filter.Topic != "" && exp.Metadata.Topic != filter.Topic {
				continue
			}
			if filter.Outcome != "" && exp.Metadata.Outcome != filter.Outcome {
				continue
			}
			// Assign mock score
			score := 0.9
			exp.Score = &score
			results = append(results, *exp)
		}
	}
	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	return &domain.ExperienceSearchResult{
		Experiences: results,
		Total:       len(results),
		LatencyMs:   m.lastLatency,
	}, nil
}

func (m *MockVectorStore) GetByID(ctx context.Context, userID, experienceID string) (*domain.Experience, error) {
	exp, ok := m.experiences[experienceID]
	if !ok || exp.UserID != userID {
		return nil, vectorstore.ErrNotFound
	}
	return exp, nil
}

func (m *MockVectorStore) Delete(ctx context.Context, userID, experienceID string) error {
	exp, ok := m.experiences[experienceID]
	if !ok || exp.UserID != userID {
		return vectorstore.ErrNotFound
	}
	delete(m.experiences, experienceID)
	// Remove from user index
	var newIDs []string
	for _, id := range m.userIndex[userID] {
		if id != experienceID {
			newIDs = append(newIDs, id)
		}
	}
	m.userIndex[userID] = newIDs
	return nil
}

func (m *MockVectorStore) DeleteByUser(ctx context.Context, userID string) error {
	for _, id := range m.userIndex[userID] {
		delete(m.experiences, id)
	}
	delete(m.userIndex, userID)
	return nil
}

func (m *MockVectorStore) Stats(ctx context.Context, userID string) (*domain.ExperienceStats, error) {
	stats := &domain.ExperienceStats{
		UserID:          userID,
		TotalExperiences: len(m.userIndex[userID]),
		ByTopic:         make(map[string]int),
		ByOutcome:       make(map[domain.ExperienceOutcome]int),
	}
	for _, id := range m.userIndex[userID] {
		exp := m.experiences[id]
		if exp.Metadata.Topic != "" {
			stats.ByTopic[exp.Metadata.Topic]++
		}
		stats.ByOutcome[exp.Metadata.Outcome]++
	}
	return stats, nil
}

func (m *MockVectorStore) Health(ctx context.Context) error {
	return nil
}

func (m *MockVectorStore) Close() error {
	return nil
}

// MockEmbedder implements embed.Embedder for testing.
type MockEmbedder struct {
	dims int
}

func NewMockEmbedder(dims int) *MockEmbedder {
	return &MockEmbedder{dims: dims}
}

func (m *MockEmbedder) Dims() int {
	return m.dims
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return a deterministic embedding based on text length
	vec := make([]float32, m.dims)
	for i := range vec {
		vec[i] = float32(len(text)%100) / 100.0
	}
	return vec, nil
}
