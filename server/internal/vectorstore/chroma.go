package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// ChromaStore implements VectorStore using Chroma vector database.
// Chroma is a good alternative for local deployment due to:
// - Simple HTTP API
// - Built-in embedding support (optional)
// - Lightweight and easy to set up
// - SQLite-based persistence (default)
//
// Architecture:
// - Single collection "experiences" with user_id as metadata
// - Filtering by user_id metadata for data isolation
// - L2 distance (can be changed to cosine)
type ChromaStore struct {
	baseURL    string
	collection string
	dimensions int
	httpClient *http.Client
	timeout    time.Duration
}

// ChromaConfig holds Chroma-specific configuration.
type ChromaConfig struct {
	// URL to Chroma server (e.g., "http://localhost:8000")
	URL string

	// Collection name (default: "experiences")
	Collection string

	// Embedding dimensions (default: 1536)
	Dimensions int

	// Distance function: "l2", "ip" (inner product), "cosine"
	DistanceFunc string

	// Timeout for operations (default: 30s)
	Timeout time.Duration
}

// NewChromaStore creates a new Chroma vector store.
func NewChromaStore(cfg ChromaConfig) (*ChromaStore, error) {
	if cfg.URL == "" {
		cfg.URL = "http://localhost:8000"
	}
	if cfg.Collection == "" {
		cfg.Collection = "experiences"
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = 1536
	}
	if cfg.DistanceFunc == "" {
		cfg.DistanceFunc = "cosine"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	store := &ChromaStore{
		baseURL:    strings.TrimSuffix(cfg.URL, "/"),
		collection: cfg.Collection,
		dimensions: cfg.Dimensions,
		timeout:    cfg.Timeout,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}

	// Create collection if not exists
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := store.ensureCollection(ctx, cfg.DistanceFunc); err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	return store, nil
}

// ensureCollection creates the collection if it doesn't exist.
func (s *ChromaStore) ensureCollection(ctx context.Context, distanceFunc string) error {
	// Check if collection exists
	resp, err := s.doRequest(ctx, "GET", "/api/v1/collections/"+s.collection, nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		slog.Info("chroma collection already exists", "collection", s.collection)
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Create collection
	createReq := map[string]any{
		"name": s.collection,
		"metadata": map[string]any{
			"hnsw:space": distanceFunc,
		},
	}

	resp, err = s.doRequest(ctx, "POST", "/api/v1/collections", createReq)
	if err != nil {
		return fmt.Errorf("create collection request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection failed: %s", string(body))
	}

	slog.Info("chroma collection created", "collection", s.collection, "distance", distanceFunc)
	return nil
}

// doRequest performs an HTTP request to the Chroma API.
func (s *ChromaStore) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
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

	return s.httpClient.Do(req)
}

// chromaMetadata converts an Experience to Chroma metadata.
func chromaMetadata(exp *domain.Experience) map[string]any {
	meta := map[string]any{
		"user_id":    exp.UserID,
		"content":    exp.Content, // Chroma requires separate document storage
		"created_at": exp.CreatedAt.Unix(),
		"updated_at": exp.UpdatedAt.Unix(),
	}

	if exp.Metadata.SessionID != "" {
		meta["session_id"] = exp.Metadata.SessionID
	}
	if exp.Metadata.Topic != "" {
		meta["topic"] = exp.Metadata.Topic
	}
	if exp.Metadata.Outcome != "" {
		meta["outcome"] = string(exp.Metadata.Outcome)
	}
	if exp.Metadata.Confidence > 0 {
		meta["confidence"] = exp.Metadata.Confidence
	}
	if len(exp.Metadata.Tags) > 0 {
		meta["tags"] = strings.Join(exp.Metadata.Tags, ",")
	}
	if exp.Metadata.Source != "" {
		meta["source"] = exp.Metadata.Source
	}
	if len(exp.Metadata.Extra) > 0 {
		meta["extra"] = string(exp.Metadata.Extra)
	}

	return meta
}

// metadataToExperience converts Chroma metadata back to Experience.
func metadataToExperience(id string, meta map[string]any, document string, distance float64) (*domain.Experience, error) {
	createdAt := time.Now()
	if t, ok := meta["created_at"].(float64); ok {
		createdAt = time.Unix(int64(t), 0)
	}
	updatedAt := time.Now()
	if t, ok := meta["updated_at"].(float64); ok {
		updatedAt = time.Unix(int64(t), 0)
	}

	var tags []string
	if t, ok := meta["tags"].(string); ok && t != "" {
		tags = strings.Split(t, ",")
	}

	var extra json.RawMessage
	if e, ok := meta["extra"].(string); ok && e != "" && e != "null" {
		extra = json.RawMessage(e)
	}

	outcome := domain.OutcomeNeutral
	if o, ok := meta["outcome"].(string); ok {
		outcome = domain.ExperienceOutcome(o)
	}

	confidence := 1.0
	if c, ok := meta["confidence"].(float64); ok {
		confidence = c
	}

	// Chroma uses distance, convert to similarity (score)
	// For cosine distance: similarity = 1 - distance
	score := 1 - distance
	if score < 0 {
		score = 0
	}

	return &domain.Experience{
		ExperienceID: id,
		UserID:       meta["user_id"].(string),
		Content:      document,
		Metadata: domain.ExperienceMetadata{
			SessionID:  getStringFromMap(meta, "session_id"),
			Topic:      getStringFromMap(meta, "topic"),
			Outcome:    outcome,
			Confidence: confidence,
			Tags:       tags,
			Source:     getStringFromMap(meta, "source"),
			Extra:      extra,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Score:     &score,
	}, nil
}

func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Write stores a single experience.
func (s *ChromaStore) Write(ctx context.Context, exp *domain.Experience) error {
	if exp == nil || exp.ExperienceID == "" || exp.UserID == "" || exp.Content == "" {
		return ErrInvalidInput
	}

	req := map[string]any{
		"ids":       []string{exp.ExperienceID},
		"embeddings": [][]float32{exp.Embedding},
		"metadatas": []map[string]any{chromaMetadata(exp)},
		"documents": []string{exp.Content},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/add", req)
	if err != nil {
		return fmt.Errorf("chroma add: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chroma add failed: %s", string(body))
	}

	return nil
}

// WriteBatch stores multiple experiences.
func (s *ChromaStore) WriteBatch(ctx context.Context, experiences []*domain.Experience) (int, error) {
	if len(experiences) == 0 {
		return 0, nil
	}

	ids := make([]string, 0, len(experiences))
	embeddings := make([][]float32, 0, len(experiences))
	metadatas := make([]map[string]any, 0, len(experiences))
	documents := make([]string, 0, len(experiences))

	for _, exp := range experiences {
		if exp == nil || exp.ExperienceID == "" || exp.UserID == "" || exp.Content == "" {
			continue
		}
		ids = append(ids, exp.ExperienceID)
		embeddings = append(embeddings, exp.Embedding)
		metadatas = append(metadatas, chromaMetadata(exp))
		documents = append(documents, exp.Content)
	}

	if len(ids) == 0 {
		return 0, ErrInvalidInput
	}

	req := map[string]any{
		"ids":        ids,
		"embeddings": embeddings,
		"metadatas":  metadatas,
		"documents":  documents,
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/add", req)
	if err != nil {
		return 0, fmt.Errorf("chroma batch add: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("chroma batch add failed: %s", string(body))
	}

	return len(ids), nil
}

// Search performs semantic search.
func (s *ChromaStore) Search(ctx context.Context, userID string, queryVec []float32, filter domain.ExperienceFilter) (*domain.ExperienceSearchResult, error) {
	start := time.Now()

	limit := filter.Limit
	if limit <= 0 {
		limit = domain.DefaultExperienceLimit
	}
	if limit > domain.MaxExperienceLimit {
		limit = domain.MaxExperienceLimit
	}

	// Build Chroma where filter for user isolation
	where := map[string]any{
		"user_id": userID,
	}

	// Add optional filters
	if filter.Topic != "" {
		where["topic"] = filter.Topic
	}
	if filter.Outcome != "" {
		where["outcome"] = string(filter.Outcome)
	}
	if filter.SessionID != "" {
		where["session_id"] = filter.SessionID
	}

	req := map[string]any{
		"query_embeddings": []float32(queryVec),
		"n_results":         limit + filter.Offset,
		"where":             where,
		"include":          []string{"metadatas", "documents", "distances"},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/query", req)
	if err != nil {
		return nil, fmt.Errorf("chroma query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chroma query failed: %s", string(body))
	}

	var queryResult struct {
		IDs        [][]string           `json:"ids"`
		Distances  [][]float64          `json:"distances"`
		Metadatas  [][]map[string]any   `json:"metadatas"`
		Documents  [][]string           `json:"documents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&queryResult); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}

	experiences := make([]domain.Experience, 0)
	if len(queryResult.IDs) > 0 && len(queryResult.IDs[0]) > 0 {
		for i, id := range queryResult.IDs[0] {
			var meta map[string]any
			if i < len(queryResult.Metadatas[0]) {
				meta = queryResult.Metadatas[0][i]
			}
			var doc string
			if i < len(queryResult.Documents[0]) {
				doc = queryResult.Documents[0][i]
			}
			var dist float64
			if i < len(queryResult.Distances[0]) {
				dist = queryResult.Distances[0][i]
			}

			// Apply min score filter
			if filter.MinScore > 0 {
				similarity := 1 - dist
				if similarity < filter.MinScore {
					continue
				}
			}

			// Apply tag filter (Chroma doesn't support array contains)
			if len(filter.Tags) > 0 {
				tagsStr, _ := meta["tags"].(string)
				tags := strings.Split(tagsStr, ",")
				if !containsAllTags(tags, filter.Tags) {
					continue
				}
			}

			exp, err := metadataToExperience(id, meta, doc, dist)
			if err != nil {
				slog.Warn("failed to parse chroma result", "id", id, "err", err)
				continue
			}
			experiences = append(experiences, *exp)
		}
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

// containsAllTags checks if all required tags are present.
func containsAllTags(have []string, need []string) bool {
	haveMap := make(map[string]bool)
	for _, t := range have {
		haveMap[strings.TrimSpace(t)] = true
	}
	for _, t := range need {
		if !haveMap[strings.TrimSpace(t)] {
			return false
		}
	}
	return true
}

// GetByID retrieves a single experience.
func (s *ChromaStore) GetByID(ctx context.Context, userID, experienceID string) (*domain.Experience, error) {
	req := map[string]any{
		"ids":     []string{experienceID},
		"include": []string{"metadatas", "documents"},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/get", req)
	if err != nil {
		return nil, fmt.Errorf("chroma get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chroma get failed: %s", string(body))
	}

	var getResult struct {
		IDs       []string           `json:"ids"`
		Metadatas []map[string]any   `json:"metadatas"`
		Documents []string           `json:"documents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&getResult); err != nil {
		return nil, fmt.Errorf("decode get response: %w", err)
	}

	if len(getResult.IDs) == 0 {
		return nil, ErrNotFound
	}

	// Verify user isolation
	if getResult.Metadatas[0]["user_id"] != userID {
		return nil, ErrNotFound
	}

	exp, err := metadataToExperience(getResult.IDs[0], getResult.Metadatas[0], getResult.Documents[0], 0)
	if err != nil {
		return nil, fmt.Errorf("parse experience: %w", err)
	}

	return exp, nil
}

// Delete removes an experience.
func (s *ChromaStore) Delete(ctx context.Context, userID, experienceID string) error {
	// First verify ownership
	_, err := s.GetByID(ctx, userID, experienceID)
	if err != nil {
		return err
	}

	req := map[string]any{
		"ids": []string{experienceID},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/delete", req)
	if err != nil {
		return fmt.Errorf("chroma delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chroma delete failed: %s", string(body))
	}

	return nil
}

// DeleteByUser removes all experiences for a user.
func (s *ChromaStore) DeleteByUser(ctx context.Context, userID string) error {
	req := map[string]any{
		"where": map[string]any{
			"user_id": userID,
		},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/delete", req)
	if err != nil {
		return fmt.Errorf("chroma delete by user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chroma delete by user failed: %s", string(body))
	}

	return nil
}

// Stats returns statistics for a user.
func (s *ChromaStore) Stats(ctx context.Context, userID string) (*domain.ExperienceStats, error) {
	// Get all user's experiences to compute stats
	req := map[string]any{
		"where": map[string]any{
			"user_id": userID,
		},
		"limit":   10000,
		"include": []string{"metadatas", "documents"},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/v1/collections/"+s.collection+"/get", req)
	if err != nil {
		return nil, fmt.Errorf("chroma get for stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chroma get for stats failed: %s", string(body))
	}

	var getResult struct {
		IDs       []string           `json:"ids"`
		Metadatas []map[string]any   `json:"metadatas"`
		Documents []string           `json:"documents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&getResult); err != nil {
		return nil, fmt.Errorf("decode get response: %w", err)
	}

	stats := &domain.ExperienceStats{
		UserID:          userID,
		TotalExperiences: len(getResult.IDs),
		ByTopic:         make(map[string]int),
		ByOutcome:       make(map[domain.ExperienceOutcome]int),
	}

	var totalLen int
	for i, meta := range getResult.Metadatas {
		if topic, ok := meta["topic"].(string); ok && topic != "" {
			stats.ByTopic[topic]++
		}
		if outcome, ok := meta["outcome"].(string); ok {
			stats.ByOutcome[domain.ExperienceOutcome(outcome)]++
		}
		if i < len(getResult.Documents) {
			totalLen += len(getResult.Documents[i])
		}
		if t, ok := meta["created_at"].(float64); ok {
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

// Health checks Chroma connectivity.
func (s *ChromaStore) Health(ctx context.Context) error {
	resp, err := s.doRequest(ctx, "GET", "/api/v1/heartbeat", nil)
	if err != nil {
		return fmt.Errorf("chroma health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chroma unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// Close releases resources.
func (s *ChromaStore) Close() error {
	s.httpClient.CloseIdleConnections()
	return nil
}
