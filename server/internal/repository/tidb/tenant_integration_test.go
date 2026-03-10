//go:build integration

package tidb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/devioslang/memorix/server/internal/domain"
)

func TestTenantCreate(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	tenant := newTestTenant()
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != tenant.Name {
		t.Fatalf("name mismatch: got %q want %q", got.Name, tenant.Name)
	}
	if got.DBHost != tenant.DBHost {
		t.Fatalf("db_host mismatch: got %q want %q", got.DBHost, tenant.DBHost)
	}
	if got.Provider != tenant.Provider {
		t.Fatalf("provider mismatch: got %q want %q", got.Provider, tenant.Provider)
	}
	if got.Status != domain.TenantProvisioning {
		t.Fatalf("status mismatch: got %q want %q", got.Status, domain.TenantProvisioning)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("schema_version mismatch: got %d want 1", got.SchemaVersion)
	}
}

func TestTenantCreateDuplicateName(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	name := "unique-" + uuid.New().String()[:8]
	t1 := newTestTenant(func(t *domain.Tenant) { t.Name = name })
	if err := repo.Create(ctx, t1); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	t2 := newTestTenant(func(t *domain.Tenant) { t.Name = name })
	if err := repo.Create(ctx, t2); err == nil {
		t.Fatal("expected error on duplicate name")
	}
}

func TestTenantGetByName(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	tenant := newTestTenant()
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByName(ctx, tenant.Name)
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.ID != tenant.ID {
		t.Fatalf("ID mismatch: got %q want %q", got.ID, tenant.ID)
	}
}

func TestTenantGetByNameDeleted(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	tenant := newTestTenant()
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Mark as deleted.
	if err := repo.UpdateStatus(ctx, tenant.ID, domain.TenantDeleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// GetByName filters out deleted.
	_, err := repo.GetByName(ctx, tenant.Name)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for deleted tenant, got %v", err)
	}
}

func TestTenantGetByIDNotFound(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTenantUpdateStatus(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	tenant := newTestTenant()
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	statuses := []domain.TenantStatus{
		domain.TenantActive,
		domain.TenantSuspended,
		domain.TenantDeleted,
	}
	for _, s := range statuses {
		if err := repo.UpdateStatus(ctx, tenant.ID, s); err != nil {
			t.Fatalf("UpdateStatus(%s): %v", s, err)
		}
		got, err := repo.GetByID(ctx, tenant.ID)
		if err != nil {
			t.Fatalf("GetByID after UpdateStatus: %v", err)
		}
		if got.Status != s {
			t.Fatalf("status mismatch: got %q want %q", got.Status, s)
		}
	}
}

func TestTenantUpdateSchemaVersion(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantRepo(testDB)
	ctx := context.Background()

	tenant := newTestTenant()
	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateSchemaVersion(ctx, tenant.ID, 5); err != nil {
		t.Fatalf("UpdateSchemaVersion: %v", err)
	}

	got, err := repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.SchemaVersion != 5 {
		t.Fatalf("schema_version mismatch: got %d want 5", got.SchemaVersion)
	}
}

// --- TenantToken tests ---

func TestTokenCreate(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantTokenRepo(testDB)
	tenantRepo := NewTenantRepo(testDB)
	ctx := context.Background()

	// Need a tenant first.
	tenant := newTestTenant()
	if err := tenantRepo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	token := newTestToken(tenant.ID)
	if err := repo.CreateToken(ctx, token); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	got, err := repo.GetByToken(ctx, token.APIToken)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if got.TenantID != tenant.ID {
		t.Fatalf("tenant_id mismatch: got %q want %q", got.TenantID, tenant.ID)
	}
}

func TestTokenGetByTokenNotFound(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantTokenRepo(testDB)
	ctx := context.Background()

	_, err := repo.GetByToken(ctx, "nonexistent-token")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTokenListByTenant(t *testing.T) {
	truncateTenants(t)
	tokenRepo := NewTenantTokenRepo(testDB)
	tenantRepo := NewTenantRepo(testDB)
	ctx := context.Background()

	tenant := newTestTenant()
	if err := tenantRepo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	// Create 3 tokens with small delays for ordering.
	var tokenStrs []string
	for i := 0; i < 3; i++ {
		tok := newTestToken(tenant.ID)
		if err := tokenRepo.CreateToken(ctx, tok); err != nil {
			t.Fatalf("CreateToken %d: %v", i, err)
		}
		tokenStrs = append(tokenStrs, tok.APIToken)
		time.Sleep(50 * time.Millisecond)
	}

	tokens, err := tokenRepo.ListByTenant(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}

	// Verify ordered by created_at.
	for i := 1; i < len(tokens); i++ {
		if tokens[i].CreatedAt.Before(tokens[i-1].CreatedAt) {
			t.Fatalf("not ordered by created_at: %v before %v", tokens[i].CreatedAt, tokens[i-1].CreatedAt)
		}
	}
}

func TestTokenListByTenantEmpty(t *testing.T) {
	truncateTenants(t)
	repo := NewTenantTokenRepo(testDB)
	ctx := context.Background()

	tokens, err := repo.ListByTenant(ctx, "nonexistent-tenant")
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}
