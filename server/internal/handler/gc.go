package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/devioslang/memorix/server/internal/domain"
)

// triggerGC manually triggers a GC run.
// POST /v1alpha1/memorix/{tenantID}/gc
func (s *Server) triggerGC(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	gcSvc := s.resolveGCServices(auth)

	// Parse request
	var req struct {
		DryRun bool `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Run GC
	result, err := gcSvc.Run(r.Context(), auth.TenantID, req.DryRun)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, result)
}

// previewGC returns a preview of what would be deleted without actually deleting.
// POST /v1alpha1/memorix/{tenantID}/gc/preview
func (s *Server) previewGC(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	gcSvc := s.resolveGCServices(auth)

	// Get preview
	preview, err := gcSvc.Preview(r.Context(), auth.TenantID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, preview)
}

// listGCLogs lists GC logs for the tenant.
// GET /v1alpha1/memorix/{tenantID}/gc/logs?limit=100&offset=0
func (s *Server) listGCLogs(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	gcSvc := s.resolveGCServices(auth)

	// Parse pagination
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	// List logs
	logs, total, err := gcSvc.ListGCLogs(r.Context(), auth.TenantID, limit, offset)
	if err != nil {
		s.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": total,
	})
}

// getGCSnapshot retrieves a GC snapshot by ID.
// GET /v1alpha1/memorix/{tenantID}/gc/snapshots/{id}
func (s *Server) getGCSnapshot(w http.ResponseWriter, r *http.Request) {
	auth := authInfo(r)
	gcSvc := s.resolveGCServices(auth)

	// Get snapshot ID from URL
	snapshotID := chi.URLParam(r, "id")
	if snapshotID == "" {
		respondError(w, http.StatusBadRequest, "snapshot ID required")
		return
	}

	// Get snapshot
	snapshot, err := gcSvc.GetSnapshot(r.Context(), snapshotID)
	if err != nil {
		if err == domain.ErrNotFound {
			respondError(w, http.StatusNotFound, "snapshot not found")
			return
		}
		s.handleError(w, err)
		return
	}

	// Verify tenant ownership
	if snapshot.TenantID != auth.TenantID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	respond(w, http.StatusOK, snapshot)
}

// writeJSON is an alias for respond (used by tests and external code)
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	respond(w, status, data)
}

// writeError is an alias for respondError (used by tests and external code)
func writeError(w http.ResponseWriter, status int, msg string) {
	respondError(w, status, msg)
}

