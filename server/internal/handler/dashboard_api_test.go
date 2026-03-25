package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
)

// mockTenantRepo is a mock implementation of repository.TenantRepo for testing.
type mockTenantRepo struct {
	tenants []domain.Tenant
}

func (m *mockTenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	m.tenants = append(m.tenants, *t)
	return nil
}

func (m *mockTenantRepo) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	for _, t := range m.tenants {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockTenantRepo) GetByName(ctx context.Context, name string) (*domain.Tenant, error) {
	for _, t := range m.tenants {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockTenantRepo) UpdateStatus(ctx context.Context, id string, status domain.TenantStatus) error {
	return nil
}

func (m *mockTenantRepo) UpdateSchemaVersion(ctx context.Context, id string, version int) error {
	return nil
}

func (m *mockTenantRepo) ListActive(ctx context.Context) ([]domain.Tenant, error) {
	return m.tenants, nil
}

func TestDashboardHandler_AuthMiddleware(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	metrics := service.NewMetricsCollector(logger)
	tenantRepo := &mockTenantRepo{tenants: []domain.Tenant{}}
	token := "test-dashboard-token"

	handler := NewDashboardHandler(metrics, tenantRepo, token)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{
			name:       "valid token",
			token:      "test-dashboard-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid token",
			token:      "wrong-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing token",
			token:      "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/dashboard/overview", nil)
			if tt.token != "" {
				req.Header.Set("X-Dashboard-Token", tt.token)
			}
			rec := httptest.NewRecorder()

			// Create a simple handler that returns 200 if auth passes
			authHandler := handler.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			authHandler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestDashboardHandler_NoToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	metrics := service.NewMetricsCollector(logger)
	tenantRepo := &mockTenantRepo{tenants: []domain.Tenant{}}

	// Create handler with empty token (dashboard disabled)
	handler := NewDashboardHandler(metrics, tenantRepo, "")

	req := httptest.NewRequest("GET", "/api/dashboard/overview", nil)
	req.Header.Set("X-Dashboard-Token", "any-token")
	rec := httptest.NewRecorder()

	authHandler := handler.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	authHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d when no token configured, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestDashboardHandler_GetOverview(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	metrics := service.NewMetricsCollector(logger)
	tenantRepo := &mockTenantRepo{tenants: []domain.Tenant{
		{ID: "tenant-1", Name: "Test Tenant", Status: domain.TenantActive},
	}}
	token := "test-token"

	handler := NewDashboardHandler(metrics, tenantRepo, token)

	// Record some requests
	metrics.RecordRequest(domain.RequestMetric{
		Path:       "/test",
		Method:     "GET",
		StatusCode: 200,
		LatencyMs:  10,
		Timestamp:  metrics.GetOverview().StartTime,
		TenantID:   "tenant-1",
		AgentID:    "agent-1",
	})

	req := httptest.NewRequest("GET", "/api/dashboard/overview", nil)
	req.Header.Set("X-Dashboard-Token", token)
	rec := httptest.NewRecorder()

	handler.GetOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestDashboardHandler_GetSearchStats(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	metrics := service.NewMetricsCollector(logger)
	tenantRepo := &mockTenantRepo{tenants: []domain.Tenant{}}
	token := "test-token"

	handler := NewDashboardHandler(metrics, tenantRepo, token)

	// Record some searches
	metrics.RecordSearch(domain.SearchMetric{
		SearchType: "vector",
		LatencyMs:  50,
		Success:    true,
		Timestamp:  metrics.GetOverview().StartTime,
		TenantID:   "tenant-1",
	})

	req := httptest.NewRequest("GET", "/api/dashboard/search-stats", nil)
	req.Header.Set("X-Dashboard-Token", token)
	rec := httptest.NewRecorder()

	handler.GetSearchStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestDashboardHandler_GetConflictStats(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	metrics := service.NewMetricsCollector(logger)
	tenantRepo := &mockTenantRepo{tenants: []domain.Tenant{}}
	token := "test-token"

	handler := NewDashboardHandler(metrics, tenantRepo, token)

	// Record some conflicts
	metrics.RecordConflict(domain.ConflictMetric{
		FactID:     "fact-1",
		UserID:     "user-1",
		Resolution: "lww",
		Success:    true,
		Timestamp:  metrics.GetOverview().StartTime,
		TenantID:   "tenant-1",
	})

	req := httptest.NewRequest("GET", "/api/dashboard/conflict-stats", nil)
	req.Header.Set("X-Dashboard-Token", token)
	rec := httptest.NewRecorder()

	handler.GetConflictStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
