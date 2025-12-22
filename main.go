package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nogo/chartup/internal/cache"
	"github.com/nogo/chartup/internal/checker"
	"github.com/nogo/chartup/internal/output"
	"github.com/nogo/chartup/internal/scanner"
)

var (
	version = "dev"
)

func main() {
	noCache := flag.Bool("no-cache", false, "Ignore cached results")
	cacheTTL := flag.Duration("cache-ttl", 1*time.Hour, "Cache validity duration (e.g., 1h, 30m, 24h)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

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

	// Initialize cache
	c := cache.New(".chartup-cache.json", *cacheTTL, *noCache)
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
			fmt.Fprintf(os.Stderr, "Try again later or use --cache-ttl to extend cache validity.\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error checking updates: %v\n", err)
			os.Exit(1)
		}
	}

	// Save cache
	if err := c.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save cache: %v\n", err)
	}

	// Output results
	output.PrintTable(updateResults)
}
