package tidb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/devioslang/memorix/server/internal/domain"
)

// UserProfileFactRepo implements repository.UserProfileFactRepo using TiDB.
type UserProfileFactRepo struct {
	db *sql.DB
}

// NewUserProfileFactRepo creates a new UserProfileFactRepo.
func NewUserProfileFactRepo(db *sql.DB) *UserProfileFactRepo {
	return &UserProfileFactRepo{db: db}
}

const factAllColumns = `fact_id, user_id, category, key, value, source, confidence, created_at, updated_at, last_accessed_at`

// Create inserts a new user profile fact.
func (r *UserProfileFactRepo) Create(ctx context.Context, fact *domain.UserProfileFact) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_profile_facts (fact_id, user_id, category, key, value, source, confidence, created_at, updated_at, last_accessed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NOW())`,
		fact.FactID, fact.UserID, string(fact.Category), fact.Key, fact.Value, string(fact.Source), fact.Confidence,
	)
	if err != nil {
		return fmt.Errorf("create user profile fact: %w", err)
	}
	return nil
}

// GetByID retrieves a fact by its ID.
func (r *UserProfileFactRepo) GetByID(ctx context.Context, factID string) (*domain.UserProfileFact, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+factAllColumns+` FROM user_profile_facts WHERE fact_id = ?`, factID,
	)
	return scanUserProfileFact(row)
}

// GetByUserID retrieves all facts for a user.
func (r *UserProfileFactRepo) GetByUserID(ctx context.Context, userID string) ([]domain.UserProfileFact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+factAllColumns+` FROM user_profile_facts WHERE user_id = ? ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user facts: %w", err)
	}
	defer rows.Close()

	return scanUserProfileFacts(rows)
}

// GetByUserIDAndCategory retrieves facts for a user filtered by category.
func (r *UserProfileFactRepo) GetByUserIDAndCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+factAllColumns+` FROM user_profile_facts WHERE user_id = ? AND category = ? ORDER BY updated_at DESC`,
		userID, string(category),
	)
	if err != nil {
		return nil, fmt.Errorf("get user facts by category: %w", err)
	}
	defer rows.Close()

	return scanUserProfileFacts(rows)
}

// List retrieves facts with filtering and pagination.
func (r *UserProfileFactRepo) List(ctx context.Context, f domain.UserProfileFactFilter) ([]domain.UserProfileFact, int, error) {
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
	countQuery := "SELECT COUNT(*) FROM user_profile_facts WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count facts: %w", err)
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

	dataQuery := "SELECT " + factAllColumns + " FROM user_profile_facts WHERE " +
		where + " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	dataArgs := make([]any, len(args), len(args)+2)
	copy(dataArgs, args)
	dataArgs = append(dataArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list facts: %w", err)
	}
	defer rows.Close()

	facts, err := scanUserProfileFacts(rows)
	if err != nil {
		return nil, 0, err
	}
	return facts, total, nil
}

// Update updates an existing fact.
func (r *UserProfileFactRepo) Update(ctx context.Context, fact *domain.UserProfileFact) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_profile_facts SET category = ?, key = ?, value = ?, source = ?, confidence = ?, updated_at = NOW()
		 WHERE fact_id = ?`,
		string(fact.Category), fact.Key, fact.Value, string(fact.Source), fact.Confidence, fact.FactID,
	)
	if err != nil {
		return fmt.Errorf("update user profile fact: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete removes a fact by ID.
func (r *UserProfileFactRepo) Delete(ctx context.Context, factID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM user_profile_facts WHERE fact_id = ?`,
		factID,
	)
	if err != nil {
		return fmt.Errorf("delete user profile fact: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteByUserID removes all facts for a user.
func (r *UserProfileFactRepo) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM user_profile_facts WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete user facts: %w", err)
	}
	return nil
}

// CountByUserID returns the total number of facts for a user.
func (r *UserProfileFactRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_profile_facts WHERE user_id = ?`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user facts: %w", err)
	}
	return count, nil
}

// DeleteOldestLowConfidence deletes facts that are oldest and have lowest confidence.
// It prioritizes: oldest last_accessed_at, then lowest confidence.
// Returns number of facts deleted.
func (r *UserProfileFactRepo) DeleteOldestLowConfidence(ctx context.Context, userID string, count int) (int64, error) {
	// Find the IDs of facts to delete (oldest accessed + lowest confidence)
	query := `
		SELECT fact_id FROM user_profile_facts
		WHERE user_id = ?
		ORDER BY last_accessed_at ASC, confidence ASC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, userID, count)
	if err != nil {
		return 0, fmt.Errorf("find oldest low confidence facts: %w", err)
	}
	defer rows.Close()

	var factIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan fact id: %w", err)
		}
		factIDs = append(factIDs, id)
	}

	if len(factIDs) == 0 {
		return 0, nil
	}

	// Delete by IDs
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM user_profile_facts WHERE fact_id IN (`+placeholders(len(factIDs))+`)`,
		stringsToArgs(factIDs)...,
	)
	if err != nil {
		return 0, fmt.Errorf("delete oldest low confidence facts: %w", err)
	}
	return result.RowsAffected()
}

// TouchLastAccessed updates the last_accessed_at timestamp for a fact.
func (r *UserProfileFactRepo) TouchLastAccessed(ctx context.Context, factID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_profile_facts SET last_accessed_at = NOW() WHERE fact_id = ?`,
		factID,
	)
	if err != nil {
		return fmt.Errorf("touch last accessed: %w", err)
	}
	return nil
}

// GetByKey retrieves a fact by user_id, category, and key.
// Returns ErrNotFound if no matching fact exists.
func (r *UserProfileFactRepo) GetByKey(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+factAllColumns+` FROM user_profile_facts WHERE user_id = ? AND category = ? AND `+"`key`"+` = ?`,
		userID, string(category), key,
	)
	return scanUserProfileFact(row)
}

// SearchByValue performs a fuzzy search for facts with similar values.
// Uses LIKE for basic text similarity matching.
// Returns facts where the value contains similar text (used for deduplication).
func (r *UserProfileFactRepo) SearchByValue(ctx context.Context, userID string, value string, limit int) ([]domain.UserProfileFact, error) {
	if limit <= 0 {
		limit = 10
	}

	// Use LIKE with wildcards for basic fuzzy matching
	// Also search for facts where the value contains key terms from the input
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+factAllColumns+` FROM user_profile_facts
		 WHERE user_id = ? AND (
		   value LIKE ? OR
		   ? LIKE CONCAT('%', value, '%')
		 )
		 ORDER BY updated_at DESC
		 LIMIT ?`,
		userID,
		"%"+value+"%",
		value,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search user facts by value: %w", err)
	}
	defer rows.Close()

	return scanUserProfileFacts(rows)
}

func (r *UserProfileFactRepo) buildFilterConds(f domain.UserProfileFactFilter) ([]string, []any) {
	conds := []string{}
	args := []any{}

	if f.UserID != "" {
		conds = append(conds, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, string(f.Category))
	}
	if f.Key != "" {
		conds = append(conds, "`key` = ?")
		args = append(args, f.Key)
	}
	if f.Source != "" {
		conds = append(conds, "source = ?")
		args = append(args, string(f.Source))
	}

	return conds, args
}

func scanUserProfileFact(row *sql.Row) (*domain.UserProfileFact, error) {
	var f domain.UserProfileFact
	var category, source sql.NullString

	err := row.Scan(&f.FactID, &f.UserID, &category, &f.Key, &f.Value, &source, &f.Confidence,
		&f.CreatedAt, &f.UpdatedAt, &f.LastAccessedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user profile fact: %w", err)
	}

	f.Category = domain.FactCategory(category.String)
	f.Source = domain.FactSource(source.String)
	return &f, nil
}

func scanUserProfileFacts(rows *sql.Rows) ([]domain.UserProfileFact, error) {
	var facts []domain.UserProfileFact
	for rows.Next() {
		var f domain.UserProfileFact
		var category, source sql.NullString

		err := rows.Scan(&f.FactID, &f.UserID, &category, &f.Key, &f.Value, &source, &f.Confidence,
			&f.CreatedAt, &f.UpdatedAt, &f.LastAccessedAt)
		if err != nil {
			return nil, fmt.Errorf("scan user profile fact row: %w", err)
		}

		f.Category = domain.FactCategory(category.String)
		f.Source = domain.FactSource(source.String)
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

// placeholders generates n SQL placeholders separated by commas.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	result := "?"
	for i := 1; i < n; i++ {
		result += ",?"
	}
	return result
}

// stringsToArgs converts a string slice to []any for use in SQL queries.
func stringsToArgs(s []string) []any {
	args := make([]any, len(s))
	for i, v := range s {
		args[i] = v
	}
	return args
}
