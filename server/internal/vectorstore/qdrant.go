package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// QdrantStore implements VectorStore using Qdrant vector database.
// Qdrant is recommended for local deployment due to:
// - Native Go client support
// - High performance (Rust-based)
// - Rich filtering capabilities
// - Persistent storage with WAL
//
// Architecture:
// - Single collection "experiences" with user_id as payload field
// - Filtering by user_id for data isolation
// - Cosine distance for semantic similarity
type QdrantStore struct {
	baseURL     string
	apiKey      string
	collection  string
	dimensions  int
	httpClient  *http.Client
	timeout     time.Duration
}

// QdrantConfig holds Qdrant-specific configuration.
type QdrantConfig struct {
	// URL to Qdrant server (e.g., "http://localhost:6333")
	URL string

	// API key for authentication (optional for local deployment)
	APIKey string

	// Collection name (default: "experiences")
	Collection string

	// Embedding dimensions (default: 1536)
	Dimensions int

	// Timeout for operations (default: 30s)
	Timeout time.Duration
}

// NewQdrantStore creates a new Qdrant vector store.
func NewQdrantStore(cfg QdrantConfig) (*QdrantStore, error) {
	if cfg.URL == "" {
		cfg.URL = "http://localhost:6333"
	}
	if cfg.Collection == "" {
		cfg.Collection = "experiences"
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = 1536
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	store := &QdrantStore{
		baseURL:    strings.TrimSuffix(cfg.URL, "/"),
		apiKey:     cfg.APIKey,
		collection: cfg.Collection,
		dimensions: cfg.Dimensions,
		timeout:    cfg.Timeout,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}

	// Create collection if not exists
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := store.ensureCollection(ctx); err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	return store, nil
}

// ensureCollection creates the collection if it doesn't exist.
func (s *QdrantStore) ensureCollection(ctx context.Context) error {
	// Check if collection exists
	resp, err := s.doRequest(ctx, "GET", "/collections/"+s.collection, nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		slog.Info("qdrant collection already exists", "collection", s.collection)
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Create collection
	createReq := map[string]any{
		"vectors": map[string]any{
			"size":     s.dimensions,
			"distance": "Cosine",
		},
	}

	resp, err = s.doRequest(ctx, "PUT", "/collections/"+s.collection, createReq)
	if err != nil {
		return fmt.Errorf("create collection request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection failed: %s", string(body))
	}

	slog.Info("qdrant collection created", "collection", s.collection, "dimensions", s.dimensions)
	return nil
}

// doRequest performs an HTTP request to the Qdrant API.
func (s *QdrantStore) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	return s.httpClient.Do(req)
}

// qdrantPoint represents a Qdrant point (vector + payload).
type qdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

// qdrantPayload converts an Experience to Qdrant payload.
func qdrantPayload(exp *domain.Experience) map[string]any {
	return map[string]any{
		"user_id":     exp.UserID,
		"content":     exp.Content,
		"session_id":  exp.Metadata.SessionID,
		"topic":       exp.Metadata.Topic,
		"outcome":     string(exp.Metadata.Outcome),
		"confidence":  exp.Metadata.Confidence,
		"tags":        exp.Metadata.Tags,
		"source":      exp.Metadata.Source,
		"extra":       string(exp.Metadata.Extra),
		"created_at":  exp.CreatedAt.Unix(),
		"updated_at":  exp.UpdatedAt.Unix(),
	}
}

// payloadToExperience converts Qdrant payload back to Experience.
func payloadToExperience(id string, payload map[string]any, score float64) (*domain.Experience, error) {
	createdAt := time.Unix(int64(payload["created_at"].(float64)), 0)
	updatedAt := time.Unix(int64(payload["updated_at"].(float64)), 0)

	var tags []string
	if t, ok := payload["tags"].([]any); ok {
		for _, tag := range t {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	var extra json.RawMessage
	if e, ok := payload["extra"].(string); ok && e != "" && e != "null" {
		extra = json.RawMessage(e)
	}

	outcome := domain.OutcomeNeutral
	if o, ok := payload["outcome"].(string); ok {
		outcome = domain.ExperienceOutcome(o)
	}

	confidence := 1.0
	if c, ok := payload["confidence"].(float64); ok {
		confidence = c
	}

	return &domain.Experience{
		ExperienceID: id,
		UserID:       payload["user_id"].(string),
		Content:      payload["content"].(string),
		Metadata: domain.ExperienceMetadata{
			SessionID:  getString(payload, "session_id"),
			Topic:      getString(payload, "topic"),
			Outcome:    outcome,
			Confidence: confidence,
			Tags:       tags,
			Source:     getString(payload, "source"),
			Extra:      extra,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Score:     &score,
	}, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Write stores a single experience.
func (s *QdrantStore) Write(ctx context.Context, exp *domain.Experience) error {
	if exp == nil || exp.ExperienceID == "" || exp.UserID == "" || exp.Content == "" {
		return ErrInvalidInput
	}

	point := qdrantPoint{
		ID:      exp.ExperienceID,
		Vector:  exp.Embedding,
		Payload: qdrantPayload(exp),
	}

	req := map[string]any{
		"points": []qdrantPoint{point},
	}

	resp, err := s.doRequest(ctx, "PUT", "/collections/"+s.collection+"/points", req)
	if err != nil {
		return fmt.Errorf("qdrant upsert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant upsert failed: %s", string(body))
	}

	return nil
}

// WriteBatch stores multiple experiences.
func (s *QdrantStore) WriteBatch(ctx context.Context, experiences []*domain.Experience) (int, error) {
	if len(experiences) == 0 {
		return 0, nil
	}

	points := make([]qdrantPoint, 0, len(experiences))
	for _, exp := range experiences {
		if exp == nil || exp.ExperienceID == "" || exp.UserID == "" || exp.Content == "" {
			continue
		}
		points = append(points, qdrantPoint{
			ID:      exp.ExperienceID,
			Vector:  exp.Embedding,
			Payload: qdrantPayload(exp),
		})
	}

	if len(points) == 0 {
		return 0, ErrInvalidInput
	}

	req := map[string]any{
		"points": points,
	}

	resp, err := s.doRequest(ctx, "PUT", "/collections/"+s.collection+"/points", req)
	if err != nil {
		return 0, fmt.Errorf("qdrant batch upsert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("qdrant batch upsert failed: %s", string(body))
	}

	return len(points), nil
}

// qdrantSearchResponse represents Qdrant search response.
type qdrantSearchResult struct {
	ID      string         `json:"id"`
	Version int64          `json:"version"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

// Search performs semantic search.
func (s *QdrantStore) Search(ctx context.Context, userID string, queryVec []float32, filter domain.ExperienceFilter) (*domain.ExperienceSearchResult, error) {
	start := time.Now()

	limit := filter.Limit
	if limit <= 0 {
		limit = domain.DefaultExperienceLimit
	}
	if limit > domain.MaxExperienceLimit {
		limit = domain.MaxExperienceLimit
	}

	// Build Qdrant filter for user isolation
	qdrantFilter := map[string]any{
		"must": []map[string]any{
			{
				"key":   "user_id",
				"match": map[string]any{"value": userID},
			},
		},
	}

	// Add optional filters
	must := qdrantFilter["must"].([]map[string]any)
	if filter.Topic != "" {
		must = append(must, map[string]any{
			"key":   "topic",
			"match": map[string]any{"value": filter.Topic},
		})
	}
	if filter.Outcome != "" {
		must = append(must, map[string]any{
			"key":   "outcome",
			"match": map[string]any{"value": string(filter.Outcome)},
		})
	}
	if filter.SessionID != "" {
		must = append(must, map[string]any{
			"key":   "session_id",
			"match": map[string]any{"value": filter.SessionID},
		})
	}
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			must = append(must, map[string]any{
				"key":   "tags",
				"match": map[string]any{"value": tag},
			})
		}
	}
	qdrantFilter["must"] = must

	req := map[string]any{
		"vector":   queryVec,
		"limit":    limit + filter.Offset,
		"filter":   qdrantFilter,
		"with_payload": true,
	}

	if filter.MinScore > 0 {
		req["score_threshold"] = filter.MinScore
	}

	resp, err := s.doRequest(ctx, "POST", "/collections/"+s.collection+"/points/search", req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qdrant search failed: %s", string(body))
	}

	var result struct {
		Result []qdrantSearchResult `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	experiences := make([]domain.Experience, 0, len(result.Result))
	for _, r := range result.Result {
		exp, err := payloadToExperience(r.ID, r.Payload, r.Score)
		if err != nil {
			slog.Warn("failed to parse qdrant result", "id", r.ID, "err", err)
			continue
		}
		experiences = append(experiences, *exp)
	}

	// Apply offset
	total := len(experiences)
	if filter.Offset > 0 && filter.Offset < total {
		experiences = experiences[filter.Offset:]
	}

	return &domain.ExperienceSearchResult{
		Experiences: experiences,
		Total:       total,
		Query:       filter.Query,
		LatencyMs:   time.Since(start).Milliseconds(),
	}, nil
}

// GetByID retrieves a single experience.
func (s *QdrantStore) GetByID(ctx context.Context, userID, experienceID string) (*domain.Experience, error) {
	resp, err := s.doRequest(ctx, "GET", "/collections/"+s.collection+"/points/"+url.PathEscape(experienceID), nil)
	if err != nil {
		return nil, fmt.Errorf("qdrant get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qdrant get failed: %s", string(body))
	}

	var result struct {
		Result struct {
			ID      string         `json:"id"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode get response: %w", err)
	}

	// Verify user isolation
	if result.Result.Payload["user_id"] != userID {
		return nil, ErrNotFound
	}

	exp, err := payloadToExperience(result.Result.ID, result.Result.Payload, 0)
	if err != nil {
		return nil, fmt.Errorf("parse experience: %w", err)
	}

	return exp, nil
}

// Delete removes an experience.
func (s *QdrantStore) Delete(ctx context.Context, userID, experienceID string) error {
	// First verify ownership
	_, err := s.GetByID(ctx, userID, experienceID)
	if err != nil {
		return err
	}

	req := map[string]any{
		"points": []string{experienceID},
	}

	resp, err := s.doRequest(ctx, "POST", "/collections/"+s.collection+"/points/delete", req)
	if err != nil {
		return fmt.Errorf("qdrant delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant delete failed: %s", string(body))
	}

	return nil
}

// DeleteByUser removes all experiences for a user.
func (s *QdrantStore) DeleteByUser(ctx context.Context, userID string) error {
	req := map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key":   "user_id",
					"match": map[string]any{"value": userID},
				},
			},
		},
	}

	resp, err := s.doRequest(ctx, "POST", "/collections/"+s.collection+"/points/delete", req)
	if err != nil {
		return fmt.Errorf("qdrant delete by user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant delete by user failed: %s", string(body))
	}

	return nil
}

// Stats returns statistics for a user.
func (s *QdrantStore) Stats(ctx context.Context, userID string) (*domain.ExperienceStats, error) {
	// Get collection info
	resp, err := s.doRequest(ctx, "GET", "/collections/"+s.collection, nil)
	if err != nil {
		return nil, fmt.Errorf("qdrant collection info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qdrant collection info failed: %s", string(body))
	}

	// Scroll through user's experiences to compute stats
	// This is expensive but necessary without additional aggregation support
	filter := map[string]any{
		"must": []map[string]any{
			{
				"key":   "user_id",
				"match": map[string]any{"value": userID},
			},
		},
	}

	scrollReq := map[string]any{
		"filter":       filter,
		"limit":        1000,
		"with_payload": true,
		"with_vector":  false,
	}

	resp, err = s.doRequest(ctx, "POST", "/collections/"+s.collection+"/points/scroll", scrollReq)
	if err != nil {
		return nil, fmt.Errorf("qdrant scroll: %w", err)
	}
	defer resp.Body.Close()

	var scrollResult struct {
		Result struct {
			Points []struct {
				ID      string         `json:"id"`
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
		return nil, fmt.Errorf("decode scroll response: %w", err)
	}

	stats := &domain.ExperienceStats{
		UserID:          userID,
		TotalExperiences: len(scrollResult.Result.Points),
		ByTopic:         make(map[string]int),
		ByOutcome:       make(map[domain.ExperienceOutcome]int),
	}

	var totalLen int
	for _, p := range scrollResult.Result.Points {
		if topic, ok := p.Payload["topic"].(string); ok && topic != "" {
			stats.ByTopic[topic]++
		}
		if outcome, ok := p.Payload["outcome"].(string); ok {
			stats.ByOutcome[domain.ExperienceOutcome(outcome)]++
		}
		if content, ok := p.Payload["content"].(string); ok {
			totalLen += len(content)
		}
		if t, ok := p.Payload["created_at"].(float64); ok {
			tm := time.Unix(int64(t), 0)
			if stats.OldestExperience == nil || tm.Before(*stats.OldestExperience) {
				stats.OldestExperience = &tm
			}
			if stats.NewestExperience == nil || tm.After(*stats.NewestExperience) {
				stats.NewestExperience = &tm
			}
		}
	}

	if stats.TotalExperiences > 0 {
		stats.AverageContentLen = float64(totalLen) / float64(stats.TotalExperiences)
	}

	return stats, nil
}

// Health checks Qdrant connectivity.
func (s *QdrantStore) Health(ctx context.Context) error {
	resp, err := s.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return fmt.Errorf("qdrant health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// Close releases resources.
func (s *QdrantStore) Close() error {
	s.httpClient.CloseIdleConnections()
	return nil
}
