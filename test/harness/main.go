package harness

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Main is the entry point for the test harness.
func Main() {
	// Parse flags
	var (
		baseDir        = flag.String("dir", "D:\\TEST", "Base directory for tests")
		jobFilter      = flag.String("job", "", "Run only this job (TEST1, TEST2, etc.)")
		scenarioFilter = flag.String("scenario", "", "Run only this scenario (1.1, 2.3, etc.)")
		verbose        = flag.Bool("v", false, "Verbose output")
		listOnly       = flag.Bool("list", false, "List all scenarios without running")
	)
	flag.Parse()

	// List scenarios if requested
	if *listOnly {
		listScenarios()
		return
	}

	// Load or create config
	cfg, err := LoadConfig(*baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Prompt for config
			cfg, err = PromptConfig(*baseDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erreur configuration: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Erreur chargement config: %v\n", err)
			os.Exit(1)
		}
	}

	// Create harness
	harness, err := New(cfg, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur création harness: %v\n", err)
		os.Exit(1)
	}
	defer harness.Close()

	// Test connection
	if err := harness.TestConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur connexion: %v\n", err)

		// Offer to reconfigure
		fmt.Print("\nVoulez-vous reconfigurer? (o/n): ")
		var answer string
		fmt.Scanln(&answer)
		if answer == "o" || answer == "O" || answer == "oui" {
			cfg, err = PromptConfig(*baseDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erreur configuration: %v\n", err)
				os.Exit(1)
			}
			if err := SaveConfig(*baseDir, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Erreur sauvegarde config: %v\n", err)
				os.Exit(1)
			}

			// Retry
			harness.Close()
			harness, err = New(cfg, *verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erreur création harness: %v\n", err)
				os.Exit(1)
			}
			if err := harness.TestConnection(); err != nil {
				fmt.Fprintf(os.Stderr, "Erreur connexion: %v\n", err)
				os.Exit(1)
			}
		} else {
			os.Exit(1)
		}
	}

	// Save config if new
	if err := SaveConfig(*baseDir, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Avertissement: impossible de sauvegarder config: %v\n", err)
	}

	// Initialize the real sync engine
	if err := harness.InitEngine(); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur initialisation engine: %v\n", err)
		os.Exit(1)
	}

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
	if err := harness.Run(ctx, *jobFilter, *scenarioFilter); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur: %v\n", err)
		os.Exit(1)
	}

	// Print failed tests details
	harness.Reporter.PrintFailedTests()
}

// listScenarios prints all available scenarios.
func listScenarios() {
	scenarios := GetAllScenarios()

	fmt.Println("\nScénarios disponibles:")
	fmt.Println("══════════════════════════════════════════════════════════")

	currentJob := ""
	for _, s := range scenarios {
		if s.Job != currentJob {
			currentJob = s.Job
			fmt.Printf("\n[%s]\n", currentJob)
		}
		fmt.Printf("  %s - %s\n", s.ID, s.Name)
	}

	fmt.Printf("\nTotal: %d scénarios\n\n", len(scenarios))
}
