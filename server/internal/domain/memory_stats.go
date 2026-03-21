package domain

import "time"

// MemoryStats represents the overall memory usage statistics for a user.
type MemoryStats struct {
	UserID string `json:"user_id"`

	// Layer counts
	FactsCount        int `json:"facts_count"`
	SummariesCount    int `json:"summaries_count"`
	ExperiencesCount  int `json:"experiences_count"`
	MemoriesCount     int `json:"memories_count"`

	// Token estimates (approximate)
	FactsTokens       int `json:"facts_tokens"`
	SummariesTokens   int `json:"summaries_tokens"`
	ExperiencesTokens int `json:"experiences_tokens"`
	MemoriesTokens    int `json:"memories_tokens"`
	TotalTokens       int `json:"total_tokens"`

	// Capacity info
	MaxFactsPerUser       int `json:"max_facts_per_user"`
	MaxSummariesPerUser   int `json:"max_summaries_per_user"`
	MaxMemoriesPerTenant  int `json:"max_memories_per_tenant"`

	// Utilization percentages
	FactsUtilization      float64 `json:"facts_utilization"`
	SummariesUtilization  float64 `json:"summaries_utilization"`
	MemoriesUtilization   float64 `json:"memories_utilization"`

	// Timestamps
	OldestFactAt       *time.Time `json:"oldest_fact_at,omitempty"`
	NewestFactAt       *time.Time `json:"newest_fact_at,omitempty"`
	OldestSummaryAt    *time.Time `json:"oldest_summary_at,omitempty"`
	NewestSummaryAt    *time.Time `json:"newest_summary_at,omitempty"`
	OldestMemoryAt     *time.Time `json:"oldest_memory_at,omitempty"`
	NewestMemoryAt     *time.Time `json:"newest_memory_at,omitempty"`

	// Calculated at
	CalculatedAt time.Time `json:"calculated_at"`
}

// MemoryLayerStats represents stats for a single memory layer.
type MemoryLayerStats struct {
	Layer       string `json:"layer"` // facts, summaries, experiences, memories
	Count       int    `json:"count"`
	Tokens      int    `json:"tokens"`
	MaxCapacity int    `json:"max_capacity,omitempty"`
}

// UserMemoryOverview represents a complete overview of user memory state.
type UserMemoryOverview struct {
	UserID string `json:"user_id"`

	// Facts
	Facts []UserProfileFact `json:"facts,omitempty"`

	// Summaries
	Summaries []ConversationSummary `json:"summaries,omitempty"`

	// Recent Memories
	RecentMemories []Memory `json:"recent_memories,omitempty"`

	// Statistics
	Stats MemoryStats `json:"stats"`
}

// DebugContextResult represents the complete context assembly result for debugging.
type DebugContextResult struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`

	// Layer breakdown
	Layers []DebugLayerInfo `json:"layers"`

	// Assembly result
	TotalTokens int    `json:"total_tokens"`
	MaxTokens   int    `json:"max_tokens"`
	Prompt      string `json:"prompt,omitempty"` // Optional, can be large

	// Warnings/Recommendations
	Warnings []string `json:"warnings,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// DebugLayerInfo represents debug info for a single layer.
type DebugLayerInfo struct {
	Layer       string `json:"layer"`
	Content     string `json:"content,omitempty"` // Truncated for display
	TokenCount  int    `json:"token_count"`
	ItemCount   int    `json:"item_count"`
	Truncated   bool   `json:"truncated"`
	Source      string `json:"source,omitempty"`
}
