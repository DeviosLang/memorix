package service

import (
	"strings"
	"testing"
)

func TestBuildMemorySchema(t *testing.T) {
	commonChecks := []string{
		"CREATE TABLE IF NOT EXISTS memories",
		"id              VARCHAR(36)",
		"INDEX idx_updated",
	}

	t.Run("no auto-model uses plain VECTOR(1536)", func(t *testing.T) {
		schema := buildMemorySchema("", 0)
		for _, needle := range commonChecks {
			if !strings.Contains(schema, needle) {
				t.Fatalf("schema missing %q", needle)
			}
		}
		if !strings.Contains(schema, "VECTOR(1536)") {
			t.Fatal("schema missing VECTOR(1536) for no-auto-model mode")
		}
		if strings.Contains(schema, "GENERATED ALWAYS AS") {
			t.Fatal("schema must not contain GENERATED ALWAYS AS for no-auto-model mode")
		}
	})

	t.Run("auto-model emits EMBED_TEXT generated column with correct dims", func(t *testing.T) {
		schema := buildMemorySchema("tidbcloud_free/amazon/titan-embed-text-v2", 1024)
		for _, needle := range commonChecks {
			if !strings.Contains(schema, needle) {
				t.Fatalf("schema missing %q", needle)
			}
		}
		if !strings.Contains(schema, "VECTOR(1024)") {
			t.Fatal("schema missing VECTOR(1024) for auto-model mode")
		}
		if !strings.Contains(schema, "GENERATED ALWAYS AS") {
			t.Fatal("schema missing GENERATED ALWAYS AS for auto-model mode")
		}
		if !strings.Contains(schema, "EMBED_TEXT") {
			t.Fatal("schema missing EMBED_TEXT for auto-model mode")
		}
		if !strings.Contains(schema, "tidbcloud_free/amazon/titan-embed-text-v2") {
			t.Fatal("schema missing model name")
		}
	})
}
