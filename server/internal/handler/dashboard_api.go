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
		// New endpoints for space and agent management
		r.Get("/spaces", h.GetSpaces)
		r.Get("/agents", h.GetAgentActivity)
		r.Get("/storage", h.GetStorageAnalysis)
		r.Get("/spaces/{tenantId}", h.GetSpaceDetail)
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

// GetSpaces handles GET /api/dashboard/spaces - returns list of all spaces with metrics.
func (h *DashboardHandler) GetSpaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Get all active tenants
	tenants, err := h.tenantRepo.ListActive(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	// Get agent tenant mapping
	agentTenants := h.metrics.GetAgentTenants()
	agentLastSeen := h.metrics.GetAgentLastSeen()
	tenantCounts := h.metrics.GetTenantRequestCounts()

	// Build agent counts per tenant and track last activity
	agentCountByTenant := make(map[string]int)
	agentActivityByTenant := make(map[string]time.Time)

	for agentID, tenantID := range agentTenants {
		agentCountByTenant[tenantID]++
		if lastSeen, exists := agentLastSeen[agentID]; exists {
			if current, exists := agentActivityByTenant[tenantID]; !exists || lastSeen.After(current) {
				agentActivityByTenant[tenantID] = lastSeen
			}
		}
	}

	// Build space list
	spaces := make([]domain.SpaceListItem, 0, len(tenants))
	for _, t := range tenants {
		lastActive := agentActivityByTenant[t.ID]

		space := domain.SpaceListItem{
			TenantID:     t.ID,
			TenantName:   t.Name,
			AgentCount:   agentCountByTenant[t.ID],
			LastActiveAt: &lastActive,
			Status:       string(t.Status),
			CreatedAt:    t.CreatedAt,
			// MemoryCount and StorageBytes would come from tenant DB queries
			// For now, estimate from request count as proxy for activity
		}

		// Estimate memory count from request activity (rough heuristic)
		// In production, this would query the tenant's database
		if requests, exists := tenantCounts[t.ID]; exists {
			// Rough estimate: 1 memory per 10 requests
			space.MemoryCount = int(requests / 10)
			// Rough estimate: 500 bytes per memory
			space.StorageBytes = int64(space.MemoryCount) * 500
		}

		spaces = append(spaces, space)
	}

	// Sort by memory count (descending) by default
	for i := 0; i < len(spaces); i++ {
		for j := i + 1; j < len(spaces); j++ {
			if spaces[j].MemoryCount > spaces[i].MemoryCount {
				spaces[i], spaces[j] = spaces[j], spaces[i]
			}
		}
	}

	respond(w, http.StatusOK, domain.SpaceListResponse{
		Spaces:      spaces,
		TotalCount:  len(spaces),
		CollectedAt: now,
	})
}

// GetSpaceDetail handles GET /api/dashboard/spaces/{tenantId} - returns detailed space info.
func (h *DashboardHandler) GetSpaceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := chi.URLParam(r, "tenantId")

	// Get tenant info
	tenant, err := h.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "tenant not found")
		return
	}

	now := time.Now()

	// Get agent info for this tenant
	agentTenants := h.metrics.GetAgentTenants()
	agentTypes := h.metrics.GetAgentTypes()
	agentLastSeen := h.metrics.GetAgentLastSeen()

	agents := make([]domain.AgentSummary, 0)
	for agentID, tID := range agentTenants {
		if tID == tenantID {
			agentType := agentTypes[agentID]
			if agentType == "" {
				agentType = domain.AgentTypeUnknown
			}
			lastSeen := agentLastSeen[agentID]

			agents = append(agents, domain.AgentSummary{
				AgentID:      agentID,
				AgentType:    string(agentType),
				LastActiveAt: lastSeen,
				// MemoryCount would need tenant DB query
			})
		}
	}

	// Build response
	space := domain.SpaceListItem{
		TenantID:     tenant.ID,
		TenantName:   tenant.Name,
		AgentCount:   len(agents),
		Agents:       agents,
		Status:       string(tenant.Status),
		CreatedAt:    tenant.CreatedAt,
		CollectedAt:  now,
	}

	respond(w, http.StatusOK, space)
}

// GetAgentActivity handles GET /api/dashboard/agents - returns agent activity with timeline.
func (h *DashboardHandler) GetAgentActivity(w http.ResponseWriter, r *http.Request) {
	// Get base activity from metrics
	response := h.metrics.GetAgentActivity()

	// Enrich with tenant names
	tenants, err := h.tenantRepo.ListActive(r.Context())
	if err == nil {
		tenantNames := make(map[string]string)
		for _, t := range tenants {
			tenantNames[t.ID] = t.Name
		}

		for i := range response.Agents {
			if name, exists := tenantNames[response.Agents[i].TenantID]; exists {
				response.Agents[i].TenantName = name
			}
		}
	}

	respond(w, http.StatusOK, response)
}

// GetStorageAnalysis handles GET /api/dashboard/storage - returns storage analysis.
func (h *DashboardHandler) GetStorageAnalysis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Get all active tenants
	tenants, err := h.tenantRepo.ListActive(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tenants")
		return
	}

	tenantCounts := h.metrics.GetTenantRequestCounts()

	// Build storage info per space
	// In production, this would query actual storage metrics from tenant DBs
	bySpace := make([]domain.SpaceStorageInfo, 0, len(tenants))
	var totalBytes int64

	for _, t := range tenants {
		// Estimate storage from activity
		// In production, this would query the tenant's database
		requests := tenantCounts[t.ID]
		memoryCount := int(requests / 10) // Rough estimate
		storageBytes := int64(memoryCount) * 500 // Rough estimate: 500 bytes per memory

		bySpace = append(bySpace, domain.SpaceStorageInfo{
			TenantID:     t.ID,
			TenantName:   t.Name,
			StorageBytes: storageBytes,
			MemoryCount:  memoryCount,
		})
		totalBytes += storageBytes
	}

	// Calculate percentages
	for i := range bySpace {
		if totalBytes > 0 {
			bySpace[i].Percent = float64(bySpace[i].StorageBytes) / float64(totalBytes) * 100
		}
	}

	// Sort by storage (descending)
	for i := 0; i < len(bySpace); i++ {
		for j := i + 1; j < len(bySpace); j++ {
			if bySpace[j].StorageBytes > bySpace[i].StorageBytes {
				bySpace[i], bySpace[j] = bySpace[j], bySpace[i]
			}
		}
	}

	// Build trend data (last 30 days)
	// In production, this would query historical metrics storage
	trend := make([]domain.StorageTrendPoint, 30)
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -(29 - i))
		trend[i] = domain.StorageTrendPoint{
			Date:         date.Format("2006-01-02"),
			StorageBytes: int64(float64(totalBytes) * (0.7 + 0.01*float64(i))), // Simulated growth
			MemoryCount:  int(float64(int(totalBytes)/500) * (0.7 + 0.01*float64(i))),
		}
	}

	respond(w, http.StatusOK, domain.StorageAnalysisResponse{
		TotalBytes:  totalBytes,
		BySpace:     bySpace,
		Trend:       trend,
		CollectedAt: now,
	})
}
