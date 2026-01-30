package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// TestResult represents the result of a single test scenario.
type TestResult struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Job         string       `json:"job"`
	Description string       `json:"description"`
	Passed      bool         `json:"passed"`
	Duration    time.Duration `json:"duration_ms"`
	Error       string       `json:"error,omitempty"`
	Actions     []Action     `json:"actions,omitempty"`
	Validations []Validation `json:"validations,omitempty"`
	StartTime   time.Time    `json:"-"`
}

// MarshalJSON custom marshaler to convert duration to milliseconds.
func (r TestResult) MarshalJSON() ([]byte, error) {
	type Alias TestResult
	return json.Marshal(&struct {
		Duration int64 `json:"duration_ms"`
		*Alias
	}{
		Duration: r.Duration.Milliseconds(),
		Alias:    (*Alias)(&r),
	})
}

// Report represents the full test report.
type Report struct {
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration_ms"`
	Summary     ReportSummary     `json:"summary"`
	Results     []*TestResult     `json:"results"`
	Environment ReportEnvironment `json:"environment"`
}

// MarshalJSON custom marshaler for Report.
func (r Report) MarshalJSON() ([]byte, error) {
	type Alias Report
	return json.Marshal(&struct {
		Duration int64 `json:"duration_ms"`
		*Alias
	}{
		Duration: r.Duration.Milliseconds(),
		Alias:    (*Alias)(&r),
	})
}

// ReportSummary contains summary statistics.
type ReportSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// ReportEnvironment contains environment info.
type ReportEnvironment struct {
	LocalBase   string `json:"local_base"`
	RemoteBase  string `json:"remote_base"`
	GoVersion   string `json:"go_version"`
	Platform    string `json:"platform"`
}

// Reporter handles test output and reporting.
type Reporter struct {
	baseDir   string
	verbose   bool
	results   []*TestResult
	startTime time.Time
	currentJob string
}

// NewReporter creates a new reporter.
func NewReporter(baseDir string, verbose bool) *Reporter {
	return &Reporter{
		baseDir: baseDir,
		verbose: verbose,
		results: make([]*TestResult, 0),
	}
}

// Start begins the test session.
func (r *Reporter) Start() {
	r.startTime = time.Now()
	r.printHeader()
}

// printHeader prints the test header.
func (r *Reporter) printHeader() {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println("  AnemoneSync Test Harness")
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println()
}

// LogScenarioStart logs the start of a scenario.
func (r *Reporter) LogScenarioStart(scenario Scenario) {
	// Print job header if changed
	if scenario.Job != r.currentJob {
		r.currentJob = scenario.Job
		fmt.Printf("\n[%s] %s\n", scenario.Job, r.getJobDescription(scenario.Job))
	}

	if r.verbose {
		fmt.Printf("  → %s: %s\n", scenario.ID, scenario.Name)
	}
}

// LogScenarioEnd logs the end of a scenario.
func (r *Reporter) LogScenarioEnd(result *TestResult) {
	if result.Passed {
		fmt.Printf("  ✓ %s %-35s (%s)\n", result.ID, result.Name, formatDuration(result.Duration))
	} else {
		fmt.Printf("  ✗ %s %-35s (%s)\n", result.ID, result.Name, formatDuration(result.Duration))
		if result.Error != "" {
			fmt.Printf("        → %s\n", result.Error)
		}
	}
}

// AddResult adds a test result.
func (r *Reporter) AddResult(result *TestResult) {
	r.results = append(r.results, result)
}

// Finish generates the final report.
func (r *Reporter) Finish() error {
	duration := time.Since(r.startTime)

	// Calculate summary
	summary := ReportSummary{Total: len(r.results)}
	for _, result := range r.results {
		if result.Passed {
			summary.Passed++
		} else {
			summary.Failed++
		}
	}

	// Print summary
	r.printSummary(summary, duration)

	// Create report
	report := Report{
		Timestamp: r.startTime,
		Duration:  duration,
		Summary:   summary,
		Results:   r.results,
		Environment: ReportEnvironment{
			LocalBase:  r.baseDir,
			RemoteBase: "", // Will be set by config
			GoVersion:  runtime.Version(),
			Platform:   runtime.GOOS + "/" + runtime.GOARCH,
		},
	}

	// Save report
	return r.saveReport(report)
}

// printSummary prints the final summary.
func (r *Reporter) printSummary(summary ReportSummary, duration time.Duration) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════")

	passRate := float64(summary.Passed) / float64(summary.Total) * 100
	if summary.Failed == 0 {
		fmt.Printf("  RÉSULTATS: %d/%d passés (%.1f%%) ✓\n", summary.Passed, summary.Total, passRate)
	} else {
		fmt.Printf("  RÉSULTATS: %d/%d passés (%.1f%%) - %d échecs\n", summary.Passed, summary.Total, passRate, summary.Failed)
	}

	fmt.Printf("  Durée totale: %s\n", formatDuration(duration))

	// Report path
	reportPath := r.getReportPath()
	fmt.Printf("  Rapport: %s\n", reportPath)

	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println()
}

// saveReport saves the report to JSON file.
func (r *Reporter) saveReport(report Report) error {
	reportPath := r.getReportPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(reportPath, data, 0644)
}

// getReportPath returns the report file path.
func (r *Reporter) getReportPath() string {
	timestamp := r.startTime.Format("2006-01-02_150405")
	return filepath.Join(ResultsDir(r.baseDir), fmt.Sprintf("report_%s.json", timestamp))
}

// getJobDescription returns a human description for a job.
func (r *Reporter) getJobDescription(job string) string {
	descriptions := map[string]string{
		"TEST1": "Mirror bidirectionnel",
		"TEST2": "PC → Serveur",
		"TEST3": "Serveur → PC",
		"TEST4": "Conflits",
		"TEST5": "Stress/Volume",
		"TEST6": "Edge cases",
	}
	if desc, ok := descriptions[job]; ok {
		return desc
	}
	return job
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// PrintFailedTests prints details of failed tests.
func (r *Reporter) PrintFailedTests() {
	failed := make([]*TestResult, 0)
	for _, result := range r.results {
		if !result.Passed {
			failed = append(failed, result)
		}
	}

	if len(failed) == 0 {
		return
	}

	fmt.Println("\n─── Tests échoués ───")
	for _, result := range failed {
		fmt.Printf("\n[%s] %s - %s\n", result.Job, result.ID, result.Name)
		fmt.Printf("  Erreur: %s\n", result.Error)
		if len(result.Validations) > 0 {
			fmt.Println("  Validations:")
			for _, v := range result.Validations {
				status := "✓"
				if !v.Passed {
					status = "✗"
				}
				fmt.Printf("    %s %s: attendu=%s, obtenu=%s\n", status, v.Check, v.Expected, v.Actual)
			}
		}
	}
}
