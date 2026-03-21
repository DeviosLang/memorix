package handler

import (
	"net/http"
	"strconv"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
	"github.com/go-chi/chi/v5"
)

// summarizeRequest is the request body for generating a conversation summary.
type summarizeRequest struct {
	UserID    string                   `json:"user_id"`
	SessionID string                   `json:"session_id,omitempty"`
	Messages  []domain.SummaryMessage  `json:"messages"`
}

// summaryListResponse is the response for listing summaries.
type summaryListResponse struct {
	Summaries []domain.ConversationSummary `json:"summaries"`
	Total     int                          `json:"total"`
	Limit     int                          `json:"limit"`
	Offset    int                          `json:"offset"`
}

// summarize handles POST /summaries
// It generates a summary for a completed conversation.
func (s *Server) summarize(w http.ResponseWriter, r *http.Request) {
	var req summarizeRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	auth := authInfo(r)
	svc := s.resolveSummarizerServices(auth)

	summarizeReq := service.SummarizeRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Messages:  req.Messages,
	}

	result, err := svc.Summarize(r.Context(), summarizeReq)
	if err != nil {
		s.handleError(w, err)
		return
	}

	status := http.StatusOK
	if result.Status == "failed" {
		status = http.StatusInternalServerError
	}

	respond(w, status, result)
}

// listSummaries handles GET /summaries
// It lists summaries for a user.
func (s *Server) listSummaries(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveSummarizerServices(auth)
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	filter := domain.ConversationSummaryFilter{
		UserID:    q.Get("user_id"),
		SessionID: q.Get("session_id"),
		KeyTopic:  q.Get("topic"),
		Limit:     limit,
		Offset:    offset,
	}

	summaries, total, err := svc.ListSummaries(r.Context(), filter)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if summaries == nil {
		summaries = []domain.ConversationSummary{}
	}

	respond(w, http.StatusOK, summaryListResponse{
		Summaries: summaries,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	})
}

// getSummary handles GET /summaries/{id}
// It retrieves a single summary by ID.
func (s *Server) getSummary(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveSummarizerServices(auth)
	id := chi.URLParam(r, "id")

	summary, err := svc.GetSummary(r.Context(), id)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, summary)
}

// deleteSummary handles DELETE /summaries/{id}
// It deletes a summary by ID.
func (s *Server) deleteSummary(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveSummarizerServices(auth)
	id := chi.URLParam(r, "id")

	if err := svc.DeleteSummary(r.Context(), id); err != nil {
		s.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
