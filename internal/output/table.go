package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/nogo/chartup/internal/checker"
)

// baseDir is used to make paths relative
var baseDir string

// SetBaseDir sets the base directory for relative path display
func SetBaseDir(dir string) {
	baseDir = dir
}

// PrintTable prints the results as formatted tables using go-pretty
func PrintTable(results *checker.Results) {
	printImagesTable(results.Images)
	fmt.Println()
	printChartsTable(results.Charts)
	fmt.Println()
	printSummary(results)
}

func printImagesTable(images []checker.ImageResult) {
	if len(images) == 0 {
		fmt.Println("No Docker images found.")
		return
	}

	// Sort by file path, then by line number
	sorted := make([]checker.ImageResult, len(images))
	copy(sorted, images)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Line < sorted[j].Line
	})

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("DOCKER IMAGES")

	t.AppendHeader(table.Row{"Repository", "Current", "Latest", "Status", "Line", "File"})

	lastFile := ""
	for _, img := range sorted {
		repo := img.Repository
		if img.Registry != "docker.io" && img.Registry != "" {
			repo = img.Registry + "/" + img.Repository
		}

		latest := img.Latest
		if img.Skipped {
			latest = "-"
		}

		status := formatStatus(img.Status)
		relPath := relativePath(img.Path)

		// Show file path only for first row in group, add separator between groups
		fileDisplay := ""
		if relPath != lastFile {
			if lastFile != "" {
				t.AppendSeparator()
			}
			fileDisplay = relPath
			lastFile = relPath
		}

		// Format line number
		lineStr := ""
		if img.Line > 0 {
			lineStr = fmt.Sprintf("%d", img.Line)
		}

		t.AppendRow(table.Row{repo, img.Current, latest, status, lineStr, fileDisplay})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, WidthMax: 40, WidthMaxEnforcer: text.WrapSoft},
		{Number: 2, WidthMax: 20, WidthMaxEnforcer: text.WrapSoft},
		{Number: 3, WidthMax: 20, WidthMaxEnforcer: text.WrapSoft},
		{Number: 4, Align: text.AlignCenter},
		{Number: 5, Align: text.AlignRight},
		{Number: 6, WidthMax: 55, WidthMaxEnforcer: text.WrapSoft},
	})

	t.SetStyle(table.StyleRounded)
	t.Style().Title.Align = text.AlignCenter
	t.Render()
}

func printChartsTable(charts []checker.ChartResult) {
	if len(charts) == 0 {
		fmt.Println("No Helm charts found.")
		return
	}

	// Sort by file path, then by line number
	sorted := make([]checker.ChartResult, len(charts))
	copy(sorted, charts)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Line < sorted[j].Line
	})

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("HELM CHARTS")

	t.AppendHeader(table.Row{"Chart", "Upstream", "Current", "Latest", "Status", "File"})

	lastFile := ""
	for _, chart := range sorted {
		upstream := chart.Upstream
		if upstream == "" {
			upstream = "(local)"
		}

		latest := chart.Latest
		if chart.Status == checker.StatusSkipped {
			latest = "-"
		}

		status := formatStatus(chart.Status)
		relPath := relativePath(chart.Path)

		// Show file path only for first row in group, add separator between groups
		fileDisplay := ""
		if relPath != lastFile {
			if lastFile != "" {
				t.AppendSeparator()
			}
			fileDisplay = relPath
			lastFile = relPath
		}

		t.AppendRow(table.Row{chart.Name, upstream, chart.Current, latest, status, fileDisplay})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, WidthMax: 25},
		{Number: 2, WidthMax: 15},
		{Number: 3, WidthMax: 15},
		{Number: 4, WidthMax: 15},
		{Number: 5, Align: text.AlignCenter},
		{Number: 6, WidthMax: 55, WidthMaxEnforcer: text.WrapSoft},
	})

	t.SetStyle(table.StyleRounded)
	t.Style().Title.Align = text.AlignCenter
	t.Render()
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

func relativePath(path string) string {
	if path == "" {
		return "-"
	}

	relPath := path

	if baseDir != "" {
		if rel, err := filepath.Rel(baseDir, path); err == nil {
			// Only use relative path if it doesn't start with ".."
			if !strings.HasPrefix(rel, "..") {
				relPath = rel
			}
		}
	}

	// Shorten home directory if not already relative
	if relPath == path {
		if home, err := os.UserHomeDir(); err == nil {
			if strings.HasPrefix(path, home) {
				relPath = "~" + strings.TrimPrefix(path, home)
			}
		}
	}

	return relPath
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

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("SUMMARY")

	t.AppendRow(table.Row{"Updates available", updates})
	t.AppendRow(table.Row{"Up to date", upToDate})
	t.AppendRow(table.Row{"Skipped", skipped})
	if errors > 0 {
		t.AppendRow(table.Row{"Errors", errors})
	}

	t.SetStyle(table.StyleRounded)
	t.Style().Title.Align = text.AlignCenter
	t.Render()
}
