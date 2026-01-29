//go:build windows
// +build windows

// Package main provides automated testing for Cloud Files API (hydration/dehydration).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	// Parse flags
	var (
		listOnly    = flag.Bool("list", false, "List all test scenarios")
		runAll      = flag.Bool("run-all", false, "Run all tests")
		runTests    = flag.String("run", "", "Run specific tests (comma-separated: T1,T2,T3)")
		cleanup     = flag.Bool("cleanup", false, "Cleanup test directory after tests")
		verbose     = flag.Bool("v", false, "Verbose output")
		reconfigure = flag.Bool("reconfig", false, "Force reconfiguration")
	)
	flag.Parse()

	// List scenarios
	if *listOnly {
		listScenarios()
		return
	}

	// Load or create config
	cfg, err := LoadConfig()
	if err != nil || *reconfigure {
		if os.IsNotExist(err) || *reconfigure {
			cfg, err = PromptConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erreur configuration: %v\n", err)
				os.Exit(1)
			}
			if err := SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Erreur sauvegarde config: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Erreur chargement config: %v\n", err)
			os.Exit(1)
		}
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration invalide: %v\n", err)
		fmt.Println("Utilisez --reconfig pour reconfigurer")
		os.Exit(1)
	}

	// Determine which tests to run
	var testsToRun []string
	if *runAll {
		testsToRun = getAllTestIDs()
	} else if *runTests != "" {
		testsToRun = strings.Split(*runTests, ",")
		for i := range testsToRun {
			testsToRun[i] = strings.TrimSpace(testsToRun[i])
		}
	} else {
		// No tests specified, show help
		fmt.Println("Cloud Files Test Tool")
		fmt.Println("=====================")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  cloudfiles_test --list              List all test scenarios")
		fmt.Println("  cloudfiles_test --run-all           Run all tests")
		fmt.Println("  cloudfiles_test --run T1,T2         Run specific tests")
		fmt.Println("  cloudfiles_test --reconfig          Reconfigure settings")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -v        Verbose output")
		fmt.Println("  -cleanup  Cleanup test directory after tests")
		fmt.Println()
		fmt.Println("Current configuration:")
		fmt.Printf("  Sync root:  %s\n", cfg.LocalSyncRoot)
		fmt.Printf("  Test dir:   %s\n", cfg.LocalTestPath())
		fmt.Printf("  Remote:     %s\n", cfg.UNCPath())
		fmt.Printf("  Source:     %s\n", cfg.SourceDir)
		return
	}

	// Validate tests exist
	for _, id := range testsToRun {
		if !isValidTestID(id) {
			fmt.Fprintf(os.Stderr, "Test inconnu: %s\n", id)
			fmt.Println("Utilisez --list pour voir les tests disponibles")
			os.Exit(1)
		}
	}

	// Create runner
	runner, err := NewRunner(cfg, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur création runner: %v\n", err)
		os.Exit(1)
	}
	defer runner.Close()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nInterruption... Arrêt en cours...")
		cancel()
	}()

	// Run tests
	results := runner.RunTests(ctx, testsToRun)

	// Print report
	printReport(results)

	// Cleanup if requested
	if *cleanup {
		fmt.Println("\nNettoyage...")
		if err := runner.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Erreur nettoyage: %v\n", err)
		}
	}

	// Exit with error if any test failed
	for _, r := range results {
		if !r.Passed {
			os.Exit(1)
		}
	}
}

// listScenarios prints all available test scenarios.
func listScenarios() {
	scenarios := GetAllScenarios()

	fmt.Println()
	fmt.Println("Cloud Files Test Scenarios")
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Println()

	for _, s := range scenarios {
		fmt.Printf("  %s  %-30s  %s\n", s.ID, s.Name, s.Description)
	}

	fmt.Printf("\nTotal: %d scénarios\n\n", len(scenarios))
}

// getAllTestIDs returns all test IDs.
func getAllTestIDs() []string {
	scenarios := GetAllScenarios()
	ids := make([]string, len(scenarios))
	for i, s := range scenarios {
		ids[i] = s.ID
	}
	return ids
}

// isValidTestID checks if a test ID exists.
func isValidTestID(id string) bool {
	for _, s := range GetAllScenarios() {
		if s.ID == id {
			return true
		}
	}
	return false
}

// printReport prints the test results.
func printReport(results []TestResult) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                          RÉSULTATS                            ")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	passed := 0
	failed := 0

	for _, r := range results {
		status := "✓ PASS"
		if !r.Passed {
			status = "✗ FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("  %s  %-6s  %s\n", status, r.ID, r.Name)
		if r.Error != "" {
			fmt.Printf("           Erreur: %s\n", r.Error)
		}
		if r.Duration > 0 {
			fmt.Printf("           Durée: %v\n", r.Duration)
		}
	}

	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("  Total: %d tests  |  Passés: %d  |  Échoués: %d\n", len(results), passed, failed)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}
