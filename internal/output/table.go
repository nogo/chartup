package output

import (
	"fmt"
	"os"
	"path/filepath"
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
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("DOCKER IMAGES")

	t.AppendHeader(table.Row{"Repository", "Current", "Latest", "Status", "File"})

	for _, img := range images {
		repo := img.Repository
		if img.Registry != "docker.io" && img.Registry != "" {
			repo = img.Registry + "/" + img.Repository
		}

		latest := img.Latest
		if img.Skipped {
			latest = "-"
		}

		status := formatStatus(img.Status)
		location := formatLocation(img.Path, img.Line)

		t.AppendRow(table.Row{repo, img.Current, latest, status, location})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, WidthMax: 45, WidthMaxEnforcer: text.WrapSoft},
		{Number: 2, WidthMax: 25, WidthMaxEnforcer: text.WrapSoft},
		{Number: 3, WidthMax: 25, WidthMaxEnforcer: text.WrapSoft},
		{Number: 4, Align: text.AlignCenter},
		{Number: 5, WidthMax: 50, WidthMaxEnforcer: text.WrapSoft},
	})

	t.SetStyle(table.StyleRounded)
	t.Style().Title.Align = text.AlignCenter

	if len(images) == 0 {
		fmt.Println("No Docker images found.")
	} else {
		t.Render()
	}
}

func printChartsTable(charts []checker.ChartResult) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("HELM CHARTS")

	t.AppendHeader(table.Row{"Chart", "Upstream", "Current", "Latest", "Status", "File"})

	for _, chart := range charts {
		upstream := chart.Upstream
		if upstream == "" {
			upstream = "(local)"
		}

		latest := chart.Latest
		if chart.Status == checker.StatusSkipped {
			latest = "-"
		}

		status := formatStatus(chart.Status)
		location := formatLocation(chart.Path, chart.Line)

		t.AppendRow(table.Row{chart.Name, upstream, chart.Current, latest, status, location})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, WidthMax: 25},
		{Number: 2, WidthMax: 15},
		{Number: 3, WidthMax: 15},
		{Number: 4, WidthMax: 15},
		{Number: 5, Align: text.AlignCenter},
		{Number: 6, WidthMax: 50, WidthMaxEnforcer: text.WrapSoft},
	})

	t.SetStyle(table.StyleRounded)
	t.Style().Title.Align = text.AlignCenter

	if len(charts) == 0 {
		fmt.Println("No Helm charts found.")
	} else {
		t.Render()
	}
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

func formatLocation(path string, line int) string {
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

	// Append line number if known
	if line > 0 {
		return fmt.Sprintf("%s:%d", relPath, line)
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
