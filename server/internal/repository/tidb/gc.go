package tidb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// MemoryGCLogRepo implements repository.MemoryGCLogRepo for TiDB.
type MemoryGCLogRepo struct {
	db *sql.DB
}

// NewMemoryGCLogRepo creates a new MemoryGCLogRepo.
func NewMemoryGCLogRepo(db *sql.DB) *MemoryGCLogRepo {
	return &MemoryGCLogRepo{db: db}
}

// Create stores a new GC log entry.
func (r *MemoryGCLogRepo) Create(ctx context.Context, log *domain.MemoryGCLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO memory_gc_logs
		 (log_id, memory_id, tenant_id, content_preview, source, memory_type, state,
		  confidence, access_count, last_accessed_at, importance_score, deletion_reason, gc_run_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		log.LogID, log.MemoryID, log.TenantID, log.ContentPreview, log.Source, log.MemoryType, log.State,
		log.Confidence, log.AccessCount, log.LastAccessedAt, log.ImportanceScore, log.DeletionReason, log.GCRunID,
	)
	if err != nil {
		return fmt.Errorf("create gc log: %w", err)
	}
	return nil
}

// BatchCreate stores multiple GC log entries in a single transaction.
func (r *MemoryGCLogRepo) BatchCreate(ctx context.Context, logs []domain.MemoryGCLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO memory_gc_logs
		 (log_id, memory_id, tenant_id, content_preview, source, memory_type, state,
		  confidence, access_count, last_accessed_at, importance_score, deletion_reason, gc_run_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
	)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, log := range logs {
		_, err := stmt.ExecContext(ctx,
			log.LogID, log.MemoryID, log.TenantID, log.ContentPreview, log.Source, log.MemoryType, log.State,
			log.Confidence, log.AccessCount, log.LastAccessedAt, log.ImportanceScore, log.DeletionReason, log.GCRunID,
		)
		if err != nil {
			return fmt.Errorf("insert gc log %s: %w", log.LogID, err)
		}
	}

	return tx.Commit()
}

// GetByGCRunID retrieves all logs for a specific GC run.
func (r *MemoryGCLogRepo) GetByGCRunID(ctx context.Context, gcRunID string) ([]domain.MemoryGCLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT log_id, memory_id, tenant_id, content_preview, source, memory_type, state,
		        confidence, access_count, last_accessed_at, importance_score, deletion_reason, gc_run_id, created_at
		 FROM memory_gc_logs
		 WHERE gc_run_id = ?
		 ORDER BY created_at DESC`,
		gcRunID,
	)
	if err != nil {
		return nil, fmt.Errorf("get by gc run id: %w", err)
	}
	defer rows.Close()

	var logs []domain.MemoryGCLog
	for rows.Next() {
		log, err := scanGCLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *log)
	}
	return logs, rows.Err()
}

// ListByTenant retrieves GC logs for a tenant with pagination.
func (r *MemoryGCLogRepo) ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]domain.MemoryGCLog, int, error) {
	// Count total
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_gc_logs WHERE tenant_id = ?`,
		tenantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count gc logs: %w", err)
	}

	// Fetch page
	rows, err := r.db.QueryContext(ctx,
		`SELECT log_id, memory_id, tenant_id, content_preview, source, memory_type, state,
		        confidence, access_count, last_accessed_at, importance_score, deletion_reason, gc_run_id, created_at
		 FROM memory_gc_logs
		 WHERE tenant_id = ?
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list gc logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.MemoryGCLog
	for rows.Next() {
		log, err := scanGCLog(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, *log)
	}
	return logs, total, rows.Err()
}

// DeleteExpired removes GC logs older than the retention period.
func (r *MemoryGCLogRepo) DeleteExpired(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM memory_gc_logs WHERE created_at < ?`,
		olderThan,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired gc logs: %w", err)
	}
	return result.RowsAffected()
}

func scanGCLog(rows *sql.Rows) (*domain.MemoryGCLog, error) {
	var log domain.MemoryGCLog
	var source, memoryType sql.NullString
	var contentPreview sql.NullString
	var confidence, importanceScore sql.NullFloat64
	var lastAccessedAt sql.NullTime
	var accessCount sql.NullInt64

	err := rows.Scan(
		&log.LogID, &log.MemoryID, &log.TenantID, &contentPreview, &source, &memoryType, &log.State,
		&confidence, &accessCount, &lastAccessedAt, &importanceScore, &log.DeletionReason, &log.GCRunID, &log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan gc log: %w", err)
	}

	log.ContentPreview = contentPreview.String
	log.Source = source.String
	log.MemoryType = domain.MemoryType(memoryType.String)
	if confidence.Valid {
		log.Confidence = confidence.Float64
	}
	if accessCount.Valid {
		log.AccessCount = int(accessCount.Int64)
	}
	if lastAccessedAt.Valid {
		log.LastAccessedAt = &lastAccessedAt.Time
	}
	if importanceScore.Valid {
		log.ImportanceScore = &importanceScore.Float64
	}

	return &log, nil
}

// MemoryGCSnapshotRepo implements repository.MemoryGCSnapshotRepo for TiDB.
type MemoryGCSnapshotRepo struct {
	db *sql.DB
}

// NewMemoryGCSnapshotRepo creates a new MemoryGCSnapshotRepo.
func NewMemoryGCSnapshotRepo(db *sql.DB) *MemoryGCSnapshotRepo {
	return &MemoryGCSnapshotRepo{db: db}
}

// Create stores a new GC snapshot.
func (r *MemoryGCSnapshotRepo) Create(ctx context.Context, snapshot *domain.MemoryGCSnapshot) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO memory_gc_snapshots
		 (snapshot_id, gc_run_id, tenant_id, memory_count, snapshot_data, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, NOW(), ?)`,
		snapshot.SnapshotID, snapshot.GCRunID, snapshot.TenantID, snapshot.MemoryCount, snapshot.SnapshotData, snapshot.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create gc snapshot: %w", err)
	}
	return nil
}

// GetByID retrieves a snapshot by its ID.
func (r *MemoryGCSnapshotRepo) GetByID(ctx context.Context, snapshotID string) (*domain.MemoryGCSnapshot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT snapshot_id, gc_run_id, tenant_id, memory_count, snapshot_data, created_at, expires_at
		 FROM memory_gc_snapshots
		 WHERE snapshot_id = ?`,
		snapshotID,
	)

	var snapshot domain.MemoryGCSnapshot
	var snapshotData []byte
	err := row.Scan(
		&snapshot.SnapshotID, &snapshot.GCRunID, &snapshot.TenantID, &snapshot.MemoryCount,
		&snapshotData, &snapshot.CreatedAt, &snapshot.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gc snapshot: %w", err)
	}
	snapshot.SnapshotData = string(snapshotData)
	return &snapshot, nil
}

// GetByGCRunID retrieves the snapshot for a specific GC run.
func (r *MemoryGCSnapshotRepo) GetByGCRunID(ctx context.Context, gcRunID string) (*domain.MemoryGCSnapshot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT snapshot_id, gc_run_id, tenant_id, memory_count, snapshot_data, created_at, expires_at
		 FROM memory_gc_snapshots
		 WHERE gc_run_id = ?`,
		gcRunID,
	)

	var snapshot domain.MemoryGCSnapshot
	var snapshotData []byte
	err := row.Scan(
		&snapshot.SnapshotID, &snapshot.GCRunID, &snapshot.TenantID, &snapshot.MemoryCount,
		&snapshotData, &snapshot.CreatedAt, &snapshot.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gc snapshot by run id: %w", err)
	}
	snapshot.SnapshotData = string(snapshotData)
	return &snapshot, nil
}

// DeleteExpired removes snapshots past their expiration date.
func (r *MemoryGCSnapshotRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM memory_gc_snapshots WHERE expires_at < ?`,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired gc snapshots: %w", err)
	}
	return result.RowsAffected()
}

// Helper function to marshal memories to JSON for snapshot storage
func marshalMemoriesForSnapshot(memories []domain.Memory) (string, error) {
	// Create a simplified representation for snapshot
	type snapshotMemory struct {
		ID              string                 `json:"id"`
		Content         string                 `json:"content"`
		MemoryType      domain.MemoryType      `json:"memory_type"`
		Source          string                 `json:"source,omitempty"`
		Tags            []string               `json:"tags,omitempty"`
		Metadata        json.RawMessage        `json:"metadata,omitempty"`
		AgentID         string                 `json:"agent_id,omitempty"`
		SessionID       string                 `json:"session_id,omitempty"`
		State           domain.MemoryState     `json:"state"`
		Version         int                    `json:"version"`
		CreatedAt       time.Time              `json:"created_at"`
		UpdatedAt       time.Time              `json:"updated_at"`
		Confidence      float64                `json:"confidence,omitempty"`
		AccessCount     int                    `json:"access_count,omitempty"`
		LastAccessedAt  *time.Time             `json:"last_accessed_at,omitempty"`
		ImportanceScore *float64               `json:"importance_score,omitempty"`
	}

	snapshotMemories := make([]snapshotMemory, len(memories))
	for i, m := range memories {
		snapshotMemories[i] = snapshotMemory{
			ID:              m.ID,
			Content:         m.Content,
			MemoryType:      m.MemoryType,
			Source:          m.Source,
			Tags:            m.Tags,
			Metadata:        m.Metadata,
			AgentID:         m.AgentID,
			SessionID:       m.SessionID,
			State:           m.State,
			Version:         m.Version,
			CreatedAt:       m.CreatedAt,
			UpdatedAt:       m.UpdatedAt,
			Confidence:      m.Confidence,
			AccessCount:     m.AccessCount,
			LastAccessedAt:  m.LastAccessedAt,
			ImportanceScore: m.ImportanceScore,
		}
	}

	data, err := json.Marshal(snapshotMemories)
	if err != nil {
		return "", fmt.Errorf("marshal memories for snapshot: %w", err)
	}
	return string(data), nil
}
