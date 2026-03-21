package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
)

// RulesHandler handles rules-related HTTP requests.
type RulesHandler struct {
	loader *service.RulesLoader
	logger interface{ Info(msg string, args ...any) }
}

// NewRulesHandler creates a new rules handler.
func NewRulesHandler(loader *service.RulesLoader, logger interface{ Info(msg string, args ...any) }) *RulesHandler {
	return &RulesHandler{
		loader: loader,
		logger: logger,
	}
}

// LoadRulesRequest is the request body for loading rules.
type LoadRulesRequest struct {
	// ProjectRoot is the root directory of the current project.
	ProjectRoot string `json:"project_root,omitempty"`

	// ActiveFilePaths are paths to files currently being edited/viewed.
	ActiveFilePaths []string `json:"active_file_paths,omitempty"`

	// IncludeContent controls whether full content is returned.
	// If false, only metadata is returned.
	IncludeContent bool `json:"include_content,omitempty"`
}

// LoadRulesResponse is the response for loading rules.
type LoadRulesResponse struct {
	// Files contains all loaded rule files.
	Files []RuleFileInfo `json:"files"`

	// MergedContent is the combined content from all rules.
	MergedContent string `json:"merged_content,omitempty"`

	// ActiveFileTypes lists the file types that triggered module rule loading.
	ActiveFileTypes []string `json:"active_file_types,omitempty"`

	// HasChanges indicates if any rule files changed since last load.
	HasChanges bool `json:"has_changes"`

	// Changes describes what changed since the previous load.
	Changes []RuleChangeInfo `json:"changes,omitempty"`

	// Warnings contains non-fatal issues encountered during loading.
	Warnings []string `json:"warnings,omitempty"`
}

// RuleFileInfo contains information about a single rule file.
type RuleFileInfo struct {
	// Level indicates the hierarchy level of this rule file.
	Level string `json:"level"`

	// Path is the filesystem path to the rule file.
	Path string `json:"path"`

	// Name is the human-readable name (from frontmatter or filename).
	Name string `json:"name,omitempty"`

	// Description provides additional context about the rule.
	Description string `json:"description,omitempty"`

	// Content is the actual rules text (only included if IncludeContent is true).
	Content string `json:"content,omitempty"`

	// Paths specifies glob patterns for module-level rules.
	Paths []string `json:"paths,omitempty"`

	// Exists indicates whether the file exists on disk.
	Exists bool `json:"exists"`

	// Enabled indicates whether the rule is active.
	Enabled bool `json:"enabled"`

	// LoadError contains any error encountered while loading this file.
	LoadError string `json:"load_error,omitempty"`
}

// RuleChangeInfo describes a change to a rule file.
type RuleChangeInfo struct {
	// Path is the file that changed.
	Path string `json:"path"`

	// Level is the hierarchy level of the changed file.
	Level string `json:"level"`

	// Type describes the change: "created", "modified", "deleted".
	Type string `json:"type"`
}

// LoadRules handles POST /rules/load requests.
// It loads all applicable rules based on the request parameters.
func (h *RulesHandler) LoadRules(w http.ResponseWriter, r *http.Request) {
	var req LoadRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Convert request to domain type
	loadReq := &domain.RulesLoadRequest{
		ProjectRoot:     req.ProjectRoot,
		ActiveFilePaths: req.ActiveFilePaths,
	}

	// Load rules
	result, err := h.loader.Load(loadReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load rules: "+err.Error())
		return
	}

	// Convert to response type
	resp := LoadRulesResponse{
		MergedContent:   result.Rules.MergedContent,
		ActiveFileTypes: result.Rules.ActiveFileTypes,
		HasChanges:      result.Rules.HasChanges,
		Warnings:        result.Warnings,
	}

	// Convert files
	for _, f := range result.Rules.Files {
		info := RuleFileInfo{
			Level:       string(f.Level),
			Path:        f.Path,
			Exists:      f.Exists,
			Enabled:     f.Frontmatter.IsEnabled(),
			LoadError:   f.LoadError,
			Name:        f.Frontmatter.Name,
			Description: f.Frontmatter.Description,
			Paths:       f.Frontmatter.Paths,
		}

		// Set name from filename if not in frontmatter
		if info.Name == "" && f.Exists {
			info.Name = strings.TrimSuffix(filepath.Base(f.Path), ".md")
		}

		if req.IncludeContent {
			info.Content = f.Body
		} else if req.IncludeContent == false {
			// Still include merged content at top level
		}

		resp.Files = append(resp.Files, info)
	}

	// Convert changes
	for _, c := range result.Changes {
		resp.Changes = append(resp.Changes, RuleChangeInfo{
			Path:  c.Path,
			Level: string(c.Level),
			Type:  c.Type,
		})
	}

	// Only include merged content if requested
	if !req.IncludeContent {
		resp.MergedContent = ""
	}

	respond(w, http.StatusOK, resp)
}

// CheckChangesResponse is the response for checking rule changes.
type CheckChangesResponse struct {
	// HasChanges indicates if any rule files have changed.
	HasChanges bool `json:"has_changes"`

	// CachedFileCount is the number of cached rule files.
	CachedFileCount int `json:"cached_file_count"`
}

// CheckChanges handles GET /rules/changes requests.
// It checks if any rule files have changed since the last load.
func (h *RulesHandler) CheckChanges(w http.ResponseWriter, r *http.Request) {
	hasChanges := h.loader.CheckForChanges()
	cachedFiles := h.loader.GetCachedFiles()

	respond(w, http.StatusOK, CheckChangesResponse{
		HasChanges:      hasChanges,
		CachedFileCount: len(cachedFiles),
	})
}

// RulesStatusResponse contains information about the rules system status.
type RulesStatusResponse struct {
	// Enabled indicates whether the rules system is enabled.
	Enabled bool `json:"enabled"`

	// CachedFileCount is the number of cached rule files.
	CachedFileCount int `json:"cached_file_count"`

	// CachedFiles lists the paths of cached rule files.
	CachedFiles []string `json:"cached_files,omitempty"`
}

// GetStatus handles GET /rules/status requests.
// It returns the current status of the rules system.
func (h *RulesHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	cachedFiles := h.loader.GetCachedFiles()

	var paths []string
	for path := range cachedFiles {
		paths = append(paths, path)
	}

	respond(w, http.StatusOK, RulesStatusResponse{
		Enabled:         true,
		CachedFileCount: len(cachedFiles),
		CachedFiles:     paths,
	})
}

// InjectRulesRequest is the request body for injecting rules.
type InjectRulesRequest struct {
	// SystemInstructions is the base system prompt to inject rules into.
	SystemInstructions string `json:"system_instructions,omitempty"`

	// ProjectRoot is the root directory of the current project.
	ProjectRoot string `json:"project_root,omitempty"`

	// ActiveFilePaths are paths to files currently being edited/viewed.
	ActiveFilePaths []string `json:"active_file_paths,omitempty"`

	// Header is prepended to the rules content (overrides default).
	Header string `json:"header,omitempty"`

	// InjectAt controls where rules are injected ("start", "end", "replace_marker").
	InjectAt string `json:"inject_at,omitempty"`
}

// InjectRulesResponse is the response for injecting rules.
type InjectRulesResponse struct {
	// Result is the system instructions with rules injected.
	Result string `json:"result"`

	// RulesContent is the rules content that was injected.
	RulesContent string `json:"rules_content,omitempty"`

	// RulesTokenCount is the approximate token count of the rules.
	RulesTokenCount int `json:"rules_token_count,omitempty"`

	// ActiveFileTypes lists the file types that triggered module rule loading.
	ActiveFileTypes []string `json:"active_file_types,omitempty"`
}

// InjectRules handles POST /rules/inject requests.
// It loads rules and injects them into system instructions.
func (h *RulesHandler) InjectRules(w http.ResponseWriter, r *http.Request, injector *service.RulesInjector) {
	var req InjectRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Load rules
	loadReq := &domain.RulesLoadRequest{
		ProjectRoot:     req.ProjectRoot,
		ActiveFilePaths: req.ActiveFilePaths,
	}

	result, err := h.loader.Load(loadReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load rules: "+err.Error())
		return
	}

	// Apply custom injection settings if provided
	if req.Header != "" || req.InjectAt != "" {
		config := domain.DefaultRulesInjectionConfig()
		if req.Header != "" {
			config.Header = req.Header
		}
		if req.InjectAt != "" {
			config.InjectAt = req.InjectAt
		}
		injector.UpdateConfig(config)
	}

	// Inject rules into system instructions
	resultInstructions := injector.InjectRules(req.SystemInstructions, result.Rules)

	// Estimate token count (rough approximation: ~4 chars per token)
	tokenCount := len(result.Rules.MergedContent) / 4

	respond(w, http.StatusOK, InjectRulesResponse{
		Result:          resultInstructions,
		RulesContent:    result.Rules.MergedContent,
		RulesTokenCount: tokenCount,
		ActiveFileTypes: result.Rules.ActiveFileTypes,
	})
}
