package domain

import (
	"os"
	"time"
)

// RulesLevel represents the hierarchy level of a rules file.
// Rules are loaded in order: Organization → User → Project → Module.
// Later levels override earlier ones for the same content.
type RulesLevel string

const (
	// LevelOrganization is the highest level rules, typically from /etc/agent/rules.md
	// or a configuration center. These are global defaults.
	LevelOrganization RulesLevel = "organization"

	// LevelUser is the user-level rules from ~/.agent/rules.md.
	// These override organization rules.
	LevelUser RulesLevel = "user"

	// LevelProject is the project-level rules from {project_root}/.agent/rules.md.
	// These override user rules.
	LevelProject RulesLevel = "project"

	// LevelModule is the module-level rules from {project_root}/.agent/rules/*.md.
	// These are path-specific and only loaded when matching files are being edited.
	LevelModule RulesLevel = "module"
)

// RulesLevelPriority returns the priority for loading rules.
// Higher values mean higher priority (loaded later, can override earlier).
func RulesLevelPriority(level RulesLevel) int {
	switch level {
	case LevelOrganization:
		return 10
	case LevelUser:
		return 20
	case LevelProject:
		return 30
	case LevelModule:
		return 40
	default:
		return 0
	}
}

// RuleFile represents a single rules file with its content and metadata.
type RuleFile struct {
	// Level indicates the hierarchy level of this rule file.
	Level RulesLevel `json:"level"`

	// Path is the filesystem path to the rule file.
	Path string `json:"path"`

	// Content is the raw content of the rule file (including frontmatter if present).
	Content string `json:"content"`

	// Frontmatter contains parsed YAML frontmatter fields (for module-level rules).
	Frontmatter RuleFrontmatter `json:"frontmatter,omitempty"`

	// Body is the content after frontmatter (the actual rules text).
	Body string `json:"body"`

	// ModTime is the last modification time of the file.
	ModTime time.Time `json:"mod_time"`

	// Exists indicates whether the file exists on disk.
	Exists bool `json:"exists"`

	// LoadError contains any error encountered while loading this file.
	LoadError string `json:"load_error,omitempty"`
}

// RuleFrontmatter contains parsed YAML frontmatter from a rule file.
// Module-level rules use frontmatter to specify which files they apply to.
type RuleFrontmatter struct {
	// Paths specifies glob patterns for files this rule applies to.
	// Examples: ["*.py", "src/**/*.ts", "**/*.go"]
	Paths []string `json:"paths,omitempty"`

	// Name is an optional human-readable name for this rule.
	Name string `json:"name,omitempty"`

	// Description provides additional context about the rule.
	Description string `json:"description,omitempty"`

	// Priority allows overriding load order within the same level.
	// Higher priority rules are loaded later.
	Priority int `json:"priority,omitempty"`

	// Enabled controls whether this rule is active.
	Enabled *bool `json:"enabled,omitempty"`

	// Tags for categorization and filtering.
	Tags []string `json:"tags,omitempty"`
}

// IsEnabled returns true if the rule is enabled (default: true).
func (rf RuleFrontmatter) IsEnabled() bool {
	if rf.Enabled == nil {
		return true
	}
	return *rf.Enabled
}

// RulesConfig holds configuration for the rules loader.
type RulesConfig struct {
	// OrganizationRulesPath is the path to organization-level rules.
	// Default: /etc/agent/rules.md
	OrganizationRulesPath string

	// UserRulesPath is the path to user-level rules.
	// Default: ~/.agent/rules.md
	UserRulesPath string

	// ProjectRulesPath is the path to project-level rules.
	// Default: {project_root}/.agent/rules.md
	ProjectRulesPath string

	// ModuleRulesPath is the path to module-level rules directory.
	// Default: {project_root}/.agent/rules/*.md
	ModuleRulesPath string

	// ProjectRoot is the root directory of the project.
	// Required for resolving project and module rule paths.
	ProjectRoot string

	// EnableOrganization enables loading organization-level rules.
	EnableOrganization bool

	// EnableUser enables loading user-level rules.
	EnableUser bool

	// EnableProject enables loading project-level rules.
	EnableProject bool

	// EnableModule enables loading module-level rules.
	EnableModule bool
}

// DefaultRulesConfig returns the default configuration.
func DefaultRulesConfig() RulesConfig {
	homeDir, _ := os.UserHomeDir()
	userPath := ""
	if homeDir != "" {
		userPath = homeDir + "/.agent/rules.md"
	}

	return RulesConfig{
		OrganizationRulesPath: "/etc/agent/rules.md",
		UserRulesPath:         userPath,
		EnableOrganization:    true,
		EnableUser:            true,
		EnableProject:         true,
		EnableModule:          true,
	}
}

// LoadedRules represents the result of loading all rule files.
type LoadedRules struct {
	// Files contains all loaded rule files, in load order.
	Files []RuleFile `json:"files"`

	// MergedContent is the combined content from all rules, properly formatted.
	MergedContent string `json:"merged_content"`

	// ActiveFileTypes lists the file types that triggered module rule loading.
	// Empty if no module rules were loaded.
	ActiveFileTypes []string `json:"active_file_types,omitempty"`

	// TotalTokens is the estimated token count of merged content.
	TotalTokens int `json:"total_tokens"`

	// LoadTime is when these rules were loaded.
	LoadTime time.Time `json:"load_time"`

	// HasChanges indicates if any rule files changed since last load.
	HasChanges bool `json:"has_changes"`
}

// RulesLoadRequest contains parameters for loading rules.
type RulesLoadRequest struct {
	// ProjectRoot is the root directory of the current project.
	ProjectRoot string `json:"project_root,omitempty"`

	// ActiveFilePaths are paths to files currently being edited/viewed.
	// Used to match module-level rules.
	ActiveFilePaths []string `json:"active_file_paths,omitempty"`

	// PreviousLoad is the result from the previous load (for change detection).
	PreviousLoad *LoadedRules `json:"previous_load,omitempty"`
}

// RulesLoadResult contains the result of loading rules.
type RulesLoadResult struct {
	// Rules is the loaded and merged rules.
	Rules *LoadedRules `json:"rules"`

	// Changes describes what changed since the previous load.
	Changes []RulesChange `json:"changes,omitempty"`

	// Warnings contains non-fatal issues encountered during loading.
	Warnings []string `json:"warnings,omitempty"`
}

// RulesChange describes a single change to the rules.
type RulesChange struct {
	// Path is the file that changed.
	Path string `json:"path"`

	// Level is the hierarchy level of the changed file.
	Level RulesLevel `json:"level"`

	// Type describes the change: "created", "modified", "deleted".
	Type string `json:"type"`

	// OldModTime is the previous modification time (for modified files).
	OldModTime *time.Time `json:"old_mod_time,omitempty"`

	// NewModTime is the new modification time (for created/modified files).
	NewModTime *time.Time `json:"new_mod_time,omitempty"`
}

// RulesInjectionConfig controls how rules are injected into prompts.
type RulesInjectionConfig struct {
	// Enabled controls whether rules are injected.
	Enabled bool `json:"enabled"`

	// Header is prepended to the rules content.
	// Default: "## Project Rules\n\n"
	Header string `json:"header,omitempty"`

	// Footer is appended to the rules content.
	Footer string `json:"footer,omitempty"`

	// MaxTokens limits the maximum tokens for rules.
	// Rules exceeding this will be truncated.
	MaxTokens int `json:"max_tokens,omitempty"`

	// InjectAt controls where rules are injected in system instructions.
	// Options: "start", "end", "replace_marker"
	// Default: "start"
	InjectAt string `json:"inject_at,omitempty"`

	// Marker is used when InjectAt is "replace_marker".
	// Rules replace content between <!-- RULES_START --> and <!-- RULES_END -->.
	Marker string `json:"marker,omitempty"`
}

// DefaultRulesInjectionConfig returns the default injection configuration.
func DefaultRulesInjectionConfig() RulesInjectionConfig {
	return RulesInjectionConfig{
		Enabled:   true,
		Header:    "## Project Rules\n\n",
		InjectAt:  "start",
		MaxTokens: 2000,
	}
}
