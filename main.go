package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nogo/chartup/internal/cache"
	"github.com/nogo/chartup/internal/checker"
	"github.com/nogo/chartup/internal/output"
	"github.com/nogo/chartup/internal/scanner"
)

var version = "dev"

func printUsage() {
	fmt.Fprintf(os.Stderr, `chartup - Check Helm charts and Docker images for updates

Usage:
  chartup [options] [directory]

Options:
  --verbose           Show all items (default: only updates)
  --refresh           Refresh cache with fresh lookups
  --editor <name>     Editor for clickable links (default: auto-detect)
                      Options: vscode, cursor, idea, sublime, zed, none
  --version           Show version
  --help              Show this help

Examples:
  chartup .                      Scan current directory
  chartup /path/to/charts        Scan specific directory
  chartup --refresh .            Force fresh lookups and update cache
  chartup --editor idea .        Use IntelliJ IDEA for links

Supported registries:
  Docker Hub, Quay.io, ghcr.io, gcr.io, registry.k8s.io

`)
}

func main() {
	flag.Usage = printUsage

	verbose := flag.Bool("verbose", false, "")
	refresh := flag.Bool("refresh", false, "")
	editor := flag.String("editor", "", "")
	showVersion := flag.Bool("version", false, "")
	showHelp := flag.Bool("help", false, "")
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("chartup %s\n", version)
		os.Exit(0)
	}

	// Get directory to scan
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	// Validate directory exists
	info, err := os.Stat(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", dir)
		os.Exit(1)
	}

	// Initialize cache (1 hour TTL)
	c := cache.New(".chartup-cache.json", 1*time.Hour, *refresh)
	if err := c.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load cache: %v\n", err)
	}

	// Scan directory for charts
	fmt.Printf("Scanning %s for Helm charts...\n\n", dir)
	results, err := scanner.Scan(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	if len(results.Charts) == 0 && len(results.Images) == 0 {
		fmt.Println("No Helm charts or Docker images found.")
		os.Exit(0)
	}

	// Check for updates
	chk := checker.New(c)
	updateResults, err := chk.CheckAll(results)
	if err != nil {
		if checker.IsRateLimitError(err) {
			fmt.Fprintf(os.Stderr, "\nError: Rate limit hit. Partial results shown below.\n")
			fmt.Fprintf(os.Stderr, "Try again later. Cached results will be used for 1 hour.\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error checking updates: %v\n", err)
			os.Exit(1)
		}
	}

	// Save cache
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save cache: %v\n", err)
	}

	// Set base directory for relative path display
	absDir, err := filepath.Abs(dir)
	if err == nil {
		output.SetBaseDir(absDir)
	}

	// Set editor for file links
	if *editor != "" {
		output.SetEditor(*editor)
	}

	// Set verbose mode
	output.SetVerbose(*verbose)

	// Output results
	output.PrintTable(updateResults)
}
