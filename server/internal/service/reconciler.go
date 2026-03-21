package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/llm"
	"github.com/devioslang/memorix/server/internal/repository"
)

// ReconcilerService provides LLM-driven memory conflict resolution.
// When a new fact conflicts with an existing fact (same user_id, category, key),
// the LLM decides whether to UPDATE (replace), APPEND (add alongside), or IGNORE.
type ReconcilerService struct {
	facts      repository.UserProfileFactRepo
	audit      repository.ReconcileAuditRepo
	llm        *llm.Client
}

// NewReconcilerService creates a new ReconcilerService.
func NewReconcilerService(
	facts repository.UserProfileFactRepo,
	audit repository.ReconcileAuditRepo,
	llmClient *llm.Client,
) *ReconcilerService {
	return &ReconcilerService{
		facts: facts,
		audit: audit,
		llm:   llmClient,
	}
}

// DetectConflict checks if a fact with the given (user_id, category, key) already exists.
// Returns the existing fact if found, or nil if no conflict.
func (s *ReconcilerService) DetectConflict(ctx context.Context, userID string, category domain.FactCategory, key string) (*domain.UserProfileFact, error) {
	existing, err := s.facts.GetByKey(ctx, userID, category, key)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, nil // No conflict
		}
		return nil, fmt.Errorf("detect conflict: %w", err)
	}
	return existing, nil
}

// Reconcile decides what to do when a new fact conflicts with an existing fact.
// If no conflict exists, the fact is simply added (returns DecisionAppend with no conflict).
// If a conflict exists, the LLM decides whether to UPDATE, APPEND, or IGNORE.
func (s *ReconcilerService) Reconcile(ctx context.Context, req domain.ReconcileRequest) (*domain.ReconcileResult, error) {
	// Step 1: Detect conflict
	existing, err := s.DetectConflict(ctx, req.UserID, req.Category, req.Key)
	if err != nil {
		return nil, fmt.Errorf("detect conflict: %w", err)
	}

	// No conflict - simply add the new fact
	if existing == nil {
		factID, err := s.addFact(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("add fact: %w", err)
		}
		return &domain.ReconcileResult{
			Request:    req,
			Decision:   domain.DecisionAppend, // Treat as append since it's new
			Reason:     "No conflict - new fact added",
			FactID:     factID,
			Conflicted: false,
		}, nil
	}

	// Conflict detected - use LLM to decide
	if s.llm == nil {
		// No LLM available - fall back to UPDATE (replace old value)
		return s.performUpdate(ctx, existing, req, "No LLM available - defaulting to UPDATE")
	}

	decision, reason, err := s.llmDecide(ctx, existing.Value, req.Value, req.Key, req.Category)
	if err != nil {
		slog.Warn("LLM reconciliation failed, defaulting to UPDATE", "err", err, "key", req.Key)
		return s.performUpdate(ctx, existing, req, fmt.Sprintf("LLM error, defaulted to UPDATE: %v", err))
	}

	switch decision {
	case domain.DecisionUpdate:
		return s.performUpdate(ctx, existing, req, reason)
	case domain.DecisionAppend:
		return s.performAppend(ctx, existing, req, reason)
	case domain.DecisionIgnore:
		return s.performIgnore(ctx, existing, req, reason)
	default:
		slog.Warn("Unknown LLM decision, defaulting to UPDATE", "decision", decision, "key", req.Key)
		return s.performUpdate(ctx, existing, req, reason)
	}
}

// BatchReconcile processes multiple facts in a single LLM call for efficiency.
// This is more efficient than calling Reconcile multiple times when there are many facts.
func (s *ReconcilerService) BatchReconcile(ctx context.Context, reqs []domain.ReconcileRequest) (*domain.BatchReconcileResult, error) {
	result := &domain.BatchReconcileResult{
		Results: make([]domain.ReconcileResult, 0, len(reqs)),
		Total:   len(reqs),
	}

	if len(reqs) == 0 {
		return result, nil
	}

	// Group facts by whether they have conflicts
	var noConflicts []struct {
		req   domain.ReconcileRequest
		index int
	}
	var conflicts []conflictPair

	for i, req := range reqs {
		existing, err := s.DetectConflict(ctx, req.UserID, req.Category, req.Key)
		if err != nil {
			return nil, fmt.Errorf("detect conflict for %s: %w", req.Key, err)
		}

		if existing == nil {
			noConflicts = append(noConflicts, struct {
				req   domain.ReconcileRequest
				index int
			}{req: req, index: i})
		} else {
			conflicts = append(conflicts, conflictPair{req: req, existing: existing, index: i})
		}
	}

	// Process non-conflicting facts (just add them)
	for _, item := range noConflicts {
		factID, err := s.addFact(ctx, item.req)
		if err != nil {
			slog.Warn("failed to add fact", "err", err, "key", item.req.Key)
			result.Results = append(result.Results, domain.ReconcileResult{
				Request:    item.req,
				Decision:   domain.DecisionIgnore,
				Reason:     fmt.Sprintf("Failed to add: %v", err),
				Conflicted: false,
			})
			result.Ignored++
			continue
		}
		result.Results = append(result.Results, domain.ReconcileResult{
			Request:    item.req,
			Decision:   domain.DecisionAppend,
			Reason:     "No conflict - new fact added",
			FactID:     factID,
			Conflicted: false,
		})
		result.Added++
	}

	// If no conflicts, we're done
	if len(conflicts) == 0 {
		return result, nil
	}

	// Process conflicts with LLM (batch call if available)
	if s.llm != nil && len(conflicts) > 1 {
		batchDecisions, err := s.llmBatchDecide(ctx, conflicts)
		if err != nil {
			slog.Warn("LLM batch reconciliation failed, falling back to individual calls", "err", err)
			// Fall back to individual calls
			for _, cp := range conflicts {
				res, err := s.Reconcile(ctx, cp.req)
				if err != nil {
					slog.Warn("individual reconciliation failed", "err", err, "key", cp.req.Key)
					result.Results = append(result.Results, domain.ReconcileResult{
						Request:    cp.req,
						Decision:   domain.DecisionIgnore,
						Reason:     fmt.Sprintf("Reconciliation failed: %v", err),
						Conflicted: true,
					})
					result.Ignored++
					result.Conflicts++
					continue
				}
				result.Results = append(result.Results, *res)
				s.updateCounts(result, res)
			}
		} else {
			// Apply batch decisions
			for i, cp := range conflicts {
				decision := batchDecisions[i]
				var res *domain.ReconcileResult
				var err error

				switch decision.Decision {
				case domain.DecisionUpdate:
					res, err = s.performUpdate(ctx, cp.existing, cp.req, decision.Reason)
				case domain.DecisionAppend:
					res, err = s.performAppend(ctx, cp.existing, cp.req, decision.Reason)
				case domain.DecisionIgnore:
					res, err = s.performIgnore(ctx, cp.existing, cp.req, decision.Reason)
				default:
					res, err = s.performUpdate(ctx, cp.existing, cp.req, decision.Reason)
				}

				if err != nil {
					slog.Warn("failed to apply decision", "err", err, "key", cp.req.Key)
					result.Results = append(result.Results, domain.ReconcileResult{
						Request:    cp.req,
						Decision:   domain.DecisionIgnore,
						Reason:     fmt.Sprintf("Failed to apply: %v", err),
						Conflicted: true,
					})
					result.Ignored++
					result.Conflicts++
					continue
				}
				result.Results = append(result.Results, *res)
				s.updateCounts(result, res)
			}
		}
	} else {
		// Process conflicts individually
		for _, cp := range conflicts {
			res, err := s.Reconcile(ctx, cp.req)
			if err != nil {
				slog.Warn("individual reconciliation failed", "err", err, "key", cp.req.Key)
				result.Results = append(result.Results, domain.ReconcileResult{
					Request:    cp.req,
					Decision:   domain.DecisionIgnore,
					Reason:     fmt.Sprintf("Reconciliation failed: %v", err),
					Conflicted: true,
				})
				result.Ignored++
				result.Conflicts++
				continue
			}
			result.Results = append(result.Results, *res)
			s.updateCounts(result, res)
		}
	}

	return result, nil
}

func (s *ReconcilerService) updateCounts(result *domain.BatchReconcileResult, res *domain.ReconcileResult) {
	result.Conflicts++
	switch res.Decision {
	case domain.DecisionUpdate:
		result.Updated++
	case domain.DecisionAppend:
		result.Added++
	case domain.DecisionIgnore:
		result.Ignored++
	}
}

// llmDecide asks the LLM what to do with a conflict.
func (s *ReconcilerService) llmDecide(ctx context.Context, oldValue, newValue, key string, category domain.FactCategory) (domain.ReconcileDecision, string, error) {
	systemPrompt := `You are a memory reconciliation engine. Your task is to analyze conflicting facts and decide how to resolve the conflict.

## Decisions

- **UPDATE**: The new fact replaces the old fact. Use when:
  - The new fact is a correction or update of the old fact (e.g., "我搬到上海了" replaces "住在北京")
  - The new fact makes the old fact obsolete
  - Both facts describe the same thing but the new one is more current

- **APPEND**: The new fact should be added alongside the old fact. Use when:
  - The facts are complementary, not contradictory (e.g., "我也会 Go" alongside "我会 Python")
  - Both facts provide useful, non-overlapping information
  - The key represents a collection (like "skills") rather than a single value

- **IGNORE**: The new fact should be discarded. Use when:
  - The new fact is unreliable or vague
  - The new fact adds no value over the existing fact
  - The new fact is redundant with the old fact

## Rules

1. Preserve the original language of the facts. Do not translate.
2. Be conservative: prefer UPDATE over APPEND for conflicting information.
3. Be explicit in your reasoning.
4. Return ONLY valid JSON. No markdown fences.

## Output Format

{"decision": "UPDATE|APPEND|IGNORE", "reason": "explanation"}`

	userPrompt := fmt.Sprintf(`Analyze this conflicting fact and decide how to resolve it.

Category: %s
Key: %s

Old value: %s
New value: %s

What should be done with this conflict?`, category, key, oldValue, newValue)

	raw, err := s.llm.CompleteJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", "", fmt.Errorf("LLM call: %w", err)
	}

	type decisionResponse struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}

	parsed, err := llm.ParseJSON[decisionResponse](raw)
	if err != nil {
		return "", "", fmt.Errorf("parse LLM response: %w", err)
	}

	decision := domain.ReconcileDecision(strings.ToUpper(parsed.Decision))
	if !decision.IsValid() {
		return "", "", fmt.Errorf("invalid decision: %s", parsed.Decision)
	}

	return decision, parsed.Reason, nil
}

// conflictPair represents a conflicting fact pair for batch processing.
type conflictPair struct {
	req      domain.ReconcileRequest
	existing *domain.UserProfileFact
	index    int
}

// llmBatchDecide processes multiple conflicts in a single LLM call.
func (s *ReconcilerService) llmBatchDecide(ctx context.Context, conflicts []conflictPair) ([]struct {
	Decision domain.ReconcileDecision
	Reason   string
}, error) {
	// Build the prompt with all conflicts
	type conflictItem struct {
		Index    int    `json:"index"`
		Category string `json:"category"`
		Key      string `json:"key"`
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
	}

	items := make([]conflictItem, len(conflicts))
	for i, cp := range conflicts {
		items[i] = conflictItem{
			Index:    i,
			Category: string(cp.req.Category),
			Key:      cp.req.Key,
			OldValue: cp.existing.Value,
			NewValue: cp.req.Value,
		}
	}

	itemsJSON, _ := json.Marshal(items)

	systemPrompt := `You are a memory reconciliation engine. Your task is to analyze multiple conflicting facts and decide how to resolve each conflict.

## Decisions

- **UPDATE**: The new fact replaces the old fact. Use when the new fact is a correction, update, or makes the old fact obsolete.
- **APPEND**: The new fact should be added alongside the old fact. Use when the facts are complementary, not contradictory.
- **IGNORE**: The new fact should be discarded. Use when it adds no value or is unreliable.

## Rules

1. Preserve the original language of the facts.
2. Be conservative: prefer UPDATE over APPEND for conflicting information.
3. Return decisions for ALL items in the same order.
4. Return ONLY valid JSON. No markdown fences.

## Output Format

{"decisions": [{"index": 0, "decision": "UPDATE|APPEND|IGNORE", "reason": "explanation"}, ...]}`

	userPrompt := fmt.Sprintf(`Analyze these %d conflicting facts and decide how to resolve each one.

%s

Return decisions for ALL items.`, len(conflicts), string(itemsJSON))

	raw, err := s.llm.CompleteJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM batch call: %w", err)
	}

	type decisionItem struct {
		Index    int    `json:"index"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	type batchResponse struct {
		Decisions []decisionItem `json:"decisions"`
	}

	parsed, err := llm.ParseJSON[batchResponse](raw)
	if err != nil {
		return nil, fmt.Errorf("parse LLM batch response: %w", err)
	}

	// Build result array in original order
	results := make([]struct {
		Decision domain.ReconcileDecision
		Reason   string
	}, len(conflicts))

	// Initialize with defaults
	for i := range results {
		results[i] = struct {
			Decision domain.ReconcileDecision
			Reason   string
		}{
			Decision: domain.DecisionUpdate,
			Reason:   "Default decision",
		}
	}

	// Apply parsed decisions
	for _, d := range parsed.Decisions {
		if d.Index >= 0 && d.Index < len(results) {
			decision := domain.ReconcileDecision(strings.ToUpper(d.Decision))
			if decision.IsValid() {
				results[d.Index] = struct {
					Decision domain.ReconcileDecision
					Reason   string
				}{
					Decision: decision,
					Reason:   d.Reason,
				}
			}
		}
	}

	return results, nil
}

// addFact creates a new fact without conflict.
func (s *ReconcilerService) addFact(ctx context.Context, req domain.ReconcileRequest) (string, error) {
	factID := uuid.New().String()
	now := time.Now()

	fact := &domain.UserProfileFact{
		FactID:     factID,
		UserID:     req.UserID,
		Category:   req.Category,
		Key:        req.Key,
		Value:      req.Value,
		Source:     domain.SourceExplicit,
		Confidence: req.Confidence,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastAccessedAt: now,
	}

	if req.Source != "" {
		fact.Source = domain.FactSource(req.Source)
	}
	if fact.Confidence == 0 {
		fact.Confidence = 1.0
	}

	if err := s.facts.Create(ctx, fact); err != nil {
		return "", err
	}

	return factID, nil
}

// performUpdate replaces the old fact with the new fact and logs the decision.
func (s *ReconcilerService) performUpdate(ctx context.Context, existing *domain.UserProfileFact, req domain.ReconcileRequest, reason string) (*domain.ReconcileResult, error) {
	// Capture the old value before update for the audit log
	oldValue := existing.Value

	// Update the fact
	existing.Value = req.Value
	if req.Source != "" {
		existing.Source = domain.FactSource(req.Source)
	}
	if req.Confidence > 0 {
		existing.Confidence = req.Confidence
	}
	existing.UpdatedAt = time.Now()

	if err := s.facts.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update fact: %w", err)
	}

	// Log the decision
	logID := uuid.New().String()
	auditLog := &domain.ReconcileAuditLog{
		LogID:     logID,
		UserID:    req.UserID,
		FactID:    existing.FactID,
		Category:  req.Category,
		Key:       req.Key,
		OldValue:  oldValue,
		NewValue:  req.Value,
		Decision:  domain.DecisionUpdate,
		Reason:    reason,
		Source:    req.Source,
		CreatedAt: time.Now(),
	}

	if err := s.audit.Create(ctx, auditLog); err != nil {
		slog.Warn("failed to create audit log", "err", err, "fact_id", existing.FactID)
	}

	return &domain.ReconcileResult{
		Request:    req,
		Decision:   domain.DecisionUpdate,
		Reason:     reason,
		FactID:     existing.FactID,
		OldValue:   oldValue,
		Conflicted: true,
	}, nil
}

// performAppend adds the new fact alongside the old fact and logs the decision.
func (s *ReconcilerService) performAppend(ctx context.Context, existing *domain.UserProfileFact, req domain.ReconcileRequest, reason string) (*domain.ReconcileResult, error) {
	// Create a new fact with a UUID suffix on the key to avoid unique constraint violation.
	// The (user_id, category, key) triplet must be unique, so we differentiate the new entry.
	newFactID := uuid.New().String()
	newKey := req.Key + ":" + newFactID
	now := time.Now()

	newFact := &domain.UserProfileFact{
		FactID:         newFactID,
		UserID:         req.UserID,
		Category:       req.Category,
		Key:            newKey,
		Value:          req.Value,
		Source:         domain.SourceExplicit,
		Confidence:     req.Confidence,
		CreatedAt:      now,
		UpdatedAt:       now,
		LastAccessedAt: now,
	}

	if req.Source != "" {
		newFact.Source = domain.FactSource(req.Source)
	}
	if newFact.Confidence == 0 {
		newFact.Confidence = 1.0
	}

	if err := s.facts.Create(ctx, newFact); err != nil {
		return nil, fmt.Errorf("append fact: %w", err)
	}

	// Log the decision
	logID := uuid.New().String()
	auditLog := &domain.ReconcileAuditLog{
		LogID:     logID,
		UserID:    req.UserID,
		FactID:    newFactID,
		Category:  req.Category,
		Key:       req.Key,
		OldValue:  "", // Empty for new facts
		NewValue:  req.Value,
		Decision:  domain.DecisionAppend,
		Reason:    reason,
		Source:    req.Source,
		CreatedAt: time.Now(),
	}

	if err := s.audit.Create(ctx, auditLog); err != nil {
		slog.Warn("failed to create audit log", "err", err, "fact_id", newFactID)
	}

	return &domain.ReconcileResult{
		Request:    req,
		Decision:   domain.DecisionAppend,
		Reason:     reason,
		FactID:     newFactID,
		OldValue:   existing.Value,
		Conflicted: true,
	}, nil
}

// performIgnore discards the new fact and logs the decision.
func (s *ReconcilerService) performIgnore(ctx context.Context, existing *domain.UserProfileFact, req domain.ReconcileRequest, reason string) (*domain.ReconcileResult, error) {
	// Log the decision
	logID := uuid.New().String()
	auditLog := &domain.ReconcileAuditLog{
		LogID:     logID,
		UserID:    req.UserID,
		FactID:    existing.FactID,
		Category:  req.Category,
		Key:       req.Key,
		OldValue:  existing.Value,
		NewValue:  req.Value,
		Decision:  domain.DecisionIgnore,
		Reason:    reason,
		Source:    req.Source,
		CreatedAt: time.Now(),
	}

	if err := s.audit.Create(ctx, auditLog); err != nil {
		slog.Warn("failed to create audit log", "err", err, "fact_id", existing.FactID)
	}

	return &domain.ReconcileResult{
		Request:    req,
		Decision:   domain.DecisionIgnore,
		Reason:     reason,
		FactID:     existing.FactID,
		OldValue:   existing.Value,
		Conflicted: true,
	}, nil
}

// GetAuditLogs retrieves audit logs for a user.
func (s *ReconcilerService) GetAuditLogs(ctx context.Context, userID string, limit, offset int) ([]domain.ReconcileAuditLog, error) {
	return s.audit.ListByUserID(ctx, userID, limit, offset)
}

// GetFactAuditLogs retrieves audit logs for a specific fact.
func (s *ReconcilerService) GetFactAuditLogs(ctx context.Context, factID string, limit, offset int) ([]domain.ReconcileAuditLog, error) {
	return s.audit.ListByFactID(ctx, factID, limit, offset)
}
