package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
)

func TestRulesLoader_Load(t *testing.T) {
	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "rules-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test rule files
	orgRules := filepath.Join(tmpDir, "org-rules.md")
	userRules := filepath.Join(tmpDir, "user-rules.md")
	projectRules := filepath.Join(tmpDir, "project-rules.md")
	moduleDir := filepath.Join(tmpDir, "rules")
	os.MkdirAll(moduleDir, 0755)

	// Organization rules
	if err := os.WriteFile(orgRules, []byte("# Organization Rules\n\nBe professional and helpful."), 0644); err != nil {
		t.Fatalf("failed to write org rules: %v", err)
	}

	// User rules
	if err := os.WriteFile(userRules, []byte("# User Rules\n\nFocus on clarity."), 0644); err != nil {
		t.Fatalf("failed to write user rules: %v", err)
	}

	// Project rules
	if err := os.WriteFile(projectRules, []byte("# Project Rules\n\nUse tabs for indentation."), 0644); err != nil {
		t.Fatalf("failed to write project rules: %v", err)
	}

	// Module rules with frontmatter
	pythonRules := `---
paths:
  - "*.py"
  - "**/*.py"
name: Python Rules
description: Rules for Python files
enabled: true
---

# Python Rules

- Use snake_case for variables
- Follow PEP 8 style guide
`
	if err := os.WriteFile(filepath.Join(moduleDir, "python.md"), []byte(pythonRules), 0644); err != nil {
		t.Fatalf("failed to write python rules: %v", err)
	}

	// TypeScript rules
	typescriptRules := `---
paths:
  - "*.ts"
  - "*.tsx"
name: TypeScript Rules
enabled: true
---

# TypeScript Rules

- Use camelCase for variables
- Prefer interfaces over types
`
	if err := os.WriteFile(filepath.Join(moduleDir, "typescript.md"), []byte(typescriptRules), 0644); err != nil {
		t.Fatalf("failed to write typescript rules: %v", err)
	}

	// Disabled rules (should not be loaded)
	disabledRules := `---
paths:
  - "*.go"
enabled: false
---

# Go Rules (Disabled)

These should not appear.
`
	if err := os.WriteFile(filepath.Join(moduleDir, "disabled.md"), []byte(disabledRules), 0644); err != nil {
		t.Fatalf("failed to write disabled rules: %v", err)
	}

	// Create loader
	config := domain.RulesConfig{
		OrganizationRulesPath: orgRules,
		UserRulesPath:         userRules,
		ProjectRulesPath:      projectRules,
		ModuleRulesPath:       moduleDir,
		EnableOrganization:    true,
		EnableUser:            true,
		EnableProject:         true,
		EnableModule:          true,
	}

	loader := NewRulesLoader(config, nil)

	t.Run("LoadAllRules", func(t *testing.T) {
		req := &domain.RulesLoadRequest{
			ActiveFilePaths: []string{"main.py", "utils/helpers.py"},
		}

		result, err := loader.Load(req)
		if err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}

		// Should have 4 files: org, user, project, python module
		if len(result.Rules.Files) != 4 {
			t.Errorf("expected 4 files, got %d", len(result.Rules.Files))
		}

		// Check merged content contains all sections
		if result.Rules.MergedContent == "" {
			t.Error("merged content should not be empty")
		}

		// Verify active file types
		if len(result.Rules.ActiveFileTypes) != 1 || result.Rules.ActiveFileTypes[0] != ".py" {
			t.Errorf("expected active file types [.py], got %v", result.Rules.ActiveFileTypes)
		}

		// Verify Python rules are included
		foundPython := false
		for _, f := range result.Rules.Files {
			if f.Level == domain.LevelModule && f.Frontmatter.Name == "Python Rules" {
				foundPython = true
				break
			}
		}
		if !foundPython {
			t.Error("Python module rules should be loaded for .py files")
		}
	})

	t.Run("LoadTypeScriptRules", func(t *testing.T) {
		req := &domain.RulesLoadRequest{
			ActiveFilePaths: []string{"app.ts", "components/Button.tsx"},
		}

		result, err := loader.Load(req)
		if err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}

		// Should have 4 files: org, user, project, typescript module
		if len(result.Rules.Files) != 4 {
			t.Errorf("expected 4 files, got %d", len(result.Rules.Files))
		}

		// Verify TypeScript rules are included
		foundTS := false
		for _, f := range result.Rules.Files {
			if f.Level == domain.LevelModule && f.Frontmatter.Name == "TypeScript Rules" {
				foundTS = true
				break
			}
		}
		if !foundTS {
			t.Error("TypeScript module rules should be loaded for .ts/.tsx files")
		}
	})

	t.Run("LoadMixedFileTypes", func(t *testing.T) {
		req := &domain.RulesLoadRequest{
			ActiveFilePaths: []string{"main.py", "utils.ts", "README.md"},
		}

		result, err := loader.Load(req)
		if err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}

		// Should have 5 files: org, user, project, python, typescript
		if len(result.Rules.Files) != 5 {
			t.Errorf("expected 5 files, got %d", len(result.Rules.Files))
		}

		// Verify both file types
		if len(result.Rules.ActiveFileTypes) != 3 {
			t.Errorf("expected 3 active file types, got %d", len(result.Rules.ActiveFileTypes))
		}
	})

	t.Run("DisabledRulesNotLoaded", func(t *testing.T) {
		req := &domain.RulesLoadRequest{
			ActiveFilePaths: []string{"main.go"},
		}

		result, err := loader.Load(req)
		if err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}

		// Should have only 3 files: org, user, project (disabled module not included)
		if len(result.Rules.Files) != 3 {
			t.Errorf("expected 3 files (disabled should not be loaded), got %d", len(result.Rules.Files))
		}
	})

	t.Run("ChangeDetection", func(t *testing.T) {
		// Use a fresh loader to ensure a clean cache (shared loader from previous
		// sub-tests already has populated cache, which would falsify "first load" semantics).
		freshLoader := NewRulesLoader(config, nil)

		// First load
		req := &domain.RulesLoadRequest{
			ActiveFilePaths: []string{"test.py"},
		}
		result, err := freshLoader.Load(req)
		if err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}
		if result.Rules.HasChanges {
			t.Error("first load should not detect changes")
		}

		// Modify a file
		time.Sleep(100 * time.Millisecond) // Ensure mod time changes
		if err := os.WriteFile(projectRules, []byte("# Modified Project Rules\n\nNew content."), 0644); err != nil {
			t.Fatalf("failed to modify project rules: %v", err)
		}

		// Second load should detect changes
		result, err = freshLoader.Load(req)
		if err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}
		if !result.Rules.HasChanges {
			t.Error("should detect changes after modification")
		}
		if len(result.Changes) == 0 {
			t.Error("should have change records")
		}
	})
}

func TestRulesLoader_ParseFrontmatter(t *testing.T) {
	loader := &RulesLoader{}

	tests := []struct {
		name           string
		content        string
		wantPaths      []string
		wantName       string
		wantEnabled    bool
		wantBodyPrefix string
	}{
		{
			name: "WithFrontmatter",
			content: `---
paths:
  - "*.py"
name: Python Rules
enabled: true
---

# Rules Body

Some rules here.`,
			wantPaths:      []string{"*.py"},
			wantName:       "Python Rules",
			wantEnabled:    true,
			wantBodyPrefix: "# Rules Body",
		},
		{
			name: "WithoutFrontmatter",
			content: `# Plain Rules

No frontmatter here.`,
			wantPaths:      nil,
			wantName:       "",
			wantEnabled:    true, // default
			wantBodyPrefix: "# Plain Rules",
		},
		{
			name: "DisabledRule",
			content: `---
enabled: false
---

# Disabled Rules

Should not be loaded.`,
			wantPaths:      nil,
			wantName:       "",
			wantEnabled:    false,
			wantBodyPrefix: "# Disabled Rules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := loader.parseFrontmatter([]byte(tt.content))

			if len(fm.Paths) != len(tt.wantPaths) {
				t.Errorf("paths mismatch: got %v, want %v", fm.Paths, tt.wantPaths)
			}

			if fm.Name != tt.wantName {
				t.Errorf("name mismatch: got %q, want %q", fm.Name, tt.wantName)
			}

			if fm.IsEnabled() != tt.wantEnabled {
				t.Errorf("enabled mismatch: got %v, want %v", fm.IsEnabled(), tt.wantEnabled)
			}

			if len(body) > 0 && len(tt.wantBodyPrefix) > 0 {
				if body[:len(tt.wantBodyPrefix)] != tt.wantBodyPrefix {
					t.Errorf("body prefix mismatch: got %q, want %q", body[:len(tt.wantBodyPrefix)], tt.wantBodyPrefix)
				}
			}
		})
	}
}

func TestRulesLoader_MatchPath(t *testing.T) {
	loader := &RulesLoader{}

	tests := []struct {
		path     string
		patterns []string
		want     bool
	}{
		{"main.py", []string{"*.py"}, true},
		{"src/main.py", []string{"*.py"}, true},
		{"src/utils/helpers.py", []string{"**/*.py"}, true},
		{"app.ts", []string{"*.py", "*.ts"}, true},
		{"README.md", []string{"*.py"}, false},
		{"src/main.go", []string{"**/*.py"}, false},
		{"test_main.py", []string{"test_*.py"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := loader.matchesAnyPattern(tt.path, tt.patterns)
			if got != tt.want {
				t.Errorf("matchesAnyPattern(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestRulesInjector_InjectRules(t *testing.T) {
	rules := &domain.LoadedRules{
		MergedContent: "# Project Rules\n\n- Rule 1\n- Rule 2",
	}

	tests := []struct {
		name              string
		systemInstructions string
		injectAt          string
		wantContains      []string
	}{
		{
			name:              "InjectAtStart",
			systemInstructions: "You are a helpful assistant.",
			injectAt:          "start",
			wantContains:      []string{"## Project Rules", "You are a helpful assistant."},
		},
		{
			name:              "InjectAtEnd",
			systemInstructions: "You are a helpful assistant.",
			injectAt:          "end",
			wantContains:      []string{"You are a helpful assistant.", "## Project Rules"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := domain.DefaultRulesInjectionConfig()
			config.InjectAt = tt.injectAt
			injector := NewRulesInjector(config, nil)

			result := injector.InjectRules(tt.systemInstructions, rules)

			for _, want := range tt.wantContains {
				if !contains(result, want) {
					t.Errorf("result should contain %q, got: %s", want, result)
				}
			}
		})
	}
}

func TestRulesInjector_InjectWithMarkers(t *testing.T) {
	config := domain.DefaultRulesInjectionConfig()
	config.InjectAt = "replace_marker"
	injector := NewRulesInjector(config, nil)

	rules := &domain.LoadedRules{
		MergedContent: "# New Rules\n\nNew content here.",
	}

	systemInstructions := `System prompt start.

<!-- RULES_START -->
Old rules content.
<!-- RULES_END -->

System prompt end.`

	result := injector.InjectRules(systemInstructions, rules)

	if contains(result, "Old rules content") {
		t.Error("old rules content should be replaced")
	}
	if !contains(result, "New Rules") {
		t.Error("new rules content should be present")
	}
	if !contains(result, "System prompt start") {
		t.Error("system prompt start should be preserved")
	}
	if !contains(result, "System prompt end") {
		t.Error("system prompt end should be preserved")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
