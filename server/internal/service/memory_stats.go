package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/devioslang/memorix/server/internal/tokenizer"
)

// MemoryStatsService provides memory usage statistics.
type MemoryStatsService struct {
	factsRepo      repository.UserProfileFactRepo
	summaryRepo    repository.ConversationSummaryRepo
	memoryRepo     repository.MemoryRepo
	tokenizer      tokenizer.Tokenizer
	maxFacts       int
	maxSummaries   int
	maxMemories    int
}

// NewMemoryStatsService creates a new MemoryStatsService.
func NewMemoryStatsService(
	factsRepo repository.UserProfileFactRepo,
	summaryRepo repository.ConversationSummaryRepo,
	memoryRepo repository.MemoryRepo,
	tok tokenizer.Tokenizer,
	maxFacts, maxSummaries, maxMemories int,
) *MemoryStatsService {
	if maxFacts <= 0 {
		maxFacts = domain.DefaultMaxFactsPerUser
	}
	if maxSummaries <= 0 {
		maxSummaries = domain.DefaultMaxSummariesPerUser
	}
	if maxMemories <= 0 {
		maxMemories = 10000
	}
	return &MemoryStatsService{
		factsRepo:    factsRepo,
		summaryRepo:  summaryRepo,
		memoryRepo:   memoryRepo,
		tokenizer:    tok,
		maxFacts:     maxFacts,
		maxSummaries: maxSummaries,
		maxMemories:  maxMemories,
	}
}

// GetStats returns memory usage statistics for a user.
func (s *MemoryStatsService) GetStats(ctx context.Context, userID string) (*domain.MemoryStats, error) {
	stats := &domain.MemoryStats{
		UserID:              userID,
		MaxFactsPerUser:     s.maxFacts,
		MaxSummariesPerUser: s.maxSummaries,
		MaxMemoriesPerTenant: s.maxMemories,
		CalculatedAt:        time.Now(),
	}

	// Get facts stats
	facts, err := s.factsRepo.GetByUserID(ctx, userID)
	if err != nil {
		slog.Warn("failed to get facts for stats", "user_id", userID, "err", err)
	} else {
		stats.FactsCount = len(facts)
		stats.FactsTokens = s.countFactTokens(facts)
		if len(facts) > 0 {
			stats.OldestFactAt = &facts[len(facts)-1].CreatedAt
			stats.NewestFactAt = &facts[0].CreatedAt
		}
	}

	// Get summaries stats
	summaries, err := s.summaryRepo.GetByUserID(ctx, userID, 100)
	if err != nil {
		slog.Warn("failed to get summaries for stats", "user_id", userID, "err", err)
	} else {
		stats.SummariesCount = len(summaries)
		stats.SummariesTokens = s.countSummaryTokens(summaries)
		if len(summaries) > 0 {
			stats.OldestSummaryAt = &summaries[len(summaries)-1].CreatedAt
			stats.NewestSummaryAt = &summaries[0].CreatedAt
		}
	}

	// Get memories stats (using agent_id as user filter for now)
	memories, _, err := s.memoryRepo.List(ctx, domain.MemoryFilter{
		AgentID: userID,
		State:   "active",
		Limit:   1000,
	})
	if err != nil {
		slog.Warn("failed to get memories for stats", "user_id", userID, "err", err)
	} else {
		stats.MemoriesCount = len(memories)
		stats.MemoriesTokens = s.countMemoryTokens(memories)
		if len(memories) > 0 {
			stats.OldestMemoryAt = &memories[len(memories)-1].CreatedAt
			stats.NewestMemoryAt = &memories[0].CreatedAt
		}
	}

	// Calculate totals
	stats.TotalTokens = stats.FactsTokens + stats.SummariesTokens + stats.MemoriesTokens + stats.ExperiencesTokens

	// Calculate utilization
	if stats.MaxFactsPerUser > 0 {
		stats.FactsUtilization = float64(stats.FactsCount) / float64(stats.MaxFactsPerUser) * 100
	}
	if stats.MaxSummariesPerUser > 0 {
		stats.SummariesUtilization = float64(stats.SummariesCount) / float64(stats.MaxSummariesPerUser) * 100
	}
	if stats.MaxMemoriesPerTenant > 0 {
		stats.MemoriesUtilization = float64(stats.MemoriesCount) / float64(stats.MaxMemoriesPerTenant) * 100
	}

	return stats, nil
}

// GetOverview returns a complete memory overview for a user.
func (s *MemoryStatsService) GetOverview(ctx context.Context, userID string, includeContent bool) (*domain.UserMemoryOverview, error) {
	overview := &domain.UserMemoryOverview{
		UserID: userID,
	}

	// Get stats
	stats, err := s.GetStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	overview.Stats = *stats

	if includeContent {
		// Get facts
		facts, err := s.factsRepo.GetByUserID(ctx, userID)
		if err == nil {
			overview.Facts = facts
		}

		// Get summaries
		summaries, err := s.summaryRepo.GetByUserID(ctx, userID, 20)
		if err == nil {
			overview.Summaries = summaries
		}

		// Get recent memories
		memories, _, err := s.memoryRepo.List(ctx, domain.MemoryFilter{
			AgentID: userID,
			State:   "active",
			Limit:   20,
		})
		if err == nil {
			overview.RecentMemories = memories
		}
	}

	return overview, nil
}

// countFactTokens counts tokens in user profile facts.
func (s *MemoryStatsService) countFactTokens(facts []domain.UserProfileFact) int {
	var total int
	for _, f := range facts {
		// Rough estimate: key + value + category overhead
		content := f.Key + ": " + f.Value
		if s.tokenizer != nil {
			total += s.tokenizer.CountTokens(content)
		} else {
			// Fallback: ~4 chars per token
			total += len(content) / 4
		}
	}
	return total
}

// countSummaryTokens counts tokens in conversation summaries.
func (s *MemoryStatsService) countSummaryTokens(summaries []domain.ConversationSummary) int {
	var total int
	for _, sum := range summaries {
		// Title + summary + topics
		content := sum.Title + " " + sum.Summary + " " + sum.UserIntent
		if s.tokenizer != nil {
			total += s.tokenizer.CountTokens(content)
		} else {
			total += len(content) / 4
		}
	}
	return total
}

// countMemoryTokens counts tokens in memories.
func (s *MemoryStatsService) countMemoryTokens(memories []domain.Memory) int {
	var total int
	for _, m := range memories {
		if s.tokenizer != nil {
			total += s.tokenizer.CountTokens(m.Content)
		} else {
			total += len(m.Content) / 4
		}
	}
	return total
}
