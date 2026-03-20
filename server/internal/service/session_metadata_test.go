package service

import (
	"strings"
	"testing"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

func TestSessionMetadataFormatter_Format(t *testing.T) {
	formatter := NewSessionMetadataFormatter(DefaultSessionMetadataConfig())

	tests := []struct {
		name       string
		metadata   *domain.SessionMetadata
		wantFields []string
		dontWant   []string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			wantFields: []string{},
		},
		{
			name: "full metadata",
			metadata: &domain.SessionMetadata{
				DeviceType:          "desktop",
				Timezone:            "Asia/Shanghai",
				LanguagePreference:  "zh-CN",
				CurrentTime:         time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
				EntrySource:         "web",
				ClientVersion:       "1.0.0",
				ActiveDaysLast7:     intPtr(5),
				ActiveDaysLast30:    intPtr(20),
			},
			wantFields: []string{
				"<session-metadata>",
				"Time:",
				"Device: desktop",
				"Language: zh-CN",
				"Source: web",
				"Client: 1.0.0",
				"Activity:",
				"</session-metadata>",
			},
		},
		{
			name: "minimal metadata",
			metadata: &domain.SessionMetadata{
				DeviceType:  "mobile",
				CurrentTime: time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			},
			wantFields: []string{
				"<session-metadata>",
				"Device: mobile",
				"</session-metadata>",
			},
			dontWant: []string{"Activity:", "Client:"},
		},
		{
			name: "with timezone",
			metadata: &domain.SessionMetadata{
				DeviceType:  "desktop",
				Timezone:    "America/New_York",
				CurrentTime: time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			},
			wantFields: []string{
				"(America/New_York)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.Format(tt.metadata)
			if tt.metadata == nil {
				if result != "" {
					t.Errorf("expected empty string for nil metadata, got %q", result)
				}
				return
			}

			for _, field := range tt.wantFields {
				if !strings.Contains(result, field) {
					t.Errorf("expected result to contain %q, got %q", field, result)
				}
			}

			for _, field := range tt.dontWant {
				if strings.Contains(result, field) {
					t.Errorf("expected result NOT to contain %q, got %q", field, result)
				}
			}
		})
	}
}

func TestSessionMetadataFormatter_FormatAndValidate(t *testing.T) {
	formatter := NewSessionMetadataFormatter(DefaultSessionMetadataConfig())

	t.Run("valid metadata", func(t *testing.T) {
		metadata := &domain.SessionMetadata{
			DeviceType:         "desktop",
			Timezone:           "Asia/Shanghai",
			LanguagePreference: "zh-CN",
			CurrentTime:        time.Now(),
		}

		result, err := formatter.FormatAndValidate(metadata)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("invalid device type", func(t *testing.T) {
		metadata := &domain.SessionMetadata{
			DeviceType:  "invalid",
			CurrentTime: time.Now(),
		}

		_, err := formatter.FormatAndValidate(metadata)
		if err == nil {
			t.Error("expected error for invalid device type")
		}
	})

	t.Run("nil metadata", func(t *testing.T) {
		result, err := formatter.FormatAndValidate(nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "" {
			t.Errorf("expected empty result for nil metadata, got %q", result)
		}
	})
}

func TestSessionMetadataFormatter_TokenLimit(t *testing.T) {
	// Test that formatted metadata stays under 200 tokens
	formatter := NewSessionMetadataFormatter(DefaultSessionMetadataConfig())

	metadata := &domain.SessionMetadata{
		DeviceType:          "desktop",
		Timezone:            "Asia/Shanghai",
		LanguagePreference:  "zh-CN",
		CurrentTime:         time.Now(),
		EntrySource:         "web",
		ClientVersion:       "1.0.0",
		ActiveDaysLast7:     intPtr(5),
		ActiveDaysLast30:    intPtr(20),
		AverageMessageLength: floatPtr(150.0),
		TotalSessions:       intPtr(100),
	}

	result, err := formatter.FormatAndValidate(metadata)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	tokens := formatter.CountTokens(metadata)
	if tokens > 200 {
		t.Errorf("metadata tokens %d exceeds limit of 200", tokens)
	}

	// Also check the token count matches
	actualTokens := formatter.tokenizer.CountTokens(result)
	if tokens != actualTokens {
		t.Errorf("CountTokens() = %d, want %d", tokens, actualTokens)
	}
}

func TestBuildMetadataMessage(t *testing.T) {
	formatter := NewSessionMetadataFormatter(DefaultSessionMetadataConfig())

	t.Run("valid metadata", func(t *testing.T) {
		metadata := &domain.SessionMetadata{
			DeviceType:  "desktop",
			CurrentTime: time.Now(),
		}

		msg, err := BuildMetadataMessage(metadata, formatter)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if msg.Role != "metadata" {
			t.Errorf("expected role 'metadata', got %q", msg.Role)
		}
		if msg.Content == "" {
			t.Error("expected non-empty content")
		}
	})

	t.Run("nil metadata", func(t *testing.T) {
		msg, err := BuildMetadataMessage(nil, formatter)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if msg.Content != "" {
			t.Errorf("expected empty content for nil metadata, got %q", msg.Content)
		}
	})

	t.Run("invalid metadata", func(t *testing.T) {
		metadata := &domain.SessionMetadata{
			DeviceType: "invalid_device_type",
		}

		_, err := BuildMetadataMessage(metadata, formatter)
		if err == nil {
			t.Error("expected error for invalid metadata")
		}
	})
}

func TestCollectMetadataFromRequest(t *testing.T) {
	metadata := CollectMetadataFromRequest("desktop", "Asia/Shanghai", "zh-CN", "web", "1.0.0")

	if metadata.DeviceType != "desktop" {
		t.Errorf("DeviceType = %q, want 'desktop'", metadata.DeviceType)
	}
	if metadata.Timezone != "Asia/Shanghai" {
		t.Errorf("Timezone = %q, want 'Asia/Shanghai'", metadata.Timezone)
	}
	if metadata.LanguagePreference != "zh-CN" {
		t.Errorf("LanguagePreference = %q, want 'zh-CN'", metadata.LanguagePreference)
	}
	if metadata.EntrySource != "web" {
		t.Errorf("EntrySource = %q, want 'web'", metadata.EntrySource)
	}
	if metadata.ClientVersion != "1.0.0" {
		t.Errorf("ClientVersion = %q, want '1.0.0'", metadata.ClientVersion)
	}
	if metadata.CurrentTime.IsZero() {
		t.Error("CurrentTime should be set")
	}
}

func TestAddUsageStatistics(t *testing.T) {
	t.Run("add to existing metadata", func(t *testing.T) {
		metadata := &domain.SessionMetadata{
			DeviceType:  "desktop",
			CurrentTime: time.Now(),
		}

		result := AddUsageStatistics(metadata, 5, 20, 150.0, 100)

		if *result.ActiveDaysLast7 != 5 {
			t.Errorf("ActiveDaysLast7 = %d, want 5", *result.ActiveDaysLast7)
		}
		if *result.ActiveDaysLast30 != 20 {
			t.Errorf("ActiveDaysLast30 = %d, want 20", *result.ActiveDaysLast30)
		}
		if *result.AverageMessageLength != 150.0 {
			t.Errorf("AverageMessageLength = %f, want 150.0", *result.AverageMessageLength)
		}
		if *result.TotalSessions != 100 {
			t.Errorf("TotalSessions = %d, want 100", *result.TotalSessions)
		}
	})

	t.Run("add to nil metadata", func(t *testing.T) {
		result := AddUsageStatistics(nil, 5, 20, 150.0, 100)

		if result == nil {
			t.Error("expected non-nil result")
		}
		if *result.ActiveDaysLast7 != 5 {
			t.Errorf("ActiveDaysLast7 = %d, want 5", *result.ActiveDaysLast7)
		}
	})

	t.Run("add zero values should not set fields", func(t *testing.T) {
		metadata := &domain.SessionMetadata{
			DeviceType:  "desktop",
			CurrentTime: time.Now(),
		}

		result := AddUsageStatistics(metadata, 0, 0, 0, 0)

		if result.ActiveDaysLast7 != nil {
			t.Errorf("ActiveDaysLast7 should be nil for zero value")
		}
		if result.ActiveDaysLast30 != nil {
			t.Errorf("ActiveDaysLast30 should be nil for zero value")
		}
		if result.AverageMessageLength != nil {
			t.Errorf("AverageMessageLength should be nil for zero value")
		}
		if result.TotalSessions != nil {
			t.Errorf("TotalSessions should be nil for zero value")
		}
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}
