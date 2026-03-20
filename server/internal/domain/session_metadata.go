package domain

import (
	"fmt"
	"time"
)

// SessionMetadata represents transient metadata that is collected at the start
// of each session and injected into the system prompt. Unlike memories, this
// data is not stored long-term and is refreshed on each new session.
//
// This is inspired by ChatGPT's "session metadata" layer that provides
// contextual information to help adjust response style in real-time.
type SessionMetadata struct {
	// Basic Information

	// DeviceType indicates the type of device the user is using.
	// Examples: "desktop", "mobile", "tablet", "cli"
	DeviceType string `json:"device_type,omitempty"`

	// Timezone is the user's timezone, e.g., "Asia/Shanghai", "America/New_York"
	Timezone string `json:"timezone,omitempty"`

	// LanguagePreference is the user's preferred language, e.g., "zh-CN", "en-US"
	LanguagePreference string `json:"language_preference,omitempty"`

	// Session Context

	// CurrentTime is the timestamp when the session started.
	CurrentTime time.Time `json:"current_time,omitempty"`

	// EntrySource indicates how the user started this session.
	// Examples: "web", "api", "cli", "plugin", "mobile_app"
	EntrySource string `json:"entry_source,omitempty"`

	// ClientVersion is the version of the client application.
	ClientVersion string `json:"client_version,omitempty"`

	// Optional Usage Statistics

	// ActiveDaysLast7 is the number of days the user was active in the last 7 days.
	ActiveDaysLast7 *int `json:"active_days_last_7,omitempty"`

	// ActiveDaysLast30 is the number of days the user was active in the last 30 days.
	ActiveDaysLast30 *int `json:"active_days_last_30,omitempty"`

	// AverageMessageLength is the average length of user messages in characters.
	AverageMessageLength *float64 `json:"average_message_length,omitempty"`

	// TotalSessions is the total number of sessions the user has had.
	TotalSessions *int `json:"total_sessions,omitempty"`
}

// Validate checks that the session metadata fields are valid.
func (m *SessionMetadata) Validate() error {
	if m.DeviceType != "" {
		validDevices := []string{"desktop", "mobile", "tablet", "cli", "api", "unknown"}
		if !contains(validDevices, m.DeviceType) {
			return &ValidationError{
				Field:   "device_type",
				Message: fmt.Sprintf("must be one of: %v", validDevices),
			}
		}
	}
	return nil
}

// contains is a helper function to check if a string is in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SessionMetadataFromHeaders extracts session metadata from HTTP headers.
// This is a convenience function for common header-based metadata extraction.
func SessionMetadataFromHeaders(headers map[string]string) *SessionMetadata {
	metadata := &SessionMetadata{}

	if v, ok := headers["X-Device-Type"]; ok {
		metadata.DeviceType = v
	}
	if v, ok := headers["X-Timezone"]; ok {
		metadata.Timezone = v
	}
	if v, ok := headers["X-Language"]; ok {
		metadata.LanguagePreference = v
	}
	if v, ok := headers["X-Entry-Source"]; ok {
		metadata.EntrySource = v
	}
	if v, ok := headers["X-Client-Version"]; ok {
		metadata.ClientVersion = v
	}

	// Set current time if not provided
	metadata.CurrentTime = time.Now()

	return metadata
}
