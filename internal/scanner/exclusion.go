package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// ExclusionLevel represents the priority level of an exclusion rule
type ExclusionLevel int

const (
	LevelIndividual ExclusionLevel = iota // Highest priority - specific file/dir exclusions
	LevelJob                              // Job-specific pattern exclusions
	LevelGlobal                           // Global pattern exclusions (lowest priority)
)

// String returns the string representation of ExclusionLevel
func (l ExclusionLevel) String() string {
	switch l {
	case LevelIndividual:
		return "individual"
	case LevelJob:
		return "job"
	case LevelGlobal:
		return "global"
	default:
		return "unknown"
	}
}

// Pattern represents a compiled exclusion pattern
type Pattern struct {
	Raw   string         // Original pattern string
	Regex *regexp.Regexp // Compiled regex
	IsDir bool           // Whether pattern is for directories (ends with /)
}

// ExclusionResult indicates whether a file was excluded and why
type ExclusionResult struct {
	Excluded bool           // Whether file is excluded
	Level    ExclusionLevel // Which level caused exclusion
	Pattern  string         // Pattern that matched (if any)
	Reason   string         // Human-readable reason
}

// Excluder handles 3-level exclusion system
type Excluder struct {
	globalPatterns  []*Pattern                // Global exclusion patterns
	jobPatterns     map[int64][]*Pattern      // Job-specific patterns (jobID -> patterns)
	individualPaths map[int64]map[string]bool // Individual path exclusions (jobID -> path -> excluded)
	logger          *zap.Logger               // Logger
}

// DefaultExclusions represents the structure of default_exclusions.json
type DefaultExclusions struct {
	Version          string   `json:"version"`
	GlobalPatterns   []string `json:"global_patterns"`
	GlobalExtensions []string `json:"global_extensions"`
	Description      string   `json:"description"`
}

// NewExcluder creates a new Excluder instance
func NewExcluder(logger *zap.Logger) *Excluder {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Excluder{
		globalPatterns:  make([]*Pattern, 0),
		jobPatterns:     make(map[int64][]*Pattern),
		individualPaths: make(map[int64]map[string]bool),
		logger:          logger.With(zap.String("component", "excluder")),
	}
}

// LoadDefaultExclusions loads global exclusion patterns from default_exclusions.json
func (e *Excluder) LoadDefaultExclusions(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return WrapError(err, "read default exclusions file %s", configPath)
	}

	var defaults DefaultExclusions
	if err := json.Unmarshal(data, &defaults); err != nil {
		return WrapError(err, "parse default exclusions JSON")
	}

	// Compile global patterns
	for _, patternStr := range defaults.GlobalPatterns {
		pattern, err := compilePattern(patternStr)
		if err != nil {
			e.logger.Warn("failed to compile global pattern",
				zap.String("pattern", patternStr),
				zap.Error(err))
			continue
		}
		e.globalPatterns = append(e.globalPatterns, pattern)
	}

	// Convert extensions to patterns (e.g., ".tmp" -> "*.tmp")
	for _, ext := range defaults.GlobalExtensions {
		patternStr := "*" + ext
		pattern, err := compilePattern(patternStr)
		if err != nil {
			e.logger.Warn("failed to compile extension pattern",
				zap.String("extension", ext),
				zap.Error(err))
			continue
		}
		e.globalPatterns = append(e.globalPatterns, pattern)
	}

	e.logger.Info("loaded default exclusions",
		zap.Int("pattern_count", len(e.globalPatterns)))

	return nil
}

// AddJobPattern adds a job-specific exclusion pattern
func (e *Excluder) AddJobPattern(jobID int64, patternStr string) error {
	pattern, err := compilePattern(patternStr)
	if err != nil {
		return WrapError(ErrInvalidPattern, "compile job pattern %s", patternStr)
	}

	if e.jobPatterns[jobID] == nil {
		e.jobPatterns[jobID] = make([]*Pattern, 0)
	}
	e.jobPatterns[jobID] = append(e.jobPatterns[jobID], pattern)

	e.logger.Debug("added job pattern",
		zap.Int64("job_id", jobID),
		zap.String("pattern", patternStr))

	return nil
}

// AddIndividualPath adds an individual file/directory path exclusion
func (e *Excluder) AddIndividualPath(jobID int64, path string) {
	if e.individualPaths[jobID] == nil {
		e.individualPaths[jobID] = make(map[string]bool)
	}
	cleanPath := filepath.Clean(path)
	e.individualPaths[jobID][cleanPath] = true

	e.logger.Debug("added individual path exclusion",
		zap.Int64("job_id", jobID),
		zap.String("path", cleanPath))
}

// ShouldExclude checks if a file/directory should be excluded
// Priority: Individual > Job > Global
func (e *Excluder) ShouldExclude(jobID int64, path string, isDir bool) *ExclusionResult {
	cleanPath := filepath.Clean(path)
	baseName := filepath.Base(cleanPath)

	// Level 1: Check individual path exclusions (highest priority)
	if paths, exists := e.individualPaths[jobID]; exists {
		if paths[cleanPath] {
			return &ExclusionResult{
				Excluded: true,
				Level:    LevelIndividual,
				Pattern:  cleanPath,
				Reason:   "individually excluded path",
			}
		}
	}

	// Level 2: Check job-specific patterns
	if patterns, exists := e.jobPatterns[jobID]; exists {
		for _, pattern := range patterns {
			if matchPattern(pattern, baseName, cleanPath, isDir) {
				return &ExclusionResult{
					Excluded: true,
					Level:    LevelJob,
					Pattern:  pattern.Raw,
					Reason:   "matched job-specific pattern",
				}
			}
		}
	}

	// Level 3: Check global patterns (lowest priority)
	for _, pattern := range e.globalPatterns {
		if matchPattern(pattern, baseName, cleanPath, isDir) {
			return &ExclusionResult{
				Excluded: true,
				Level:    LevelGlobal,
				Pattern:  pattern.Raw,
				Reason:   "matched global pattern",
			}
		}
	}

	// Not excluded
	return &ExclusionResult{
		Excluded: false,
	}
}

// compilePattern compiles a glob pattern into a regex
func compilePattern(patternStr string) (*Pattern, error) {
	pattern := &Pattern{
		Raw:   patternStr,
		IsDir: strings.HasSuffix(patternStr, "/"),
	}

	// Remove trailing slash if present
	if pattern.IsDir {
		patternStr = strings.TrimSuffix(patternStr, "/")
	}

	// Convert glob to regex
	regexStr := globToRegex(patternStr)

	// Compile regex
	regex, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, WrapError(err, "compile pattern regex %s", patternStr)
	}

	pattern.Regex = regex
	return pattern, nil
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(glob string) string {
	// Escape special regex characters except * and ?
	var result strings.Builder
	result.WriteString("^")

	for i := 0; i < len(glob); i++ {
		ch := glob[i]
		switch ch {
		case '*':
			// Check for **
			if i+1 < len(glob) && glob[i+1] == '*' {
				result.WriteString(".*") // ** matches anything including /
				i++                      // Skip next *
			} else {
				result.WriteString("[^/\\\\]*") // * matches anything except directory separators
			}
		case '?':
			result.WriteString("[^/\\\\]") // ? matches single character except directory separators
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			result.WriteRune('\\')
			result.WriteRune(rune(ch))
		default:
			result.WriteRune(rune(ch))
		}
	}

	result.WriteString("$")
	return result.String()
}

// matchPattern checks if a path matches a pattern
func matchPattern(pattern *Pattern, baseName, fullPath string, isDir bool) bool {
	// If pattern is for directories only, check isDir flag
	if pattern.IsDir && !isDir {
		return false
	}

	// Match against basename first (most common case)
	if pattern.Regex.MatchString(baseName) {
		return true
	}

	// For patterns with path separators, match against full path
	if strings.ContainsAny(pattern.Raw, "/\\") {
		// Normalize path separators to /
		normalizedPath := filepath.ToSlash(fullPath)
		if pattern.Regex.MatchString(normalizedPath) {
			return true
		}
	}

	return false
}

// GetStatistics returns statistics about loaded exclusion rules
func (e *Excluder) GetStatistics() map[string]interface{} {
	jobCount := len(e.jobPatterns)
	individualCount := 0
	for _, paths := range e.individualPaths {
		individualCount += len(paths)
	}

	return map[string]interface{}{
		"global_patterns":  len(e.globalPatterns),
		"job_pattern_sets": jobCount,
		"individual_paths": individualCount,
	}
}
