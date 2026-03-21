package service

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/devioslang/memorix/server/internal/repository/tidb"
)

// GCWorker runs periodic memory garbage collection across all tenants.
type GCWorker struct {
	tenantRepo  repository.TenantRepo
	getTenantDB func(ctx context.Context, tenantID string) (*sql.DB, error)
	gcConfig    domain.GCConfig
	logger      *slog.Logger
}

// NewGCWorker creates a new GC worker.
func NewGCWorker(
	tenantRepo repository.TenantRepo,
	getTenantDB func(ctx context.Context, tenantID string) (*sql.DB, error),
	gcConfig domain.GCConfig,
	logger *slog.Logger,
) *GCWorker {
	return &GCWorker{
		tenantRepo:  tenantRepo,
		getTenantDB: getTenantDB,
		gcConfig:    gcConfig,
		logger:      logger,
	}
}

// Run starts the GC worker loop.
func (w *GCWorker) Run(ctx context.Context) error {
	if !w.gcConfig.Enabled {
		w.logger.Info("GC worker disabled")
		return nil
	}

	w.logger.Info("starting GC worker",
		"interval", w.gcConfig.Interval,
		"stale_threshold", w.gcConfig.StaleThreshold,
		"max_memories_per_tenant", w.gcConfig.MaxMemoriesPerTenant,
	)

	ticker := time.NewTicker(w.gcConfig.Interval)
	defer ticker.Stop()

	// Run once on startup (optional, can be commented out if not desired)
	w.logger.Info("running initial GC cycle")
	w.runGC(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("GC worker stopped")
			return ctx.Err()
		case <-ticker.C:
			w.logger.Info("running scheduled GC cycle")
			w.runGC(ctx)
		}
	}
}

// runGC runs GC for all active tenants.
func (w *GCWorker) runGC(ctx context.Context) {
	startTime := time.Now()

	// Get all active tenants (we'll need to add this method to TenantRepo)
	// For now, we'll iterate through the pool's cached tenants
	// In a real implementation, we'd want to get all active tenants from the control plane DB

	// Note: This is a simplified implementation. In production, you'd want to:
	// 1. Query all active tenants from the control plane DB
	// 2. Run GC for each tenant in parallel (with rate limiting)
	// 3. Handle errors gracefully

	w.logger.Info("GC cycle completed",
		"duration", time.Since(startTime),
	)

	// TODO: Implement tenant iteration when TenantRepo.List() is available
	// For now, GC must be triggered manually via API
}

// RunGCForTenant runs GC for a specific tenant (used by manual triggers and API).
func (w *GCWorker) RunGCForTenant(ctx context.Context, tenantID string, dryRun bool) (*domain.GCResult, error) {
	// Get tenant DB connection
	db, err := w.getTenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Create repositories for this tenant
	memRepo := tidb.NewMemoryRepo(db, "", false) // autoModel and ftsEnabled not needed for GC
	gcLogRepo := tidb.NewMemoryGCLogRepo(db)
	gcSnapshotRepo := tidb.NewMemoryGCSnapshotRepo(db)

	// Create GC service
	gcSvc := NewGCService(memRepo, gcLogRepo, gcSnapshotRepo, w.gcConfig, w.logger)

	// Run GC
	return gcSvc.Run(ctx, tenantID, dryRun)
}

// CleanupExpiredData cleans up expired snapshots and logs.
func (w *GCWorker) CleanupExpiredData(ctx context.Context) error {
	// Note: In production, this would iterate through all tenants
	// For now, this is a placeholder that should be called per-tenant
	w.logger.Info("cleanup expired data placeholder - implement tenant iteration")
	return nil
}
