package tidb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/devioslang/memorix/server/internal/domain"
)

// ConversationSummaryRepo implements repository.ConversationSummaryRepo using TiDB.
type ConversationSummaryRepo struct {
	db *sql.DB
}

// NewConversationSummaryRepo creates a new ConversationSummaryRepo.
func NewConversationSummaryRepo(db *sql.DB) *ConversationSummaryRepo {
	return &ConversationSummaryRepo{db: db}
}

const summaryAllColumns = `summary_id, user_id, session_id, title, summary, key_topics, user_intent, created_at`

// Create inserts a new conversation summary.
func (r *ConversationSummaryRepo) Create(ctx context.Context, summary *domain.ConversationSummary) error {
	var topicsJSON []byte
	var err error
	if len(summary.KeyTopics) > 0 {
		topicsJSON, err = json.Marshal(summary.KeyTopics)
		if err != nil {
			return fmt.Errorf("marshal key_topics: %w", err)
		}
	} else {
		topicsJSON = []byte("[]")
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO conversation_summaries (summary_id, user_id, session_id, title, summary, key_topics, user_intent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`,
		summary.SummaryID, summary.UserID, summary.SessionID, summary.Title, summary.Summary, topicsJSON, summary.UserIntent,
	)
	if err != nil {
		return fmt.Errorf("create conversation summary: %w", err)
	}
	return nil
}

// GetByID retrieves a summary by its ID.
func (r *ConversationSummaryRepo) GetByID(ctx context.Context, summaryID string) (*domain.ConversationSummary, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+summaryAllColumns+` FROM conversation_summaries WHERE summary_id = ?`, summaryID,
	)
	return scanConversationSummary(row)
}

// GetByUserID retrieves all summaries for a user, ordered by created_at desc.
func (r *ConversationSummaryRepo) GetByUserID(ctx context.Context, userID string, limit int) ([]domain.ConversationSummary, error) {
	if limit <= 0 {
		limit = domain.DefaultMaxSummariesPerUser
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+summaryAllColumns+` FROM conversation_summaries WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get user summaries: %w", err)
	}
	defer rows.Close()

	return scanConversationSummaries(rows)
}

// GetBySessionID retrieves a summary by session ID.
func (r *ConversationSummaryRepo) GetBySessionID(ctx context.Context, sessionID string) (*domain.ConversationSummary, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+summaryAllColumns+` FROM conversation_summaries WHERE session_id = ?`,
		sessionID,
	)
	return scanConversationSummary(row)
}

// List retrieves summaries with filtering and pagination.
func (r *ConversationSummaryRepo) List(ctx context.Context, f domain.ConversationSummaryFilter) ([]domain.ConversationSummary, int, error) {
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
	countQuery := "SELECT COUNT(*) FROM conversation_summaries WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count summaries: %w", err)
	}

	// Fetch page.
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := "SELECT " + summaryAllColumns + " FROM conversation_summaries WHERE " +
		where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	dataArgs := make([]any, len(args), len(args)+2)
	copy(dataArgs, args)
	dataArgs = append(dataArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list summaries: %w", err)
	}
	defer rows.Close()

	summaries, err := scanConversationSummaries(rows)
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

// Delete removes a summary by ID.
func (r *ConversationSummaryRepo) Delete(ctx context.Context, summaryID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM conversation_summaries WHERE summary_id = ?`,
		summaryID,
	)
	if err != nil {
		return fmt.Errorf("delete conversation summary: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteOldest removes the oldest summaries for a user to maintain the sliding window.
// It deletes the oldest summaries first (by created_at).
// Returns the number of summaries deleted.
func (r *ConversationSummaryRepo) DeleteOldest(ctx context.Context, userID string, count int) (int64, error) {
	// Find the IDs of summaries to delete (oldest first)
	query := `
		SELECT summary_id FROM conversation_summaries
		WHERE user_id = ?
		ORDER BY created_at ASC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, userID, count)
	if err != nil {
		return 0, fmt.Errorf("find oldest summaries: %w", err)
	}
	defer rows.Close()

	var summaryIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan summary id: %w", err)
		}
		summaryIDs = append(summaryIDs, id)
	}

	if len(summaryIDs) == 0 {
		return 0, nil
	}

	// Delete by IDs
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM conversation_summaries WHERE summary_id IN (`+placeholders(len(summaryIDs))+`)`,
		stringsToArgs(summaryIDs)...,
	)
	if err != nil {
		return 0, fmt.Errorf("delete oldest summaries: %w", err)
	}
	return result.RowsAffected()
}

// CountByUserID returns the total number of summaries for a user.
func (r *ConversationSummaryRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM conversation_summaries WHERE user_id = ?`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user summaries: %w", err)
	}
	return count, nil
}

func (r *ConversationSummaryRepo) buildFilterConds(f domain.ConversationSummaryFilter) ([]string, []any) {
	conds := []string{}
	args := []any{}

	if f.UserID != "" {
		conds = append(conds, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.SessionID != "" {
		conds = append(conds, "session_id = ?")
		args = append(args, f.SessionID)
	}
	if f.KeyTopic != "" {
		conds = append(conds, "JSON_CONTAINS(key_topics, ?)")
		args = append(args, fmt.Sprintf(`"%s"`, f.KeyTopic))
	}

	return conds, args
}

func scanConversationSummary(row *sql.Row) (*domain.ConversationSummary, error) {
	var s domain.ConversationSummary
	var topicsJSON []byte
	var sessionID sql.NullString

	err := row.Scan(&s.SummaryID, &s.UserID, &sessionID, &s.Title, &s.Summary, &topicsJSON, &s.UserIntent, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan conversation summary: %w", err)
	}

	s.SessionID = sessionID.String

	// Parse key_topics JSON
	if len(topicsJSON) > 0 {
		if err := json.Unmarshal(topicsJSON, &s.KeyTopics); err != nil {
			// Log warning but continue with empty topics
			s.KeyTopics = []string{}
		}
	}

	return &s, nil
}

func scanConversationSummaries(rows *sql.Rows) ([]domain.ConversationSummary, error) {
	var summaries []domain.ConversationSummary
	for rows.Next() {
		var s domain.ConversationSummary
		var topicsJSON []byte
		var sessionID sql.NullString

		err := rows.Scan(&s.SummaryID, &s.UserID, &sessionID, &s.Title, &s.Summary, &topicsJSON, &s.UserIntent, &s.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan conversation summary row: %w", err)
		}

		s.SessionID = sessionID.String

		// Parse key_topics JSON
		if len(topicsJSON) > 0 {
			if err := json.Unmarshal(topicsJSON, &s.KeyTopics); err != nil {
				// Log warning but continue with empty topics
				s.KeyTopics = []string{}
			}
		}

		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}
