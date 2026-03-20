package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/devioslang/memorix/server/internal/tenant"
	"github.com/go-sql-driver/mysql"
)

const tenantMemorySchemaBase = `CREATE TABLE IF NOT EXISTS memories (
	    id              VARCHAR(36)     PRIMARY KEY,
	    content         TEXT            NOT NULL,
	    source          VARCHAR(100),
	    tags            JSON,
	    metadata        JSON,
	    %s
	    memory_type     VARCHAR(20)     NOT NULL DEFAULT 'pinned',
	    agent_id        VARCHAR(100)    NULL,
	    session_id      VARCHAR(100)    NULL,
	    state           VARCHAR(20)     NOT NULL DEFAULT 'active',
	    version         INT             DEFAULT 1,
	    updated_by      VARCHAR(100),
	    superseded_by   VARCHAR(36)     NULL,
	    created_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
	    updated_at      TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	    INDEX idx_memory_type         (memory_type),
	    INDEX idx_source              (source),
	    INDEX idx_state               (state),
	    INDEX idx_agent               (agent_id),
	    INDEX idx_session             (session_id),
	    INDEX idx_updated             (updated_at)
	)`

const tenantUserProfileFactsSchema = `CREATE TABLE IF NOT EXISTS user_profile_facts (
	    fact_id           VARCHAR(36)     PRIMARY KEY,
	    user_id           VARCHAR(100)    NOT NULL,
	    category          VARCHAR(20)     NOT NULL COMMENT 'personal|preference|goal|skill',
	    ` + "`key`" + `             VARCHAR(100)    NOT NULL,
	    value             TEXT            NOT NULL,
	    source            VARCHAR(20)     NOT NULL COMMENT 'explicit|inferred',
	    confidence        DECIMAL(3,2)    NOT NULL DEFAULT 1.0 COMMENT '0.00-1.00, confidence for inferred facts',
	    created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
	    updated_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	    last_accessed_at  TIMESTAMP       DEFAULT CURRENT_TIMESTAMP COMMENT 'Used for capacity-based cleanup',
	    INDEX idx_user_facts (user_id),
	    INDEX idx_user_category (user_id, category),
	    INDEX idx_user_key (user_id, ` + "`key`" + `),
	    INDEX idx_accessed_confidence (user_id, last_accessed_at, confidence)
	)`

const tenantReconcileAuditLogsSchema = `CREATE TABLE IF NOT EXISTS reconcile_audit_logs (
	    log_id            VARCHAR(36)     PRIMARY KEY,
	    user_id           VARCHAR(100)    NOT NULL,
	    fact_id           VARCHAR(36)     NOT NULL COMMENT 'The fact ID that was reconciled',
	    category          VARCHAR(20)     NOT NULL COMMENT 'personal|preference|goal|skill',
	    ` + "`key`" + `             VARCHAR(100)    NOT NULL,
	    old_value         TEXT            NULL COMMENT 'Previous value (empty for new facts)',
	    new_value         TEXT            NOT NULL COMMENT 'Incoming value',
	    decision          VARCHAR(20)     NOT NULL COMMENT 'UPDATE|APPEND|IGNORE',
	    reason            TEXT            NULL COMMENT 'LLM explanation for the decision',
	    source            VARCHAR(100)    NULL COMMENT 'Agent name that provided the new fact',
	    created_at        TIMESTAMP       DEFAULT CURRENT_TIMESTAMP,
	    INDEX idx_user_audit (user_id, created_at DESC),
	    INDEX idx_fact_audit (fact_id, created_at DESC),
	    INDEX idx_category_audit (user_id, category, created_at DESC)
	)`

func buildMemorySchema(autoModel string, autoDims int) string {
	var embeddingCol string
	if autoModel != "" {
		dims := strconv.Itoa(autoDims)
		embeddingCol = `embedding VECTOR(` + dims + `) GENERATED ALWAYS AS (EMBED_TEXT('` + autoModel + `', content)) STORED,`
	} else {
		embeddingCol = `embedding VECTOR(1536) NULL,`
	}
	return fmt.Sprintf(tenantMemorySchemaBase, embeddingCol)
}

type TenantService struct {
	tenants    repository.TenantRepo
	zero       *tenant.ZeroClient
	pool       *tenant.TenantPool
	logger     *slog.Logger
	autoModel  string
	autoDims   int
	ftsEnabled bool
}

func NewTenantService(
	tenants repository.TenantRepo,
	zero *tenant.ZeroClient,
	pool *tenant.TenantPool,
	logger *slog.Logger,
	autoModel string,
	autoDims int,
	ftsEnabled bool,
) *TenantService {
	return &TenantService{
		tenants:    tenants,
		zero:       zero,
		pool:       pool,
		logger:     logger,
		autoModel:  autoModel,
		autoDims:   autoDims,
		ftsEnabled: ftsEnabled,
	}
}

// ProvisionResult is the output of Provision.
type ProvisionResult struct {
	ID string `json:"id"`
}

// Provision creates a new TiDB Zero instance and registers it as a tenant.
// The TiDB Zero instance ID is used as the tenant ID.
func (s *TenantService) Provision(ctx context.Context) (*ProvisionResult, error) {
	if s.zero == nil {
		return nil, &domain.ValidationError{Message: "provisioning disabled (TiDB Zero not configured)"}
	}

	instance, err := s.zero.CreateInstance(ctx, "memorix")
	if err != nil {
		return nil, fmt.Errorf("provision TiDB Zero instance: %w", err)
	}

	// Use the TiDB Zero instance ID as the tenant ID.
	tenantID := instance.ID

	t := &domain.Tenant{
		ID:             tenantID,
		Name:           tenantID, // Use ID as name for auto-provisioned tenants.
		DBHost:         instance.Host,
		DBPort:         instance.Port,
		DBUser:         instance.Username,
		DBPassword:     instance.Password,
		DBName:         "test",
		DBTLS:          true,
		Provider:       "tidb_zero",
		ClusterID:      instance.ID,
		ClaimURL:       instance.ClaimURL,
		ClaimExpiresAt: instance.ClaimExpiresAt,
		Status:         domain.TenantProvisioning,
		SchemaVersion:  0,
	}
	if err := s.tenants.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create tenant record: %w", err)
	}

	if err := s.initSchema(ctx, t); err != nil {
		if s.logger != nil {
			s.logger.Error("tenant schema init failed", "tenant_id", tenantID, "err", err)
		}
		return nil, fmt.Errorf("init tenant schema: %w", err)
	}

	if err := s.tenants.UpdateStatus(ctx, tenantID, domain.TenantActive); err != nil {
		return nil, fmt.Errorf("activate tenant: %w", err)
	}
	if err := s.tenants.UpdateSchemaVersion(ctx, tenantID, 1); err != nil {
		return nil, fmt.Errorf("update schema version: %w", err)
	}

	return &ProvisionResult{
		ID: tenantID,
	}, nil
}

// GetInfo returns tenant info including agent and memory counts.
func (s *TenantService) GetInfo(ctx context.Context, tenantID string) (*domain.TenantInfo, error) {
	t, err := s.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if s.pool == nil {
		return nil, fmt.Errorf("tenant pool not configured")
	}
	db, err := s.pool.Get(ctx, tenantID, t.DSN())
	if err != nil {
		return nil, err
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memories").Scan(&count); err != nil {
		return nil, err
	}

	return &domain.TenantInfo{
		TenantID:    t.ID,
		Name:        t.Name,
		Status:      t.Status,
		Provider:    t.Provider,
		MemoryCount: count,
		CreatedAt:   t.CreatedAt,
	}, nil
}

func (s *TenantService) initSchema(ctx context.Context, t *domain.Tenant) error {
	if s.pool == nil {
		return fmt.Errorf("tenant pool not configured")
	}
	db, err := s.pool.Get(ctx, t.ID, t.DSN())
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, buildMemorySchema(s.autoModel, s.autoDims)); err != nil {
		return fmt.Errorf("init tenant schema: memories: %w", err)
	}
	if s.autoModel != "" {
		_, err := db.ExecContext(ctx,
			`ALTER TABLE memories ADD VECTOR INDEX idx_cosine ((VEC_COSINE_DISTANCE(embedding))) ADD_COLUMNAR_REPLICA_ON_DEMAND`)
		if err != nil && !isIndexExistsError(err) {
			return fmt.Errorf("init tenant schema: vector index: %w", err)
		}
	}
	if s.ftsEnabled {
		_, err := db.ExecContext(ctx,
			`ALTER TABLE memories ADD FULLTEXT INDEX idx_fts_content (content) WITH PARSER MULTILINGUAL ADD_COLUMNAR_REPLICA_ON_DEMAND`)
		if err != nil && !isIndexExistsError(err) {
			return fmt.Errorf("init tenant schema: fulltext index: %w", err)
		}
	}
	// Create user_profile_facts table for structured user profile storage.
	if _, err := db.ExecContext(ctx, tenantUserProfileFactsSchema); err != nil {
		return fmt.Errorf("init tenant schema: user_profile_facts: %w", err)
	}
	// Create reconcile_audit_logs table for LLM reconciliation audit trail.
	if _, err := db.ExecContext(ctx, tenantReconcileAuditLogsSchema); err != nil {
		return fmt.Errorf("init tenant schema: reconcile_audit_logs: %w", err)
	}
	return nil
}

func isIndexExistsError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1061
	}
	return false
}
