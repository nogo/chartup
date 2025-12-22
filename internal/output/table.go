package output

import (
	"fmt"
	"strings"

	"github.com/nogo/chartup/internal/checker"
)

// PrintTable prints the results as a plain text table
func PrintTable(results *checker.Results) {
	// Print Docker Images section
	fmt.Println("DOCKER IMAGES")
	fmt.Println(strings.Repeat("=", 80))

	if len(results.Images) == 0 {
		fmt.Println("No Docker images found.")
	} else {
		// Calculate column widths
		repoWidth := 40
		tagWidth := 25
		latestWidth := 25
		statusWidth := 10

		// Header
		fmt.Printf("%-*s %-*s %-*s %-*s\n",
			repoWidth, "Repository",
			tagWidth, "Current",
			latestWidth, "Latest",
			statusWidth, "Status")
		fmt.Println(strings.Repeat("-", 80))

		// Rows
		for _, img := range results.Images {
			repo := img.Repository
			if img.Registry != "docker.io" && img.Registry != "" {
				repo = img.Registry + "/" + img.Repository
			}

			current := truncate(img.Current, tagWidth-1)
			latest := truncate(img.Latest, latestWidth-1)
			if img.Skipped {
				latest = "-"
			}

			status := formatStatus(img.Status)

			fmt.Printf("%-*s %-*s %-*s %s\n",
				repoWidth, truncate(repo, repoWidth-1),
				tagWidth, current,
				latestWidth, latest,
				status)
		}
	}

	fmt.Println()

	// Print Helm Charts section
	fmt.Println("HELM CHARTS")
	fmt.Println(strings.Repeat("=", 80))

	if len(results.Charts) == 0 {
		fmt.Println("No Helm charts found.")
	} else {
		// Calculate column widths
		nameWidth := 30
		upstreamWidth := 15
		currentWidth := 15
		latestWidth := 15
		statusWidth := 10

		// Header
		fmt.Printf("%-*s %-*s %-*s %-*s %-*s\n",
			nameWidth, "Chart",
			upstreamWidth, "Upstream",
			currentWidth, "Current",
			latestWidth, "Latest",
			statusWidth, "Status")
		fmt.Println(strings.Repeat("-", 80))

		// Rows
		for _, chart := range results.Charts {
			upstream := chart.Upstream
			if upstream == "" {
				upstream = "(local)"
			}

			latest := chart.Latest
			if chart.Status == checker.StatusSkipped {
				latest = "-"
			}

			status := formatStatus(chart.Status)

			fmt.Printf("%-*s %-*s %-*s %-*s %s\n",
				nameWidth, truncate(chart.Name, nameWidth-1),
				upstreamWidth, truncate(upstream, upstreamWidth-1),
				currentWidth, truncate(chart.Current, currentWidth-1),
				latestWidth, truncate(latest, latestWidth-1),
				status)
		}
	}

	fmt.Println()

	// Print summary
	printSummary(results)
}

func formatStatus(status checker.Status) string {
	switch status {
	case checker.StatusUpToDate:
		return "✓ OK"
	case checker.StatusUpdateAvailable:
		return "⚠ UPDATE"
	case checker.StatusSkipped:
		return "⏭ SKIP"
	case checker.StatusError:
		return "✗ ERROR"
	default:
		return "? UNKNOWN"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func printSummary(results *checker.Results) {
	var updates, upToDate, skipped, errors int

	for _, img := range results.Images {
		switch img.Status {
		case checker.StatusUpdateAvailable:
			updates++
		case checker.StatusUpToDate:
			upToDate++
		case checker.StatusSkipped:
			skipped++
		case checker.StatusError:
			errors++
		}
	}

	for _, chart := range results.Charts {
		switch chart.Status {
		case checker.StatusUpdateAvailable:
			updates++
		case checker.StatusUpToDate:
			upToDate++
		case checker.StatusSkipped:
			skipped++
		case checker.StatusError:
			errors++
		}
	}

	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Updates available: %d\n", updates)
	fmt.Printf("Up to date:        %d\n", upToDate)
	fmt.Printf("Skipped:           %d\n", skipped)
	if errors > 0 {
		fmt.Printf("Errors:            %d\n", errors)
	}
}
