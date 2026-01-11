package scanner

import (
	"path/filepath"
	"testing"
)

func TestExcluder_GlobalPatterns(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	// Load default exclusions
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	err := excluder.LoadDefaultExclusions(configPath)
	h.AssertNoError(err, "load default exclusions")

	jobID := int64(1)

	// Test all patterns from default_exclusions.json
	tests := []struct {
		path     string
		isDir    bool
		excluded bool
		reason   string
	}{
		// Temporary files
		{"file.tmp", false, true, "*.tmp pattern"},
		{"file.temp", false, true, "*.temp pattern"},
		{"~$document.docx", false, true, "~$* pattern"},
		{".DS_Store", false, true, ".DS_Store pattern"},
		{"Thumbs.db", false, true, "Thumbs.db pattern"},
		{"desktop.ini", false, true, "desktop.ini pattern"},
		{"~lock.file", false, true, "~lock.* pattern"},

		// Version control (directories)
		{".git", true, true, ".git/ pattern"},
		{".svn", true, true, ".svn/ pattern"},
		{".hg", true, true, ".hg/ pattern"},
		{".bzr", true, true, ".bzr/ pattern"},

		// Development (directories)
		{"node_modules", true, true, "node_modules/ pattern"},
		{"__pycache__", true, true, "__pycache__/ pattern"},
		{".venv", true, true, ".venv/ pattern"},
		{"venv", true, true, "venv/ pattern"},
		{".env", false, true, ".env pattern"},

		// Editor files
		{"file.swp", false, true, "*.swp pattern"},
		{"file.swo", false, true, "*.swo pattern"},
		{"file~", false, true, "*~ pattern"},

		// Extensions
		{"backup.bak", false, true, ".bak extension"},
		{"old.old", false, true, ".old extension"},
		{"cache.cache", false, true, ".cache extension"},

		// Should NOT be excluded
		{"regular.txt", false, false, "regular file"},
		{"document.pdf", false, false, "pdf file"},
		{"src", true, false, "regular directory"},
		{"tmpfile.txt", false, false, "contains 'tmp' but doesn't match pattern"},
		{"git-info.txt", false, false, "contains 'git' but doesn't match pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := excluder.ShouldExclude(jobID, tt.path, tt.isDir)

			if result.Excluded != tt.excluded {
				t.Errorf("path %s: expected excluded=%v, got %v (reason: %s)",
					tt.path, tt.excluded, result.Excluded, tt.reason)
			}

			if result.Excluded && result.Level != LevelGlobal {
				t.Errorf("path %s: should be excluded at global level, got %s",
					tt.path, result.Level.String())
			}
		})
	}
}

func TestExcluder_JobPattern(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	jobID := int64(1)

	// Add job-specific pattern
	err := excluder.AddJobPattern(jobID, "*.log")
	h.AssertNoError(err, "add job pattern")

	// Test job pattern
	result := excluder.ShouldExclude(jobID, "app.log", false)
	if !result.Excluded {
		t.Error("*.log should be excluded for this job")
	}
	if result.Level != LevelJob {
		t.Errorf("expected job level exclusion, got %s", result.Level.String())
	}

	// Different job should not exclude
	result2 := excluder.ShouldExclude(99, "app.log", false)
	if result2.Excluded {
		t.Error("*.log should not be excluded for different job")
	}
}

func TestExcluder_IndividualPath(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	jobID := int64(1)

	// Add individual path exclusion
	specificPath := "/path/to/specific/file.txt"
	excluder.AddIndividualPath(jobID, specificPath)

	// Test individual path (highest priority)
	result := excluder.ShouldExclude(jobID, specificPath, false)
	if !result.Excluded {
		t.Error("individual path should be excluded")
	}
	if result.Level != LevelIndividual {
		t.Errorf("expected individual level exclusion, got %s", result.Level.String())
	}

	// Different path should not exclude
	result2 := excluder.ShouldExclude(jobID, "/path/to/other/file.txt", false)
	if result2.Excluded {
		t.Error("different path should not be excluded")
	}
}

func TestExcluder_Hierarchy(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	jobID := int64(1)

	// Load global patterns
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	err := excluder.LoadDefaultExclusions(configPath)
	h.AssertNoError(err, "load default exclusions")

	// Add job pattern
	err = excluder.AddJobPattern(jobID, "*.job")
	h.AssertNoError(err, "add job pattern")

	// Add individual path
	excluder.AddIndividualPath(jobID, "/specific/file.txt")

	// Test hierarchy
	tests := []struct {
		name          string
		path          string
		expectedLevel ExclusionLevel
	}{
		{"individual", "/specific/file.txt", LevelIndividual},
		{"job", "test.job", LevelJob},
		{"global", "file.tmp", LevelGlobal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := excluder.ShouldExclude(jobID, tt.path, false)
			if !result.Excluded {
				t.Errorf("path %s should be excluded", tt.path)
			}
			if result.Level != tt.expectedLevel {
				t.Errorf("expected level %s, got %s",
					tt.expectedLevel.String(), result.Level.String())
			}
		})
	}
}

func TestExcluder_NestedDirectories(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	err := excluder.LoadDefaultExclusions(configPath)
	h.AssertNoError(err, "load default exclusions")

	jobID := int64(1)

	// Test that directories themselves are excluded
	// Note: Files under excluded dirs are never seen during walk (dir is skipped)
	tests := []struct {
		path     string
		isDir    bool
		excluded bool
	}{
		{"project/.git", true, true},          // .git directory excluded
		{"project/node_modules", true, true},  // node_modules directory excluded
		{"app/__pycache__", true, true},       // __pycache__ directory excluded
		{"project/src", true, false},          // Regular directory not excluded
		{"project/src/main.go", false, false}, // Regular file not excluded
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := excluder.ShouldExclude(jobID, tt.path, tt.isDir)
			if result.Excluded != tt.excluded {
				t.Errorf("path %s (isDir=%v): expected excluded=%v, got %v",
					tt.path, tt.isDir, tt.excluded, result.Excluded)
			}
		})
	}
}

func TestExcluder_WildcardPatterns(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	jobID := int64(1)

	// Add wildcard patterns
	patterns := []string{
		"test_*.txt",     // test_1.txt, test_2.txt
		"*.backup.*",     // file.backup.old
		"temp??.log",     // temp01.log, temp99.log
	}

	for _, pattern := range patterns {
		err := excluder.AddJobPattern(jobID, pattern)
		h.AssertNoError(err, "add pattern %s", pattern)
	}

	tests := []struct {
		path     string
		excluded bool
	}{
		{"test_1.txt", true},
		{"test_abc.txt", true},
		{"file.backup.old", true},
		{"data.backup.bak", true},
		{"temp01.log", true},
		{"temp99.log", true},

		{"test.txt", false},         // No underscore
		{"testfile.txt", false},     // No underscore
		{"file.backup", false},      // Missing second extension
		{"temp1.log", false},        // Only one char after temp
		{"temp123.log", false},      // Three chars after temp
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := excluder.ShouldExclude(jobID, tt.path, false)
			if result.Excluded != tt.excluded {
				t.Errorf("path %s: expected excluded=%v, got %v",
					tt.path, tt.excluded, result.Excluded)
			}
		})
	}
}

func TestExcluder_DirectoryVsFile(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	jobID := int64(1)

	// Add directory-specific pattern (trailing slash)
	err := excluder.AddJobPattern(jobID, "temp/")
	h.AssertNoError(err, "add directory pattern")

	// Directory should be excluded
	result := excluder.ShouldExclude(jobID, "temp", true)
	if !result.Excluded {
		t.Error("directory 'temp' should be excluded")
	}

	// File with same name should NOT be excluded (pattern has trailing slash)
	result2 := excluder.ShouldExclude(jobID, "temp", false)
	if result2.Excluded {
		t.Error("file 'temp' should not be excluded by 'temp/' pattern")
	}
}

func TestExcluder_Statistics(t *testing.T) {
	h := NewTestHelpers(t)
	excluder := NewExcluder(h.GetTestLogger(false))

	// Load global patterns
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	err := excluder.LoadDefaultExclusions(configPath)
	h.AssertNoError(err, "load default exclusions")

	// Add job patterns
	excluder.AddJobPattern(1, "*.log")
	excluder.AddJobPattern(1, "*.tmp")
	excluder.AddJobPattern(2, "*.bak")

	// Add individual paths
	excluder.AddIndividualPath(1, "/path1")
	excluder.AddIndividualPath(1, "/path2")
	excluder.AddIndividualPath(2, "/path3")

	stats := excluder.GetStatistics()

	// Should have global patterns (from default_exclusions.json)
	globalCount := stats["global_patterns"].(int)
	if globalCount == 0 {
		t.Error("should have global patterns loaded")
	}

	// Should have 2 job pattern sets
	jobSets := stats["job_pattern_sets"].(int)
	h.AssertEqual(2, jobSets, "job pattern sets")

	// Should have 3 individual paths
	individualCount := stats["individual_paths"].(int)
	h.AssertEqual(3, individualCount, "individual paths")
}

// --- Benchmarks ---

func BenchmarkExcluder_CheckPath(b *testing.B) {
	excluder := NewExcluder(nil)
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	if err := excluder.LoadDefaultExclusions(configPath); err != nil {
		b.Skip("cannot load default exclusions:", err)
	}

	jobID := int64(1)
	testPath := "src/main.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		excluder.ShouldExclude(jobID, testPath, false)
	}
}

func BenchmarkExcluder_CheckMultiplePaths(b *testing.B) {
	excluder := NewExcluder(nil)
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	if err := excluder.LoadDefaultExclusions(configPath); err != nil {
		b.Skip("cannot load default exclusions:", err)
	}

	jobID := int64(1)
	paths := []string{
		"src/main.go",
		"node_modules/package.json",
		"file.tmp",
		".git/config",
		"README.md",
		"test.log",
		"data.json",
		"__pycache__/module.pyc",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			excluder.ShouldExclude(jobID, path, false)
		}
	}
}

func BenchmarkExcluder_LargeBatchCheck(b *testing.B) {
	excluder := NewExcluder(nil)
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	if err := excluder.LoadDefaultExclusions(configPath); err != nil {
		b.Skip("cannot load default exclusions:", err)
	}

	jobID := int64(1)

	// Generate 10000 test paths
	paths := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		paths[i] = filepath.Join("src", "package", "file"+string(rune(i))+".go")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			excluder.ShouldExclude(jobID, path, false)
		}
	}
}
