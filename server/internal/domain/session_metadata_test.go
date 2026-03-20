package domain

import (
	"testing"
	"time"
)

func TestSessionMetadata_Validate(t *testing.T) {
	tests := []struct {
		name    string
		metadata *SessionMetadata
		wantErr bool
	}{
		{
			name:    "nil metadata",
			metadata: nil,
			wantErr: false,
		},
		{
			name: "valid metadata with all fields",
			metadata: &SessionMetadata{
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
			},
			wantErr: false,
		},
		{
			name: "valid metadata with minimal fields",
			metadata: &SessionMetadata{
				DeviceType:  "mobile",
				CurrentTime: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "invalid device type",
			metadata: &SessionMetadata{
				DeviceType: "invalid_device",
			},
			wantErr: true,
		},
		{
			name: "valid device types",
			metadata: &SessionMetadata{
				DeviceType: "cli",
			},
			wantErr: false,
		},
		{
			name: "valid device type api",
			metadata: &SessionMetadata{
				DeviceType: "api",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.metadata == nil {
				return // nil metadata is valid
			}
			err := tt.metadata.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SessionMetadata.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSessionMetadataFromHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected *SessionMetadata
	}{
		{
			name:     "empty headers",
			headers:  map[string]string{},
			expected: &SessionMetadata{CurrentTime: time.Now()},
		},
		{
			name: "all headers present",
			headers: map[string]string{
				"X-Device-Type":    "desktop",
				"X-Timezone":       "America/New_York",
				"X-Language":       "en-US",
				"X-Entry-Source":   "cli",
				"X-Client-Version": "2.0.0",
			},
			expected: &SessionMetadata{
				DeviceType:         "desktop",
				Timezone:           "America/New_York",
				LanguagePreference: "en-US",
				EntrySource:        "cli",
				ClientVersion:      "2.0.0",
				CurrentTime:        time.Now(),
			},
		},
		{
			name: "partial headers",
			headers: map[string]string{
				"X-Device-Type": "mobile",
				"X-Timezone":    "Europe/London",
			},
			expected: &SessionMetadata{
				DeviceType:  "mobile",
				Timezone:    "Europe/London",
				CurrentTime: time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SessionMetadataFromHeaders(tt.headers)
			if result.DeviceType != tt.expected.DeviceType {
				t.Errorf("DeviceType = %v, want %v", result.DeviceType, tt.expected.DeviceType)
			}
			if result.Timezone != tt.expected.Timezone {
				t.Errorf("Timezone = %v, want %v", result.Timezone, tt.expected.Timezone)
			}
			if result.LanguagePreference != tt.expected.LanguagePreference {
				t.Errorf("LanguagePreference = %v, want %v", result.LanguagePreference, tt.expected.LanguagePreference)
			}
			if result.EntrySource != tt.expected.EntrySource {
				t.Errorf("EntrySource = %v, want %v", result.EntrySource, tt.expected.EntrySource)
			}
			if result.ClientVersion != tt.expected.ClientVersion {
				t.Errorf("ClientVersion = %v, want %v", result.ClientVersion, tt.expected.ClientVersion)
			}
			if result.CurrentTime.IsZero() {
				t.Error("CurrentTime should be set")
			}
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}
