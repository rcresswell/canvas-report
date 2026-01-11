// ABOUTME: CLI entry point for canvas-report.
// ABOUTME: Handles config setup and orchestrates report generation.

package main

import (
	"fmt"
	"os"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			cfg, err = runSetup()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	showAll := false
	for _, arg := range os.Args[1:] {
		if arg == "--all" {
			showAll = true
		}
	}

	client := NewCanvasClient(cfg.BaseURL, cfg.AccessToken)
	report := NewReport(client, showAll)

	if err := report.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
