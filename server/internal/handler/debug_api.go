package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
)

// debugContextHandler handles the debug context API.

// getDebugContext handles GET /api/debug/context/{session_id}
// Returns the complete context assembly result for a session.
func (s *Server) getDebugContext(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	q := r.URL.Query()
	userID := q.Get("user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	// Optional parameters
	maxTokens := 0
	if mt := q.Get("max_tokens"); mt != "" {
		if parsed, err := parseInt(mt); err == nil && parsed > 0 {
			maxTokens = parsed
		}
	}
	if maxTokens == 0 {
		maxTokens = s.contextBuilderConfig.MaxTokens
	}

	includePrompt := q.Get("include_prompt") == "true"

	auth := authInfo(r)
	svc := s.resolveServices(auth)

	// Build the debug context
	result, err := s.buildDebugContext(r, userID, sessionID, maxTokens, svc)
	if err != nil {
		s.handleError(w, err)
		return
	}

	// Optionally include the full prompt
	if !includePrompt {
		result.Prompt = ""
	}

	respond(w, http.StatusOK, result)
}

// buildDebugContext assembles the debug context information.
func (s *Server) buildDebugContext(r *http.Request, userID, sessionID string, maxTokens int, svc resolvedSvc) (*domain.DebugContextResult, error) {
	result := &domain.DebugContextResult{
		SessionID:  sessionID,
		UserID:     userID,
		MaxTokens:  maxTokens,
		Layers:     []domain.DebugLayerInfo{},
		Warnings:   []string{},
	}

	// Build context request
	buildReq := &domain.BuildContextRequest{
		SessionID:          sessionID,
		UserID:             userID,
		MaxTokens:          maxTokens,
	}

	// Fetch user memories
	memories, _, err := svc.memory.Search(r.Context(), domain.MemoryFilter{
		AgentID: userID,
		State:   "active",
		Limit:   50,
	})
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to fetch memories: %v", err))
	} else if len(memories) > 0 {
		buildReq.UserMemories = memories
		result.Layers = append(result.Layers, domain.DebugLayerInfo{
			Layer:      "memories",
			ItemCount:  len(memories),
			TokenCount: s.countMemoryTokens(memories),
			Source:     "memory_search",
		})
	}

	// Fetch user profile facts
	facts, err := svc.userProfile.GetFactsByUser(r.Context(), userID)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to fetch facts: %v", err))
	} else if len(facts) > 0 {
		buildReq.UserProfileFacts = facts
		result.Layers = append(result.Layers, domain.DebugLayerInfo{
			Layer:      "facts",
			ItemCount:  len(facts),
			TokenCount: s.countFactTokens(facts),
			Source:     "user_profile",
		})
	}

	// Fetch conversation summaries
	summaries, err := svc.summarizer.GetSummariesByUser(r.Context(), userID, 20)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to fetch summaries: %v", err))
	} else if len(summaries) > 0 {
		// Format summaries as text
		var summaryTexts []string
		for _, sum := range summaries {
			summaryTexts = append(summaryTexts, fmt.Sprintf("- %s: %s", sum.Title, sum.Summary))
		}
		buildReq.ConversationSummary = strings.Join(summaryTexts, "\n")
		result.Layers = append(result.Layers, domain.DebugLayerInfo{
			Layer:      "summaries",
			ItemCount:  len(summaries),
			TokenCount: s.tokenizer.CountTokens(buildReq.ConversationSummary),
			Source:     "conversation_summaries",
		})
	}

	// Fetch session metadata if available (future: implement SessionMetadataService)
	// For now, session metadata can be passed in via the build request
	_ = sessionID // sessionID is used for future session metadata lookup

	// Build the context using the context builder
	builder := service.NewContextBuilder(s.contextBuilderConfig)
	buildResult, err := builder.Build(buildReq)
	if err != nil {
		return nil, err
	}

	// Copy results
	result.TotalTokens = buildResult.TotalTokens
	result.Prompt = buildResult.Prompt
	result.CreatedAt = time.Now()

	// Add truncation warnings
	if buildResult.Truncated {
		for _, detail := range buildResult.TruncationDetails {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("Layer '%s' was truncated: %s (%d tokens removed)", 
					detail.Layer, detail.Reason, detail.TokensRemoved))
		}
	}

	// Update layer stats from build result
	for _, stat := range buildResult.LayerStats {
		for i := range result.Layers {
			if string(result.Layers[i].Layer) == string(stat.Layer) {
				result.Layers[i].TokenCount = stat.FinalTokens
				result.Layers[i].Truncated = stat.Truncated
				break
			}
		}
	}

	return result, nil
}

// countMemoryTokens counts tokens in memories.
func (s *Server) countMemoryTokens(memories []domain.Memory) int {
	var total int
	for _, m := range memories {
		total += s.tokenizer.CountTokens(m.Content)
	}
	return total
}

// countFactTokens counts tokens in user profile facts.
func (s *Server) countFactTokens(facts []domain.UserProfileFact) int {
	var total int
	for _, f := range facts {
		content := f.Key + ": " + f.Value
		total += s.tokenizer.CountTokens(content)
	}
	return total
}

// formatSessionMetadata formats session metadata for display.
func formatSessionMetadata(meta *domain.SessionMetadata) string {
	var parts []string
	if meta.DeviceType != "" {
		parts = append(parts, "Device: "+meta.DeviceType)
	}
	if meta.Timezone != "" {
		parts = append(parts, "Timezone: "+meta.Timezone)
	}
	if meta.EntrySource != "" {
		parts = append(parts, "Source: "+meta.EntrySource)
	}
	if meta.LanguagePreference != "" {
		parts = append(parts, "Language: "+meta.LanguagePreference)
	}
	return strings.Join(parts, ", ")
}

// parseInt parses an integer string.
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
