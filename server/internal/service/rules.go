package service

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devioslang/memorix/server/internal/domain"
	"gopkg.in/yaml.v3"
)

// RulesLoader handles loading and merging rules from multiple hierarchy levels.
type RulesLoader struct {
	config domain.RulesConfig
	logger *slog.Logger
	cache  *rulesCache
	mu     sync.RWMutex
}

// rulesCache stores loaded rules for change detection.
type rulesCache struct {
	files    map[string]time.Time // path -> modTime
	loadTime time.Time
}

// NewRulesLoader creates a new rules loader with the given configuration.
func NewRulesLoader(config domain.RulesConfig, logger *slog.Logger) *RulesLoader {
	if logger == nil {
		logger = slog.Default()
	}

	return &RulesLoader{
		config: config,
		logger: logger,
		cache: &rulesCache{
			files: make(map[string]time.Time),
		},
	}
}

// Load loads all applicable rules based on the request parameters.
// Rules are loaded in order: Organization → User → Project → Module.
// Later levels can override earlier ones.
func (l *RulesLoader) Load(req *domain.RulesLoadRequest) (*domain.RulesLoadResult, error) {
	startTime := time.Now()
	result := &domain.RulesLoadResult{
		Rules: &domain.LoadedRules{
			Files:      []domain.RuleFile{},
			LoadTime:   startTime,
			HasChanges: false,
		},
		Changes:  []domain.RulesChange{},
		Warnings: []string{},
	}

	// Resolve configuration paths
	config := l.resolvePaths(req)

	// Track current cache state for change detection
	newCache := make(map[string]time.Time)

	// Load organization-level rules
	if config.EnableOrganization && config.OrganizationRulesPath != "" {
		file, change := l.loadRuleFile(config.OrganizationRulesPath, domain.LevelOrganization, req.PreviousLoad)
		if file.Exists {
			result.Rules.Files = append(result.Rules.Files, file)
			if file.ModTime.IsZero() {
				result.Warnings = append(result.Warnings, "could not stat organization rules file")
			} else {
				newCache[file.Path] = file.ModTime
			}
		}
		if change != nil {
			result.Changes = append(result.Changes, *change)
		}
	}

	// Load user-level rules
	if config.EnableUser && config.UserRulesPath != "" {
		file, change := l.loadRuleFile(config.UserRulesPath, domain.LevelUser, req.PreviousLoad)
		if file.Exists {
			result.Rules.Files = append(result.Rules.Files, file)
			if !file.ModTime.IsZero() {
				newCache[file.Path] = file.ModTime
			}
		}
		if change != nil {
			result.Changes = append(result.Changes, *change)
		}
	}

	// Load project-level rules
	if config.EnableProject && config.ProjectRulesPath != "" {
		file, change := l.loadRuleFile(config.ProjectRulesPath, domain.LevelProject, req.PreviousLoad)
		if file.Exists {
			result.Rules.Files = append(result.Rules.Files, file)
			if !file.ModTime.IsZero() {
				newCache[file.Path] = file.ModTime
			}
		}
		if change != nil {
			result.Changes = append(result.Changes, *change)
		}
	}

	// Load module-level rules based on active file paths
	if config.EnableModule && config.ModuleRulesPath != "" && len(req.ActiveFilePaths) > 0 {
		moduleFiles, changes := l.loadModuleRules(config.ModuleRulesPath, req.ActiveFilePaths, req.PreviousLoad)
		for _, file := range moduleFiles {
			if file.Exists {
				result.Rules.Files = append(result.Rules.Files, file)
				if !file.ModTime.IsZero() {
					newCache[file.Path] = file.ModTime
				}
			}
		}
		result.Changes = append(result.Changes, changes...)

		// Track active file types
		result.Rules.ActiveFileTypes = l.extractFileTypes(req.ActiveFilePaths)
	}

	// Merge all loaded rules
	result.Rules.MergedContent = l.mergeRules(result.Rules.Files)

	// Update cache and check for changes
	l.mu.Lock()
	result.Rules.HasChanges = len(result.Changes) > 0
	l.cache.files = newCache
	l.cache.loadTime = startTime
	l.mu.Unlock()

	l.logger.Debug("rules loaded",
		"file_count", len(result.Rules.Files),
		"change_count", len(result.Changes),
		"active_file_types", result.Rules.ActiveFileTypes,
	)

	return result, nil
}

// resolvePaths resolves the configuration paths based on the request.
func (l *RulesLoader) resolvePaths(req *domain.RulesLoadRequest) domain.RulesConfig {
	config := l.config

	// Override project root if provided in request
	if req.ProjectRoot != "" {
		config.ProjectRoot = req.ProjectRoot
	}

	// Resolve project and module paths if project root is set
	if config.ProjectRoot != "" {
		if config.ProjectRulesPath == "" || strings.HasPrefix(config.ProjectRulesPath, "{project_root}") {
			config.ProjectRulesPath = filepath.Join(config.ProjectRoot, ".agent", "rules.md")
		}
		if config.ModuleRulesPath == "" || strings.HasPrefix(config.ModuleRulesPath, "{project_root}") {
			config.ModuleRulesPath = filepath.Join(config.ProjectRoot, ".agent", "rules")
		}
	}

	return config
}

// loadRuleFile loads a single rule file and detects changes.
func (l *RulesLoader) loadRuleFile(path string, level domain.RulesLevel, prevLoad *domain.LoadedRules) (domain.RuleFile, *domain.RulesChange) {
	file := domain.RuleFile{
		Level: level,
		Path:  path,
	}

	// Check if file exists
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		file.Exists = false
		// Check if it was deleted
		if prevModTime, wasCached := l.cache.files[path]; wasCached {
			return file, &domain.RulesChange{
				Path:      path,
				Level:     level,
				Type:      "deleted",
				OldModTime: &prevModTime,
			}
		}
		return file, nil
	}
	if err != nil {
		file.Exists = true
		file.LoadError = err.Error()
		return file, nil
	}

	file.Exists = true
	file.ModTime = stat.ModTime()

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		file.LoadError = err.Error()
		return file, nil
	}

	file.Content = string(content)

	// Parse frontmatter and body
	file.Frontmatter, file.Body = l.parseFrontmatter(content)

	// Detect changes
	var change *domain.RulesChange
	if prevModTime, wasCached := l.cache.files[path]; wasCached {
		if file.ModTime.After(prevModTime) {
			change = &domain.RulesChange{
				Path:      path,
				Level:     level,
				Type:      "modified",
				OldModTime: &prevModTime,
				NewModTime: &file.ModTime,
			}
		}
	} else {
		change = &domain.RulesChange{
			Path:      path,
			Level:     level,
			Type:      "created",
			NewModTime: &file.ModTime,
		}
	}

	return file, change
}

// loadModuleRules loads all module-level rules that match the active file paths.
func (l *RulesLoader) loadModuleRules(modulePath string, activeFilePaths []string, prevLoad *domain.LoadedRules) ([]domain.RuleFile, []domain.RulesChange) {
	var files []domain.RuleFile
	var changes []domain.RulesChange

	// Check if module directory exists
	stat, err := os.Stat(modulePath)
	if os.IsNotExist(err) || !stat.IsDir() {
		return files, changes
	}

	// Read all .md files in the module directory
	entries, err := os.ReadDir(modulePath)
	if err != nil {
		l.logger.Warn("failed to read module rules directory", "path", modulePath, "err", err)
		return files, changes
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(modulePath, entry.Name())

		// Load the rule file
		file, change := l.loadRuleFile(filePath, domain.LevelModule, prevLoad)

		// Skip if file doesn't exist or has error
		if !file.Exists || file.LoadError != "" {
			continue
		}

		// Check if this rule's paths match any active file
		if len(file.Frontmatter.Paths) == 0 {
			// No path restrictions - load for all file types
			files = append(files, file)
		} else {
			// Check if any active file matches the rule's paths
			for _, activePath := range activeFilePaths {
				if l.matchesAnyPattern(activePath, file.Frontmatter.Paths) {
					// Skip disabled rules
					if !file.Frontmatter.IsEnabled() {
						continue
					}
					files = append(files, file)
					break
				}
			}
		}

		if change != nil {
			changes = append(changes, *change)
		}
	}

	// Sort module files by priority (higher priority = loaded later)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].Frontmatter.Priority < files[j].Frontmatter.Priority {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	return files, changes
}

// parseFrontmatter extracts YAML frontmatter from the content.
// Returns the frontmatter and the body (content after frontmatter).
func (l *RulesLoader) parseFrontmatter(content []byte) (domain.RuleFrontmatter, string) {
	var frontmatter domain.RuleFrontmatter
	body := string(content)

	// Check for YAML frontmatter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return frontmatter, body
	}

	// Find closing delimiter
	endIdx := bytes.Index(content[4:], []byte("\n---\n"))
	if endIdx == -1 {
		return frontmatter, body
	}

	// Extract frontmatter YAML
	fmBytes := content[4 : endIdx+4]
	body = string(content[endIdx+8:]) // Skip both delimiters

	// Parse YAML
	if err := yaml.Unmarshal(fmBytes, &frontmatter); err != nil {
		l.logger.Warn("failed to parse rule frontmatter", "err", err)
		return frontmatter, body
	}

	return frontmatter, body
}

// matchesAnyPattern checks if the file path matches any of the glob patterns.
func (l *RulesLoader) matchesAnyPattern(filePath string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err != nil {
			l.logger.Warn("invalid glob pattern", "pattern", pattern, "err", err)
			continue
		}
		if matched {
			return true
		}

		// Also try matching against relative path
		matched, err = filepath.Match(pattern, filePath)
		if err != nil {
			continue
		}
		if matched {
			return true
		}

		// Try double-star pattern matching for subdirectories
		if strings.Contains(pattern, "**") {
			if l.matchDoubleStar(filePath, pattern) {
				return true
			}
		}
	}
	return false
}

// matchDoubleStar handles ** patterns in glob matching.
func (l *RulesLoader) matchDoubleStar(path, pattern string) bool {
	// Convert ** pattern to a more flexible match
	// e.g., "src/**/*.ts" should match "src/foo/bar.ts"
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}

	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	// Check prefix
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	// Check suffix
	if suffix != "" {
		// Extract pattern after last / in suffix for extension match
		if strings.Contains(suffix, "*") {
			extPattern := suffix
			if idx := strings.LastIndex(suffix, "/"); idx >= 0 {
				extPattern = suffix[idx+1:]
			}
			matched, _ := filepath.Match(extPattern, filepath.Base(path))
			return matched
		}
		return strings.HasSuffix(path, suffix)
	}

	return true
}

// mergeRules combines all rule files into a single string.
// Rules are merged in level order, with later levels appended.
func (l *RulesLoader) mergeRules(files []domain.RuleFile) string {
	var builder strings.Builder

	for _, file := range files {
		if !file.Exists || file.Body == "" {
			continue
		}

		// Add section header for each level
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}

		// Add level indicator as a comment
		builder.WriteString(l.formatRuleSection(file))
	}

	return builder.String()
}

// formatRuleSection formats a single rule file as a section.
func (l *RulesLoader) formatRuleSection(file domain.RuleFile) string {
	var builder strings.Builder

	// Add source comment
	switch file.Level {
	case domain.LevelOrganization:
		builder.WriteString("<!-- Organization Rules -->\n")
	case domain.LevelUser:
		builder.WriteString("<!-- User Rules -->\n")
	case domain.LevelProject:
		builder.WriteString("<!-- Project Rules -->\n")
	case domain.LevelModule:
		name := file.Frontmatter.Name
		if name == "" {
			name = filepath.Base(file.Path)
		}
		builder.WriteString("<!-- Module Rules: " + name + " -->\n")
	}

	// Add rule content
	builder.WriteString(strings.TrimSpace(file.Body))

	return builder.String()
}

// extractFileTypes extracts unique file extensions from the given paths.
func (l *RulesLoader) extractFileTypes(paths []string) []string {
	seen := make(map[string]bool)
	var types []string

	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" && !seen[ext] {
			seen[ext] = true
			types = append(types, ext)
		}
	}

	return types
}

// CheckForChanges checks if any rule files have changed since the last load.
func (l *RulesLoader) CheckForChanges() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for path, cachedTime := range l.cache.files {
		stat, err := os.Stat(path)
		if os.IsNotExist(err) {
			return true // File was deleted
		}
		if err != nil {
			continue
		}
		if stat.ModTime().After(cachedTime) {
			return true // File was modified
		}
	}

	return false
}

// GetCachedFiles returns the currently cached rule files.
func (l *RulesLoader) GetCachedFiles() map[string]time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]time.Time)
	for k, v := range l.cache.files {
		result[k] = v
	}
	return result
}

// RulesInjector handles injecting rules into system prompts.
type RulesInjector struct {
	config domain.RulesInjectionConfig
	logger *slog.Logger
}

// NewRulesInjector creates a new rules injector with the given configuration.
func NewRulesInjector(config domain.RulesInjectionConfig, logger *slog.Logger) *RulesInjector {
	if logger == nil {
		logger = slog.Default()
	}

	return &RulesInjector{
		config: config,
		logger: logger,
	}
}

// InjectRules injects rules content into the system instructions.
func (i *RulesInjector) InjectRules(systemInstructions string, rules *domain.LoadedRules) string {
	if !i.config.Enabled || rules == nil || rules.MergedContent == "" {
		return systemInstructions
	}

	// Format the rules section
	rulesSection := i.config.Header + rules.MergedContent
	if i.config.Footer != "" {
		rulesSection += "\n" + i.config.Footer
	}

	// Inject based on configuration
	switch i.config.InjectAt {
	case "start":
		return rulesSection + "\n\n" + systemInstructions
	case "end":
		return systemInstructions + "\n\n" + rulesSection
	case "replace_marker":
		// Find and replace content between markers
		startMarker := "<!-- RULES_START -->"
		endMarker := "<!-- RULES_END -->"
		startIdx := strings.Index(systemInstructions, startMarker)
		endIdx := strings.Index(systemInstructions, endMarker)

		if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
			return systemInstructions[:startIdx+len(startMarker)] +
				"\n" + rulesSection + "\n" +
				systemInstructions[endIdx:]
		}
		// Markers not found, fall back to start injection
		return rulesSection + "\n\n" + systemInstructions
	default:
		return rulesSection + "\n\n" + systemInstructions
	}
}

// UpdateConfig updates the injector configuration.
func (i *RulesInjector) UpdateConfig(config domain.RulesInjectionConfig) {
	i.config = config
}
