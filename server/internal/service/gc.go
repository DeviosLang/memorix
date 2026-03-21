package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
)

// GCService implements memory garbage collection.
type GCService struct {
	memories    repository.MemoryRepo
	gcLogs      repository.MemoryGCLogRepo
	gcSnapshots repository.MemoryGCSnapshotRepo
	config      domain.GCConfig
	logger      *slog.Logger
}

// NewGCService creates a new GC service.
func NewGCService(
	memories repository.MemoryRepo,
	gcLogs repository.MemoryGCLogRepo,
	gcSnapshots repository.MemoryGCSnapshotRepo,
	config domain.GCConfig,
	logger *slog.Logger,
) *GCService {
	return &GCService{
		memories:    memories,
		gcLogs:      gcLogs,
		gcSnapshots: gcSnapshots,
		config:      config,
		logger:      logger,
	}
}

// Run executes a GC cycle.
// If dryRun is true, it only previews what would be deleted without actually deleting.
func (s *GCService) Run(ctx context.Context, tenantID string, dryRun bool) (*domain.GCResult, error) {
	gcRunID := uuid.New().String()
	s.logger.Info("starting GC run",
		"gc_run_id", gcRunID,
		"tenant_id", tenantID,
		"dry_run", dryRun,
	)

	result := &domain.GCResult{
		GCRunID:  gcRunID,
		TenantID: tenantID,
		DryRun:   dryRun,
	}

	// Step 1: Recalculate importance scores
	if !dryRun {
		count, err := s.memories.RecalculateImportanceScores(ctx)
		if err != nil {
			s.logger.Error("failed to recalculate importance scores", "err", err)
			// Continue anyway - not critical
		} else {
			s.logger.Info("recalculated importance scores", "count", count)
		}
	}

	// Step 2: Find stale memories (time decay)
	staleThreshold := time.Now().Add(-s.config.StaleThreshold)
	staleMemories, err := s.memories.FindStaleMemories(ctx, staleThreshold, s.config.LowConfidenceThreshold, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("find stale memories: %w", err)
	}
	result.StaleMarked = len(staleMemories)

	// Step 3: Find low importance memories
	lowImportanceMemories, err := s.memories.FindLowImportanceMemories(ctx, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("find low importance memories: %w", err)
	}
	result.LowImportanceCount = len(lowImportanceMemories)

	// Step 4: Find over capacity memories
	overCapacityMemories, err := s.memories.FindOverCapacityMemories(ctx, s.config.MaxMemoriesPerTenant, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("find over capacity memories: %w", err)
	}
	result.CapacityCleaned = len(overCapacityMemories)

	// Combine all memories to delete, avoiding duplicates
	memoriesToDelete := s.mergeMemories(staleMemories, lowImportanceMemories, overCapacityMemories)
	// TotalDeleted will be updated to the actual count after deletion;
	// set the planned count now so dry-run callers see the preview value.
	result.TotalDeleted = len(memoriesToDelete)

	if len(memoriesToDelete) == 0 {
		s.logger.Info("no memories to clean up", "gc_run_id", gcRunID)
		return result, nil
	}

	// Step 5: Create snapshot before deletion (only in non-dry-run mode)
	if !dryRun {
		snapshotID, err := s.createSnapshot(ctx, gcRunID, tenantID, memoriesToDelete)
		if err != nil {
			s.logger.Error("failed to create snapshot", "err", err)
			// Continue anyway - snapshot is for recovery, not critical for GC
		} else {
			result.SnapshotCreated = true
			result.SnapshotID = snapshotID
		}
	}

	// Step 6: Log deletions (always log, even in dry-run)
	if err := s.logDeletions(ctx, gcRunID, tenantID, memoriesToDelete, dryRun); err != nil {
		s.logger.Error("failed to log deletions", "err", err)
		// Continue anyway - logging is not critical
	}

	// Step 7: Delete memories (only in non-dry-run mode)
	if !dryRun {
		ids := make([]string, len(memoriesToDelete))
		for i, m := range memoriesToDelete {
			ids[i] = m.ID
		}

		deleted, err := s.memories.HardDelete(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("delete memories: %w", err)
		}

		// Update TotalDeleted with the actual number of rows removed.
		result.TotalDeleted = int(deleted)

		s.logger.Info("deleted memories",
			"gc_run_id", gcRunID,
			"count", deleted,
			"stale", len(staleMemories),
			"low_importance", len(lowImportanceMemories),
			"over_capacity", len(overCapacityMemories),
		)
	} else {
		s.logger.Info("dry-run: would delete memories",
			"gc_run_id", gcRunID,
			"count", len(memoriesToDelete),
			"stale", len(staleMemories),
			"low_importance", len(lowImportanceMemories),
			"over_capacity", len(overCapacityMemories),
		)
	}

	return result, nil
}

// Preview returns a preview of what would be deleted without actually deleting.
func (s *GCService) Preview(ctx context.Context, tenantID string) (*domain.GCPreview, error) {
	preview := &domain.GCPreview{
		TenantID:       tenantID,
		StaleThreshold: s.config.StaleThreshold,
		MaxMemories:    s.config.MaxMemoriesPerTenant,
	}

	// Get current memory count
	count, err := s.memories.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count memories: %w", err)
	}
	preview.CurrentMemoryCount = count

	// Find stale memories
	staleThreshold := time.Now().Add(-s.config.StaleThreshold)
	staleMemories, err := s.memories.FindStaleMemories(ctx, staleThreshold, s.config.LowConfidenceThreshold, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("find stale memories: %w", err)
	}
	preview.StaleMemories = staleMemories

	// Find low importance memories
	lowImportanceMemories, err := s.memories.FindLowImportanceMemories(ctx, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("find low importance memories: %w", err)
	}
	preview.LowImportance = lowImportanceMemories

	// Find over capacity memories
	overCapacityMemories, err := s.memories.FindOverCapacityMemories(ctx, s.config.MaxMemoriesPerTenant, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("find over capacity memories: %w", err)
	}
	preview.OverCapacity = overCapacityMemories

	// Calculate total to delete (avoiding duplicates)
	allMemories := s.mergeMemories(staleMemories, lowImportanceMemories, overCapacityMemories)
	preview.TotalToDelete = len(allMemories)

	return preview, nil
}

// mergeMemories combines multiple memory slices, removing duplicates.
func (s *GCService) mergeMemories(slices ...[]domain.Memory) []domain.Memory {
	seen := make(map[string]bool)
	var result []domain.Memory

	for _, slice := range slices {
		for _, m := range slice {
			if !seen[m.ID] {
				seen[m.ID] = true
				result = append(result, m)
			}
		}
	}

	return result
}

// createSnapshot creates a backup snapshot before deletion.
func (s *GCService) createSnapshot(ctx context.Context, gcRunID, tenantID string, memories []domain.Memory) (string, error) {
	snapshotID := uuid.New().String()

	// Get all memories for backup (not just the ones being deleted)
	// This provides a complete recovery point
	allMemories, err := s.memories.GetAllForSnapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("get all memories for snapshot: %w", err)
	}

	// Marshal to JSON
	snapshotData, err := json.Marshal(allMemories)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot data: %w", err)
	}

	// Calculate expiration time
	expiresAt := time.Now().AddDate(0, 0, s.config.SnapshotRetentionDays)

	// Create snapshot
	snapshot := &domain.MemoryGCSnapshot{
		SnapshotID:   snapshotID,
		GCRunID:      gcRunID,
		TenantID:     tenantID,
		MemoryCount:  len(allMemories),
		SnapshotData: string(snapshotData),
		ExpiresAt:    expiresAt,
	}

	if err := s.gcSnapshots.Create(ctx, snapshot); err != nil {
		return "", fmt.Errorf("create snapshot: %w", err)
	}

	s.logger.Info("created GC snapshot",
		"snapshot_id", snapshotID,
		"gc_run_id", gcRunID,
		"memory_count", len(allMemories),
		"expires_at", expiresAt,
	)

	return snapshotID, nil
}

// logDeletions creates audit log entries for deleted memories.
func (s *GCService) logDeletions(ctx context.Context, gcRunID, tenantID string, memories []domain.Memory, dryRun bool) error {
	logs := make([]domain.MemoryGCLog, len(memories))

	for i, m := range memories {
		// Determine deletion reason
		var reason domain.DeletionReason
		staleThreshold := time.Now().Add(-s.config.StaleThreshold)
		if m.LastAccessedAt == nil || m.LastAccessedAt.Before(staleThreshold) {
			reason = domain.DeletionReasonStale
		} else if m.ImportanceScore != nil && *m.ImportanceScore < 0.3 {
			reason = domain.DeletionReasonLowImportance
		} else {
			reason = domain.DeletionReasonCapacity
		}

		// Truncate content preview to 500 chars
		contentPreview := m.Content
		if len(contentPreview) > 500 {
			contentPreview = contentPreview[:500]
		}

		logs[i] = domain.MemoryGCLog{
			LogID:           uuid.New().String(),
			MemoryID:        m.ID,
			TenantID:        tenantID,
			ContentPreview:  contentPreview,
			Source:          m.Source,
			MemoryType:      m.MemoryType,
			State:           m.State,
			Confidence:      m.Confidence,
			AccessCount:     m.AccessCount,
			LastAccessedAt:  m.LastAccessedAt,
			ImportanceScore: m.ImportanceScore,
			DeletionReason:  reason,
			GCRunID:         gcRunID,
		}
	}

	if err := s.gcLogs.BatchCreate(ctx, logs); err != nil {
		return fmt.Errorf("create gc logs: %w", err)
	}

	s.logger.Info("created GC log entries",
		"gc_run_id", gcRunID,
		"count", len(logs),
		"dry_run", dryRun,
	)

	return nil
}

// CleanupExpiredSnapshots removes old snapshots past their retention period.
func (s *GCService) CleanupExpiredSnapshots(ctx context.Context) (int64, error) {
	count, err := s.gcSnapshots.DeleteExpired(ctx, time.Now())
	if err != nil {
		return 0, fmt.Errorf("delete expired snapshots: %w", err)
	}

	s.logger.Info("cleaned up expired snapshots", "count", count)
	return count, nil
}

// CleanupExpiredLogs removes old GC logs.
func (s *GCService) CleanupExpiredLogs(ctx context.Context, retentionDays int) (int64, error) {
	olderThan := time.Now().AddDate(0, 0, -retentionDays)
	count, err := s.gcLogs.DeleteExpired(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete expired logs: %w", err)
	}

	s.logger.Info("cleaned up expired GC logs", "count", count, "older_than", olderThan)
	return count, nil
}

// ListGCLogs retrieves GC logs for a tenant with pagination.
func (s *GCService) ListGCLogs(ctx context.Context, tenantID string, limit, offset int) ([]domain.MemoryGCLog, int, error) {
	return s.gcLogs.ListByTenant(ctx, tenantID, limit, offset)
}

// GetSnapshot retrieves a GC snapshot by ID.
func (s *GCService) GetSnapshot(ctx context.Context, snapshotID string) (*domain.MemoryGCSnapshot, error) {
	return s.gcSnapshots.GetByID(ctx, snapshotID)
}

