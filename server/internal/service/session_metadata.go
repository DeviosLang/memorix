package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/tokenizer"
)

// SessionMetadataConfig holds configuration for session metadata handling.
type SessionMetadataConfig struct {
	// MaxTokens is the maximum number of tokens allowed for metadata injection.
	// Default is 200 tokens as per the acceptance criteria.
	MaxTokens int

	// Tokenizer is used to count tokens in the formatted metadata.
	Tokenizer tokenizer.Tokenizer
}

// DefaultSessionMetadataConfig returns sensible defaults.
func DefaultSessionMetadataConfig() SessionMetadataConfig {
	return SessionMetadataConfig{
		MaxTokens: 200,
		Tokenizer: tokenizer.NewDefault(),
	}
}

// SessionMetadataFormatter formats session metadata for injection into system prompts.
type SessionMetadataFormatter struct {
	config    SessionMetadataConfig
	tokenizer tokenizer.Tokenizer
}

// NewSessionMetadataFormatter creates a new formatter with the given configuration.
func NewSessionMetadataFormatter(config SessionMetadataConfig) *SessionMetadataFormatter {
	if config.Tokenizer == nil {
		config.Tokenizer = tokenizer.NewDefault()
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 200
	}
	return &SessionMetadataFormatter{
		config:    config,
		tokenizer: config.Tokenizer,
	}
}

// Format formats the session metadata into a text block suitable for injection
// into the system prompt. The output is designed to be concise and informative.
//
// The format is:
//
//	<session-metadata>
//	Device: desktop
//	Time: 2024-01-15 14:30 (Asia/Shanghai)
//	Language: zh-CN
//	Source: web
//	</session-metadata>
func (f *SessionMetadataFormatter) Format(metadata *domain.SessionMetadata) string {
	if metadata == nil {
		return ""
	}

	var lines []string
	lines = append(lines, "<session-metadata>")

	// Current time with timezone (always included)
	timeStr := f.formatTime(metadata)
	if timeStr != "" {
		lines = append(lines, fmt.Sprintf("Time: %s", timeStr))
	}

	// Device type
	if metadata.DeviceType != "" {
		lines = append(lines, fmt.Sprintf("Device: %s", metadata.DeviceType))
	}

	// Language preference
	if metadata.LanguagePreference != "" {
		lines = append(lines, fmt.Sprintf("Language: %s", metadata.LanguagePreference))
	}

	// Entry source
	if metadata.EntrySource != "" {
		lines = append(lines, fmt.Sprintf("Source: %s", metadata.EntrySource))
	}

	// Client version (optional, less important)
	if metadata.ClientVersion != "" && f.hasTokenBudget(lines, 20) {
		lines = append(lines, fmt.Sprintf("Client: %s", metadata.ClientVersion))
	}

	// Usage statistics (optional, included only if we have token budget)
	if f.hasTokenBudget(lines, 50) {
		if metadata.ActiveDaysLast7 != nil || metadata.ActiveDaysLast30 != nil {
			var stats []string
			if metadata.ActiveDaysLast7 != nil {
				stats = append(stats, fmt.Sprintf("active %dd/7d", *metadata.ActiveDaysLast7))
			}
			if metadata.ActiveDaysLast30 != nil {
				stats = append(stats, fmt.Sprintf("%dd/30d", *metadata.ActiveDaysLast30))
			}
			if len(stats) > 0 {
				lines = append(lines, fmt.Sprintf("Activity: %s", strings.Join(stats, ", ")))
			}
		}
	}

	lines = append(lines, "</session-metadata>")

	return strings.Join(lines, "\n")
}

// formatTime formats the current time with timezone information.
func (f *SessionMetadataFormatter) formatTime(metadata *domain.SessionMetadata) string {
	if metadata.CurrentTime.IsZero() {
		return ""
	}

	// Format time as: "2024-01-15 14:30 (Asia/Shanghai)" or "2024-01-15 14:30 UTC"
	timeStr := metadata.CurrentTime.Format("2006-01-02 15:04")

	if metadata.Timezone != "" {
		return fmt.Sprintf("%s (%s)", timeStr, metadata.Timezone)
	}
	return fmt.Sprintf("%s UTC", timeStr)
}

// hasTokenBudget checks if we have enough token budget to add more content.
func (f *SessionMetadataFormatter) hasTokenBudget(lines []string, additionalTokens int) bool {
	current := f.tokenizer.CountTokens(strings.Join(lines, "\n"))
	return current+additionalTokens < f.config.MaxTokens
}

// FormatAndValidate formats the metadata and ensures it stays within token limits.
// If the formatted metadata exceeds the limit, it will be truncated.
func (f *SessionMetadataFormatter) FormatAndValidate(metadata *domain.SessionMetadata) (string, error) {
	if metadata == nil {
		return "", nil
	}

	// Validate the metadata
	if err := metadata.Validate(); err != nil {
		return "", err
	}

	// Format the metadata
	formatted := f.Format(metadata)

	// Check token count
	tokens := f.tokenizer.CountTokens(formatted)
	if tokens > f.config.MaxTokens {
		// Truncate by removing optional fields
		formatted = f.formatMinimal(metadata)
		tokens = f.tokenizer.CountTokens(formatted)
	}

	// Final check - this should rarely happen with minimal format
	if tokens > f.config.MaxTokens {
		return "", fmt.Errorf("session metadata exceeds maximum token limit (%d > %d)", tokens, f.config.MaxTokens)
	}

	return formatted, nil
}

// formatMinimal formats only the essential metadata fields.
func (f *SessionMetadataFormatter) formatMinimal(metadata *domain.SessionMetadata) string {
	var lines []string
	lines = append(lines, "<session-metadata>")

	// Only essential: time
	timeStr := f.formatTime(metadata)
	if timeStr != "" {
		lines = append(lines, fmt.Sprintf("Time: %s", timeStr))
	}

	// Device type (important for response style)
	if metadata.DeviceType != "" {
		lines = append(lines, fmt.Sprintf("Device: %s", metadata.DeviceType))
	}

	// Language (important for localization)
	if metadata.LanguagePreference != "" {
		lines = append(lines, fmt.Sprintf("Language: %s", metadata.LanguagePreference))
	}

	lines = append(lines, "</session-metadata>")
	return strings.Join(lines, "\n")
}

// CountTokens counts the tokens in the formatted metadata.
func (f *SessionMetadataFormatter) CountTokens(metadata *domain.SessionMetadata) int {
	formatted := f.Format(metadata)
	return f.tokenizer.CountTokens(formatted)
}

// BuildMetadataWithContext creates a ContextMessage from session metadata.
// The message has role "metadata" which is treated specially - always preserved
// during truncation, similar to "memory" role.
func BuildMetadataMessage(metadata *domain.SessionMetadata, formatter *SessionMetadataFormatter) (ContextMessage, error) {
	if metadata == nil {
		return ContextMessage{}, nil
	}

	content, err := formatter.FormatAndValidate(metadata)
	if err != nil {
		return ContextMessage{}, err
	}

	if content == "" {
		return ContextMessage{}, nil
	}

	return ContextMessage{
		Role:    "metadata",
		Content: content,
	}, nil
}

// CollectMetadataFromRequest extracts session metadata from an HTTP request.
// This function reads from standard headers and query parameters.
func CollectMetadataFromRequest(deviceType, timezone, language, entrySource, clientVersion string) *domain.SessionMetadata {
	metadata := &domain.SessionMetadata{
		CurrentTime: time.Now(),
	}

	if deviceType != "" {
		metadata.DeviceType = deviceType
	}
	if timezone != "" {
		metadata.Timezone = timezone
	}
	if language != "" {
		metadata.LanguagePreference = language
	}
	if entrySource != "" {
		metadata.EntrySource = entrySource
	}
	if clientVersion != "" {
		metadata.ClientVersion = clientVersion
	}

	return metadata
}

// AddUsageStatistics adds optional usage statistics to the metadata.
func AddUsageStatistics(metadata *domain.SessionMetadata, activeDays7, activeDays30 int, avgMsgLength float64, totalSessions int) *domain.SessionMetadata {
	if metadata == nil {
		metadata = &domain.SessionMetadata{CurrentTime: time.Now()}
	}

	if activeDays7 > 0 {
		metadata.ActiveDaysLast7 = &activeDays7
	}
	if activeDays30 > 0 {
		metadata.ActiveDaysLast30 = &activeDays30
	}
	if avgMsgLength > 0 {
		metadata.AverageMessageLength = &avgMsgLength
	}
	if totalSessions > 0 {
		metadata.TotalSessions = &totalSessions
	}

	return metadata
}
