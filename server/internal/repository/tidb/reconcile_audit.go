package tidb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/devioslang/memorix/server/internal/domain"
)

// ReconcileAuditRepo implements repository.ReconcileAuditRepo using TiDB.
type ReconcileAuditRepo struct {
	db *sql.DB
}

// NewReconcileAuditRepo creates a new ReconcileAuditRepo.
func NewReconcileAuditRepo(db *sql.DB) *ReconcileAuditRepo {
	return &ReconcileAuditRepo{db: db}
}

const auditLogColumns = `log_id, user_id, fact_id, category, key, old_value, new_value, decision, reason, source, created_at`

// Create inserts a new reconciliation audit log.
func (r *ReconcileAuditRepo) Create(ctx context.Context, log *domain.ReconcileAuditLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO reconcile_audit_logs (log_id, user_id, fact_id, category, key, old_value, new_value, decision, reason, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.LogID, log.UserID, log.FactID, string(log.Category), log.Key, log.OldValue, log.NewValue,
		string(log.Decision), log.Reason, log.Source, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create reconcile audit log: %w", err)
	}
	return nil
}

// GetByID retrieves an audit log by its ID.
func (r *ReconcileAuditRepo) GetByID(ctx context.Context, logID string) (*domain.ReconcileAuditLog, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+auditLogColumns+` FROM reconcile_audit_logs WHERE log_id = ?`, logID,
	)
	return scanReconcileAuditLog(row)
}

// ListByUserID retrieves all audit logs for a user, ordered by created_at desc.
func (r *ReconcileAuditRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]domain.ReconcileAuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+auditLogColumns+` FROM reconcile_audit_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit logs by user: %w", err)
	}
	defer rows.Close()

	return scanReconcileAuditLogs(rows)
}

// ListByFactID retrieves all audit logs for a specific fact, ordered by created_at desc.
func (r *ReconcileAuditRepo) ListByFactID(ctx context.Context, factID string, limit, offset int) ([]domain.ReconcileAuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+auditLogColumns+` FROM reconcile_audit_logs WHERE fact_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		factID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit logs by fact: %w", err)
	}
	defer rows.Close()

	return scanReconcileAuditLogs(rows)
}

// List returns audit logs based on filter criteria.
func (r *ReconcileAuditRepo) List(ctx context.Context, f domain.ReconcileAuditFilter) ([]domain.ReconcileAuditLog, int, error) {
	conds, args := r.buildFilterConds(f)
	where := "1=1"
	if len(conds) > 0 {
		where = ""
		for i, cond := range conds {
			if i > 0 {
				where += " AND "
			}
			where += cond
		}
	}

	// Count total matches.
	var total int
	countQuery := "SELECT COUNT(*) FROM reconcile_audit_logs WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Fetch page.
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := "SELECT " + auditLogColumns + " FROM reconcile_audit_logs WHERE " +
		where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	dataArgs := make([]any, len(args), len(args)+2)
	copy(dataArgs, args)
	dataArgs = append(dataArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	logs, err := scanReconcileAuditLogs(rows)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// DeleteByUserID deletes all audit logs for a user.
func (r *ReconcileAuditRepo) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM reconcile_audit_logs WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete audit logs by user: %w", err)
	}
	return nil
}

func (r *ReconcileAuditRepo) buildFilterConds(f domain.ReconcileAuditFilter) ([]string, []any) {
	conds := []string{}
	args := []any{}

	if f.UserID != "" {
		conds = append(conds, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.FactID != "" {
		conds = append(conds, "fact_id = ?")
		args = append(args, f.FactID)
	}
	if f.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, string(f.Category))
	}

	return conds, args
}

func scanReconcileAuditLog(row *sql.Row) (*domain.ReconcileAuditLog, error) {
	var log domain.ReconcileAuditLog
	var category, decision sql.NullString

	err := row.Scan(&log.LogID, &log.UserID, &log.FactID, &category, &log.Key,
		&log.OldValue, &log.NewValue, &decision, &log.Reason, &log.Source, &log.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan reconcile audit log: %w", err)
	}

	log.Category = domain.FactCategory(category.String)
	log.Decision = domain.ReconcileDecision(decision.String)
	return &log, nil
}

func scanReconcileAuditLogs(rows *sql.Rows) ([]domain.ReconcileAuditLog, error) {
	var logs []domain.ReconcileAuditLog
	for rows.Next() {
		var log domain.ReconcileAuditLog
		var category, decision sql.NullString

		err := rows.Scan(&log.LogID, &log.UserID, &log.FactID, &category, &log.Key,
			&log.OldValue, &log.NewValue, &decision, &log.Reason, &log.Source, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan reconcile audit log row: %w", err)
		}

		log.Category = domain.FactCategory(category.String)
		log.Decision = domain.ReconcileDecision(decision.String)
		logs = append(logs, log)
	}
	return logs, rows.Err()
}
