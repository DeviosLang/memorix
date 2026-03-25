package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/devioslang/memorix/server/internal/service"
)

// DashboardHandler handles dashboard API endpoints.
// All dashboard endpoints require authentication via X-Dashboard-Token header.
type DashboardHandler struct {
	metrics    *service.MetricsCollector
	tenantRepo repository.TenantRepo
	token      string // Dashboard authentication token
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(
	metrics *service.MetricsCollector,
	tenantRepo repository.TenantRepo,
	token string,
) *DashboardHandler {
	return &DashboardHandler{
		metrics:    metrics,
		tenantRepo: tenantRepo,
		token:      token,
	}
}

// RegisterRoutes registers dashboard routes on the given router.
func (h *DashboardHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/dashboard", func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Get("/overview", h.GetOverview)
		r.Get("/memory-stats", h.GetMemoryStats)
		r.Get("/search-stats", h.GetSearchStats)
		r.Get("/gc-stats", h.GetGCStats)
		r.Get("/space-stats", h.GetSpaceStats)
		r.Get("/conflict-stats", h.GetConflictStats)
	})
}

// authMiddleware validates the X-Dashboard-Token header.
func (h *DashboardHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If no token configured, deny access
		if h.token == "" {
			respondError(w, http.StatusServiceUnavailable, "dashboard not configured")
			return
		}

		token := r.Header.Get("X-Dashboard-Token")
		if token == "" {
			respondError(w, http.StatusUnauthorized, "missing X-Dashboard-Token header")
			return
		}

		if token != h.token {
			respondError(w, http.StatusUnauthorized, "invalid dashboard token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetOverview handles GET /api/dashboard/overview
func (h *DashboardHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	overview := h.metrics.GetOverview()

	// Get active tenant count
	tenants, err := h.tenantRepo.ListActive(ctx)
	if err == nil {
		overview.ActiveTenants = len(tenants)
	}

	respond(w, http.StatusOK, overview)
}

// GetMemoryStats handles GET /api/dashboard/memory-stats
func (h *DashboardHandler) GetMemoryStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	now := time.Now()
	stats := domain.DashboardMemoryStats{
		ByState:      make(map[string]int),
		ByType:       make(map[string]int),
		CollectedAt:  now,
	}

	// Get memory stats from all active tenants
	// For this implementation, we return basic aggregated data
	// In a production system, this would query tenant databases
	// or use pre-computed aggregates stored in a metrics table
	// Note: A more complete implementation would query each tenant DB
	// using the tenant pool to get accurate memory counts.
	_ = ctx // Will be used when querying tenant DBs

	respond(w, http.StatusOK, stats)
}

// GetSearchStats handles GET /api/dashboard/search-stats
func (h *DashboardHandler) GetSearchStats(w http.ResponseWriter, r *http.Request) {
	stats := h.metrics.GetSearchStats()
	respond(w, http.StatusOK, stats)
}

// GetGCStats handles GET /api/dashboard/gc-stats
func (h *DashboardHandler) GetGCStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	now := time.Now()
	stats := domain.DashboardGCStats{
		CollectedAt: now,
	}

	// Get GC stats from active tenants
	// This would require querying each tenant's GC log repository
	// For now, we return basic stats from metrics
	tenants, err := h.tenantRepo.ListActive(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	// Placeholder: In production, aggregate GC logs from all tenants
	_ = tenants // Will be used for tenant-specific queries

	respond(w, http.StatusOK, stats)
}

// GetSpaceStats handles GET /api/dashboard/space-stats
func (h *DashboardHandler) GetSpaceStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	now := time.Now()
	stats := domain.DashboardSpaceStats{
		AgentsByTenant: make(map[string]int),
		CollectedAt:    now,
	}

	// Get tenant stats
	tenants, err := h.tenantRepo.ListActive(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	stats.TotalTenants = len(tenants)
	stats.ActiveTenants = len(tenants) // ListActive only returns active tenants

	// Get active agents from metrics
	stats.ActiveAgents = h.metrics.GetActiveAgents()
	stats.TotalAgents = stats.ActiveAgents // Simplified: active agents count

	// Get tenant request counts
	tenantCounts := h.metrics.GetTenantRequestCounts()

	// Build top active tenants list
	type tenantActivity struct {
		tenantID   string
		tenantName string
		requests   int64
	}

	var activities []tenantActivity
	for _, t := range tenants {
		activities = append(activities, tenantActivity{
			tenantID:   t.ID,
			tenantName: t.Name,
			requests:   tenantCounts[t.ID],
		})
	}

	// Sort by request count (simple sort - could use sort.Slice)
	for i := 0; i < len(activities); i++ {
		for j := i + 1; j < len(activities); j++ {
			if activities[j].requests > activities[i].requests {
				activities[i], activities[j] = activities[j], activities[i]
			}
		}
	}

	// Take top 10
	topCount := 10
	if len(activities) < topCount {
		topCount = len(activities)
	}
	stats.TopActiveTenants = make([]domain.TenantActivity, topCount)
	for i := 0; i < topCount; i++ {
		stats.TopActiveTenants[i] = domain.TenantActivity{
			TenantID:     activities[i].tenantID,
			TenantName:   activities[i].tenantName,
			RequestCount: activities[i].requests,
		}
	}

	respond(w, http.StatusOK, stats)
}

// GetConflictStats handles GET /api/dashboard/conflict-stats
func (h *DashboardHandler) GetConflictStats(w http.ResponseWriter, r *http.Request) {
	stats := h.metrics.GetConflictStats()
	respond(w, http.StatusOK, stats)
}

// DashboardService provides methods for gathering dashboard data from tenant databases.
type DashboardService struct {
	tenantRepo repository.TenantRepo
	metrics    *service.MetricsCollector
}

// NewDashboardService creates a new dashboard service.
func NewDashboardService(
	tenantRepo repository.TenantRepo,
	metrics *service.MetricsCollector,
) *DashboardService {
	return &DashboardService{
		tenantRepo: tenantRepo,
		metrics:    metrics,
	}
}

// GatherTenantMemoryStats collects memory statistics from tenant databases.
// This is a helper that can be called periodically to pre-compute aggregates.
func (s *DashboardService) GatherTenantMemoryStats(ctx context.Context) (map[string]int64, error) {
	tenants, err := s.tenantRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	// In a production implementation, this would query each tenant's database
	// and aggregate the results. For now, return placeholder.
	stats := make(map[string]int64)
	for _, t := range tenants {
		stats[t.ID] = 0 // Placeholder
	}
	return stats, nil
}
