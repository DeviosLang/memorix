package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

// mockReconcileFactRepo is a mock implementation of UserProfileFactRepo for testing.
type mockReconcileFactRepo struct {
	facts       map[string]*domain.UserProfileFact
	byKey       map[string]*domain.UserProfileFact // key: "userID:category:key"
	createErr   error
	getByKeyErr error
	updateErr   error
}

func newMockReconcileFactRepo() *mockReconcileFactRepo {
	return &mockReconcileFactRepo{
		facts: make(map[string]*domain.UserProfileFact),
		byKey: make(map[string]*domain.UserProfileFact),
	}
}

func (m *mockReconcileFactRepo) Create(ctx context.Context, fact *domain.UserProfileFact) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.facts[fact.FactID] = fact
	key := fact.UserID + ":" + string(fact.Category) + ":" + fact.Key
	m.byKey[key] = fact
	return nil
}

func (m *mockReconcileFactRepo) GetByID(ctx context.Context, factID string) (*domain.UserProfileFact, error) {
	if f, ok := m.facts[factID]; ok {
		return f, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockReconcileFactRepo) GetByUserID(ctx context.Context, userID string) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.facts {
		if f.UserID == userID {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockReconcileFactRepo) GetByUserIDAndCategory(ctx context.Context, userID string, category domain.FactCategory) ([]domain.UserProfileFact, error) {
	var result []domain.UserProfileFact
	for _, f := range m.facts {
		if f.UserID == userID && f.Category == category {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockReconcileFactRepo) List(ctx context.Context, f domain.UserProfileFactFilter) ([]domain.UserProfileFact, int, error) {
	return nil, 0, nil
}

func (m *mockReconcileFactRepo) Update(ctx context.Context, fact *domain.UserProfileFact) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.facts[fact.FactID]; !ok {
		return domain.ErrNotFound
	}
	m.facts[fact.FactID] = fact
	key := fact.UserID + ":" + string(fact.Category) + ":" + fact.Key
	m.byKey[key] = fact
	return nil
}

func (m *mockReconcileFactRepo) Delete(ctx context.Context, factID string) error {
	delete(m.facts, factID)
	return nil
}

func (m *mockReconcileFactRepo) DeleteByUserID(ctx context.Context, userID string) error {
	return nil
}

func (m *mockReconcileFactRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	count := 0
	for _, f := range m.facts {
		if f.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockReconcileFactRepo) DeleteOldestLowConfidence(ctx context.Context, userID string, count int) (int64, error) {
	return 0, nil
}

func (m *mockReconcileFactRepo) TouchLastAccessed(ctx context.Context, factID string) error {
	return nil
}

func (m *mockReconcileFactRepo) GetByKey(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error) {
	if m.getByKeyErr != nil {
		return nil, m.getByKeyErr
	}
	k := userID + ":" + string(category) + ":" + key
	if f, ok := m.byKey[k]; ok {
		return f, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockReconcileFactRepo) SearchByValue(ctx context.Context, userID string, value string, limit int) ([]domain.UserProfileFact, error) {
	return nil, nil
}

// mockAuditRepo is a mock implementation of ReconcileAuditRepo for testing.
type mockAuditRepo struct {
	logs []*domain.ReconcileAuditLog
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{
		logs: make([]*domain.ReconcileAuditLog, 0),
	}
}

func (m *mockAuditRepo) Create(ctx context.Context, log *domain.ReconcileAuditLog) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *mockAuditRepo) GetByID(ctx context.Context, logID string) (*domain.ReconcileAuditLog, error) {
	for _, l := range m.logs {
		if l.LogID == logID {
			return l, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockAuditRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]domain.ReconcileAuditLog, error) {
	var result []domain.ReconcileAuditLog
	for _, l := range m.logs {
		if l.UserID == userID {
			result = append(result, *l)
		}
	}
	return result, nil
}

func (m *mockAuditRepo) ListByFactID(ctx context.Context, factID string, limit, offset int) ([]domain.ReconcileAuditLog, error) {
	var result []domain.ReconcileAuditLog
	for _, l := range m.logs {
		if l.FactID == factID {
			result = append(result, *l)
		}
	}
	return result, nil
}

func (m *mockAuditRepo) List(ctx context.Context, f domain.ReconcileAuditFilter) ([]domain.ReconcileAuditLog, int, error) {
	return nil, 0, nil
}

func (m *mockAuditRepo) DeleteByUserID(ctx context.Context, userID string) error {
	return nil
}

func TestDetectConflict_NoConflict(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()
	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	existing, err := svc.DetectConflict(ctx, "user1", domain.CategoryPersonal, "city")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existing != nil {
		t.Fatalf("expected nil, got %+v", existing)
	}
}

func TestDetectConflict_ConflictExists(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()

	// Pre-create a fact
	now := time.Now()
	fact := &domain.UserProfileFact{
		FactID:    "fact1",
		UserID:    "user1",
		Category:  domain.CategoryPersonal,
		Key:       "city",
		Value:     "Beijing",
		Source:    domain.SourceExplicit,
		CreatedAt: now,
		UpdatedAt: now,
	}
	factRepo.facts[fact.FactID] = fact
	factRepo.byKey["user1:personal:city"] = fact

	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	existing, err := svc.DetectConflict(ctx, "user1", domain.CategoryPersonal, "city")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existing == nil {
		t.Fatal("expected conflict, got nil")
	}
	if existing.Value != "Beijing" {
		t.Fatalf("expected Beijing, got %s", existing.Value)
	}
}

func TestReconcile_NoConflict_AddsFact(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()
	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	req := domain.ReconcileRequest{
		UserID:   "user1",
		Category: domain.CategoryPersonal,
		Key:      "city",
		Value:    "Shanghai",
		Source:   "agent1",
	}

	result, err := svc.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Decision != domain.DecisionAppend {
		t.Fatalf("expected APPEND, got %s", result.Decision)
	}
	if result.Conflicted {
		t.Fatal("expected no conflict")
	}
	if result.FactID == "" {
		t.Fatal("expected fact ID")
	}

	// Verify fact was created
	if len(factRepo.facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(factRepo.facts))
	}
}

func TestReconcile_WithConflict_NoLLM_Updates(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()

	// Pre-create a fact
	now := time.Now()
	fact := &domain.UserProfileFact{
		FactID:    "fact1",
		UserID:    "user1",
		Category:  domain.CategoryPersonal,
		Key:       "city",
		Value:     "Beijing",
		Source:    domain.SourceExplicit,
		CreatedAt: now,
		UpdatedAt: now,
	}
	factRepo.facts[fact.FactID] = fact
	factRepo.byKey["user1:personal:city"] = fact

	svc := NewReconcilerService(factRepo, auditRepo, nil) // No LLM

	ctx := context.Background()
	req := domain.ReconcileRequest{
		UserID:   "user1",
		Category: domain.CategoryPersonal,
		Key:      "city",
		Value:    "Shanghai",
		Source:   "agent1",
	}

	result, err := svc.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Decision != domain.DecisionUpdate {
		t.Fatalf("expected UPDATE, got %s", result.Decision)
	}
	if !result.Conflicted {
		t.Fatal("expected conflict")
	}
	if result.OldValue != "Beijing" {
		t.Fatalf("expected old value Beijing, got %s", result.OldValue)
	}

	// Verify fact was updated
	updated := factRepo.facts["fact1"]
	if updated.Value != "Shanghai" {
		t.Fatalf("expected Shanghai, got %s", updated.Value)
	}

	// Verify audit log was created
	if len(auditRepo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditRepo.logs))
	}
}

func TestReconcile_WithConflict_NoLLMFallbackUpdate(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()

	// Pre-create a fact
	now := time.Now()
	fact := &domain.UserProfileFact{
		FactID:    "fact1",
		UserID:    "user1",
		Category:  domain.CategoryPersonal,
		Key:       "city",
		Value:     "住在北京",
		Source:    domain.SourceExplicit,
		CreatedAt: now,
		UpdatedAt: now,
	}
	factRepo.facts[fact.FactID] = fact
	factRepo.byKey["user1:personal:city"] = fact

	// Note: For actual LLM testing, we would need to mock the llm.Client
	// For this test, we test the no-LLM fallback path

	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	req := domain.ReconcileRequest{
		UserID:   "user1",
		Category: domain.CategoryPersonal,
		Key:      "city",
		Value:    "搬到上海了",
		Source:   "agent1",
	}

	result, err := svc.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without LLM, it should default to UPDATE
	if result.Decision != domain.DecisionUpdate {
		t.Fatalf("expected UPDATE, got %s", result.Decision)
	}
	if !result.Conflicted {
		t.Fatal("expected conflict")
	}
}

func TestBatchReconcile_MixedConflicts(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()

	// Pre-create some facts
	now := time.Now()
	factRepo.facts["fact1"] = &domain.UserProfileFact{
		FactID:    "fact1",
		UserID:    "user1",
		Category:  domain.CategoryPersonal,
		Key:       "city",
		Value:     "Beijing",
		Source:    domain.SourceExplicit,
		CreatedAt: now,
		UpdatedAt: now,
	}
	factRepo.byKey["user1:personal:city"] = factRepo.facts["fact1"]

	factRepo.facts["fact2"] = &domain.UserProfileFact{
		FactID:    "fact2",
		UserID:    "user1",
		Category:  domain.CategorySkill,
		Key:       "languages",
		Value:     "Python",
		Source:    domain.SourceExplicit,
		CreatedAt: now,
		UpdatedAt: now,
	}
	factRepo.byKey["user1:skill:languages"] = factRepo.facts["fact2"]

	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	reqs := []domain.ReconcileRequest{
		{
			UserID:   "user1",
			Category: domain.CategoryPersonal,
			Key:      "city",
			Value:    "Shanghai",
			Source:   "agent1",
		},
		{
			UserID:   "user1",
			Category: domain.CategorySkill,
			Key:      "languages",
			Value:    "Go",
			Source:   "agent1",
		},
		{
			UserID:   "user1",
			Category: domain.CategoryPersonal,
			Key:      "name",
			Value:    "John",
			Source:   "agent1",
		},
	}

	result, err := svc.BatchReconcile(ctx, reqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}

	// city = UPDATE (conflict, no LLM = default UPDATE)
	// languages = UPDATE (conflict, no LLM = default UPDATE)
	// name = APPEND (no conflict)
	if result.Updated != 2 {
		t.Fatalf("expected 2 updated, got %d", result.Updated)
	}
	if result.Added != 1 {
		t.Fatalf("expected 1 added, got %d", result.Added)
	}
	if result.Conflicts != 2 {
		t.Fatalf("expected 2 conflicts, got %d", result.Conflicts)
	}
}

func TestAuditLog_CreatedOnUpdate(t *testing.T) {
	factRepo := newMockReconcileFactRepo()
	auditRepo := newMockAuditRepo()

	// Pre-create a fact
	now := time.Now()
	fact := &domain.UserProfileFact{
		FactID:    "fact1",
		UserID:    "user1",
		Category:  domain.CategoryPersonal,
		Key:       "city",
		Value:     "Beijing",
		Source:    domain.SourceExplicit,
		CreatedAt: now,
		UpdatedAt: now,
	}
	factRepo.facts[fact.FactID] = fact
	factRepo.byKey["user1:personal:city"] = fact

	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	req := domain.ReconcileRequest{
		UserID:   "user1",
		Category: domain.CategoryPersonal,
		Key:      "city",
		Value:    "Shanghai",
		Source:   "agent1",
	}

	_, err := svc.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify audit log
	if len(auditRepo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditRepo.logs))
	}

	log := auditRepo.logs[0]
	if log.UserID != "user1" {
		t.Errorf("expected user1, got %s", log.UserID)
	}
	if log.FactID != "fact1" {
		t.Errorf("expected fact1, got %s", log.FactID)
	}
	if log.Decision != domain.DecisionUpdate {
		t.Errorf("expected UPDATE, got %s", log.Decision)
	}
	if log.OldValue != "Beijing" {
		t.Errorf("expected Beijing, got %s", log.OldValue)
	}
	if log.NewValue != "Shanghai" {
		t.Errorf("expected Shanghai, got %s", log.NewValue)
	}
}

func TestReconcileRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.ReconcileRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: domain.ReconcileRequest{
				UserID:   "user1",
				Category: domain.CategoryPersonal,
				Key:      "city",
				Value:    "Shanghai",
			},
			wantErr: false,
		},
		{
			name: "empty value creates fact anyway",
			req: domain.ReconcileRequest{
				UserID:   "user1",
				Category: domain.CategoryPersonal,
				Key:      "city",
				Value:    "", // Empty value is allowed
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factRepo := newMockReconcileFactRepo()
			auditRepo := newMockAuditRepo()
			svc := NewReconcilerService(factRepo, auditRepo, nil)

			ctx := context.Background()
			_, err := svc.Reconcile(ctx, tt.req)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestReconcileDecision_IsValid(t *testing.T) {
	tests := []struct {
		decision  domain.ReconcileDecision
		wantValid bool
	}{
		{domain.DecisionUpdate, true},
		{domain.DecisionAppend, true},
		{domain.DecisionIgnore, true},
		{domain.ReconcileDecision("INVALID"), false},
		{domain.ReconcileDecision(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.decision), func(t *testing.T) {
			if got := tt.decision.IsValid(); got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestGetAuditLogs(t *testing.T) {
	factRepo := newMockFactRepo()
	auditRepo := newMockAuditRepo()

	// Add some audit logs
	now := time.Now()
	auditRepo.logs = []*domain.ReconcileAuditLog{
		{
			LogID:     "log1",
			UserID:    "user1",
			FactID:    "fact1",
			Category:  domain.CategoryPersonal,
			Key:       "city",
			OldValue:  "Beijing",
			NewValue:  "Shanghai",
			Decision:  domain.DecisionUpdate,
			Reason:    "User moved",
			CreatedAt: now,
		},
		{
			LogID:     "log2",
			UserID:    "user1",
			FactID:    "fact2",
			Category:  domain.CategorySkill,
			Key:       "languages",
			OldValue:  "",
			NewValue:  "Go",
			Decision:  domain.DecisionAppend,
			Reason:    "New skill",
			CreatedAt: now,
		},
		{
			LogID:     "log3",
			UserID:    "user2",
			FactID:    "fact3",
			Category:  domain.CategoryPersonal,
			Key:       "name",
			OldValue:  "",
			NewValue:  "Alice",
			Decision:  domain.DecisionAppend,
			Reason:    "New user",
			CreatedAt: now,
		},
	}

	svc := NewReconcilerService(factRepo, auditRepo, nil)

	ctx := context.Background()
	logs, err := svc.GetAuditLogs(ctx, "user1", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}

	// Test by fact ID
	logs, err = svc.GetFactAuditLogs(ctx, "fact1", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].FactID != "fact1" {
		t.Errorf("expected fact1, got %s", logs[0].FactID)
	}
}

// Test that error types are handled correctly
func TestReconcile_ErrorHandling(t *testing.T) {
	t.Run("GetByKey error", func(t *testing.T) {
		factRepo := newMockReconcileFactRepo()
		factRepo.getByKeyErr = errors.New("db error")
		auditRepo := newMockAuditRepo()
		svc := NewReconcilerService(factRepo, auditRepo, nil)

		ctx := context.Background()
		req := domain.ReconcileRequest{
			UserID:   "user1",
			Category: domain.CategoryPersonal,
			Key:      "city",
			Value:    "Shanghai",
		}

		_, err := svc.Reconcile(ctx, req)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("Create error", func(t *testing.T) {
		factRepo := newMockReconcileFactRepo()
		factRepo.createErr = errors.New("db error")
		auditRepo := newMockAuditRepo()
		svc := NewReconcilerService(factRepo, auditRepo, nil)

		ctx := context.Background()
		req := domain.ReconcileRequest{
			UserID:   "user1",
			Category: domain.CategoryPersonal,
			Key:      "city",
			Value:    "Shanghai",
		}

		_, err := svc.Reconcile(ctx, req)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
