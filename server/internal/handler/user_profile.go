package handler

import (
	"net/http"
	"strconv"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
	"github.com/go-chi/chi/v5"
)

// createFactRequest is the request body for creating a new fact.
type createFactRequest struct {
	UserID     string  `json:"user_id"`
	Category   string  `json:"category"`
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// factListResponse is the response for listing facts.
type factListResponse struct {
	Facts  []domain.UserProfileFact `json:"facts"`
	Total  int                      `json:"total"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
}

// createFact handles POST /user-profile/facts
func (s *Server) createFact(w http.ResponseWriter, r *http.Request) {
	var req createFactRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)

	input := service.CreateFactInput{
		UserID:     req.UserID,
		Category:   domain.FactCategory(req.Category),
		Key:        req.Key,
		Value:      req.Value,
		Source:     domain.FactSource(req.Source),
		Confidence: req.Confidence,
	}

	// Set default confidence for explicit facts if not provided
	if input.Source == domain.SourceExplicit && input.Confidence == 0 {
		input.Confidence = 1.0
	}

	fact, err := svc.CreateFact(r.Context(), input)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusCreated, fact)
}

// listFacts handles GET /user-profile/facts
func (s *Server) listFacts(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filter := domain.UserProfileFactFilter{
		UserID:   q.Get("user_id"),
		Category: domain.FactCategory(q.Get("category")),
		Key:      q.Get("key"),
		Source:   domain.FactSource(q.Get("source")),
		Limit:    limit,
		Offset:   offset,
	}

	facts, total, err := svc.ListFacts(r.Context(), filter)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if facts == nil {
		facts = []domain.UserProfileFact{}
	}

	respond(w, http.StatusOK, factListResponse{
		Facts:  facts,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// getFact handles GET /user-profile/facts/{id}
func (s *Server) getFact(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)
	id := chi.URLParam(r, "id")

	fact, err := svc.GetFact(r.Context(), id)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, fact)
}

// updateFactRequest is the request body for updating a fact.
type updateFactRequest struct {
	Category   *string  `json:"category,omitempty"`
	Key        *string  `json:"key,omitempty"`
	Value      *string  `json:"value,omitempty"`
	Source     *string  `json:"source,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// updateFact handles PUT /user-profile/facts/{id}
func (s *Server) updateFact(w http.ResponseWriter, r *http.Request) {
	var req updateFactRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)
	id := chi.URLParam(r, "id")

	input := service.UpdateFactInput{}
	if req.Category != nil {
		cat := domain.FactCategory(*req.Category)
		input.Category = &cat
	}
	if req.Key != nil {
		input.Key = req.Key
	}
	if req.Value != nil {
		input.Value = req.Value
	}
	if req.Source != nil {
		src := domain.FactSource(*req.Source)
		input.Source = &src
	}
	if req.Confidence != nil {
		input.Confidence = req.Confidence
	}

	fact, err := svc.UpdateFact(r.Context(), id, input)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, fact)
}

// deleteFact handles DELETE /user-profile/facts/{id}
func (s *Server) deleteFact(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveUserProfileServices(auth)
	id := chi.URLParam(r, "id")

	if err := svc.DeleteFact(r.Context(), id); err != nil {
		s.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractFactsRequest is the request body for extracting facts from conversation.
type extractFactsRequest struct {
	Messages  []service.IngestMessage `json:"messages"`
	UserID    string                  `json:"user_id"`
	SessionID string                  `json:"session_id,omitempty"`
}

// extractFacts handles POST /user-profile/extract
// It extracts structured facts from conversation content using LLM.
func (s *Server) extractFacts(w http.ResponseWriter, r *http.Request) {
	var req extractFactsRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	auth := authInfo(r)
	svc := s.resolveExtractorServices(auth)

	extractReq := service.ExtractRequest{
		Messages:  req.Messages,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	}

	// Run extraction synchronously for now
	// Can be made async if needed for large conversations
	result, err := svc.Extract(r.Context(), extractReq)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, result)
}

// batchReconcileRequest is the request body for batch reconciliation.
type batchReconcileRequest struct {
	Facts []domain.ReconcileRequest `json:"facts"`
}

// batchReconcile handles POST /user-profile/reconcile
// It reconciles multiple facts with LLM-driven conflict resolution.
func (s *Server) batchReconcile(w http.ResponseWriter, r *http.Request) {
	var req batchReconcileRequest
	if err := decode(r, &req); err != nil {
		s.handleError(w, err)
		return
	}

	if len(req.Facts) == 0 {
		respondError(w, http.StatusBadRequest, "facts array is required")
		return
	}

	auth := authInfo(r)
	svc := s.resolveReconcilerServices(auth)

	result, err := svc.BatchReconcile(r.Context(), req.Facts)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, result)
}

// auditLogListResponse is the response for listing audit logs.
type auditLogListResponse struct {
	Logs   []domain.ReconcileAuditLog `json:"logs"`
	Total  int                        `json:"total"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
}

// listAuditLogs handles GET /user-profile/reconcile/audit
// It lists audit logs for a user.
func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveReconcilerServices(auth)
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	userID := q.Get("user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	logs, err := svc.GetAuditLogs(r.Context(), userID, limit, offset)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if logs == nil {
		logs = []domain.ReconcileAuditLog{}
	}

	respond(w, http.StatusOK, auditLogListResponse{
		Logs:   logs,
		Total:  len(logs),
		Limit:  limit,
		Offset: offset,
	})
}

// listFactAuditLogs handles GET /user-profile/reconcile/audit/{fact_id}
// It lists audit logs for a specific fact.
func (s *Server) listFactAuditLogs(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	svc := s.resolveReconcilerServices(auth)
	q := r.URL.Query()
	factID := chi.URLParam(r, "fact_id")

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	logs, err := svc.GetFactAuditLogs(r.Context(), factID, limit, offset)
	if err != nil {
		s.handleError(w, err)
		return
	}

	if logs == nil {
		logs = []domain.ReconcileAuditLog{}
	}

	respond(w, http.StatusOK, auditLogListResponse{
		Logs:   logs,
		Total:  len(logs),
		Limit:  limit,
		Offset: offset,
	})
}
