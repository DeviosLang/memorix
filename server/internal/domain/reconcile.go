package domain

import (
	"time"
)

// ReconcileDecision represents the decision made by the LLM reconciler
// when a conflict is detected between new and existing facts.
type ReconcileDecision string

const (
	// DecisionUpdate means the new fact replaces the old fact.
	// Example: "我搬到上海了" replaces "住在北京"
	DecisionUpdate ReconcileDecision = "UPDATE"

	// DecisionAppend means the new fact should be added alongside the old fact.
	// Example: "我也会 Go" is added alongside "我会 Python"
	DecisionAppend ReconcileDecision = "APPEND"

	// DecisionIgnore means the new fact is unreliable or unimportant and should be ignored.
	DecisionIgnore ReconcileDecision = "IGNORE"
)

// IsValid checks if a decision is valid.
func (d ReconcileDecision) IsValid() bool {
	switch d {
	case DecisionUpdate, DecisionAppend, DecisionIgnore:
		return true
	default:
		return false
	}
}

// ReconcileAuditLog records a single reconciliation decision for audit purposes.
type ReconcileAuditLog struct {
	LogID     string           `json:"log_id"`
	UserID    string           `json:"user_id"`
	FactID    string           `json:"fact_id"`    // The fact ID that was reconciled
	Category  FactCategory     `json:"category"`   // Category of the fact
	Key       string           `json:"key"`        // Key of the fact
	OldValue  string           `json:"old_value"`  // Previous value (empty for new facts)
	NewValue  string           `json:"new_value"`  // Incoming value
	Decision  ReconcileDecision `json:"decision"`  // The decision made
	Reason    string           `json:"reason"`     // LLM's explanation for the decision
	Source    string           `json:"source"`     // Where the new fact came from (agent_name)
	CreatedAt time.Time        `json:"created_at"`
}

// ReconcileRequest represents a single fact to be reconciled.
type ReconcileRequest struct {
	UserID    string       `json:"user_id"`
	Category  FactCategory `json:"category"`
	Key       string       `json:"key"`
	Value     string       `json:"value"`
	Source    string       `json:"source,omitempty"`    // Optional: agent name
	Confidence float64     `json:"confidence,omitempty"` // Optional: confidence level
}

// ReconcileResult represents the outcome of reconciling a single fact.
type ReconcileResult struct {
	Request    ReconcileRequest   `json:"request"`
	Decision   ReconcileDecision  `json:"decision"`
	Reason     string             `json:"reason"`
	FactID     string             `json:"fact_id,omitempty"`     // ID of created/updated fact
	OldValue   string             `json:"old_value,omitempty"`   // Previous value if updated
	Conflicted bool               `json:"conflicted"`            // True if conflict was detected
}

// BatchReconcileRequest is the input for batch reconciliation.
type BatchReconcileRequest struct {
	Facts []ReconcileRequest `json:"facts"`
}

// BatchReconcileResult is the output of batch reconciliation.
type BatchReconcileResult struct {
	Results []ReconcileResult `json:"results"`
	// Summary counts
	Total     int `json:"total"`
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Ignored   int `json:"ignored"`
	Conflicts int `json:"conflicts"`
}

// ReconcileAuditFilter encapsulates query parameters for audit log queries.
type ReconcileAuditFilter struct {
	UserID   string
	FactID   string
	Category FactCategory
	Limit    int
	Offset   int
}
