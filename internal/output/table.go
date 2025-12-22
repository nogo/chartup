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

// editorScheme determines how file links are formatted
var editorScheme = ""

// verbose controls whether to show all items or only updates
var verbose = false

// SetBaseDir sets the base directory for relative path display
func SetBaseDir(dir string) {
	baseDir = dir
}

// SetEditor sets the editor scheme for hyperlinks
// Supported: "vscode", "idea", "sublime", "cursor", "zed", "none", or empty for auto-detect
func SetEditor(editor string) {
	editorScheme = editor
}

// SetVerbose sets whether to show all items or only updates
func SetVerbose(v bool) {
	verbose = v
}

// detectEditor tries to determine the editor from environment variables
func detectEditor() string {
	// Check VISUAL first (preferred for GUI editors), then EDITOR
	for _, envVar := range []string{"VISUAL", "EDITOR"} {
		if editor := os.Getenv(envVar); editor != "" {
			// Extract editor name from path
			editorName := filepath.Base(editor)
			// Remove common suffixes
			editorName = strings.TrimSuffix(editorName, ".exe")
			editorName = strings.TrimSuffix(editorName, ".cmd")
			editorName = strings.TrimSuffix(editorName, ".bat")

			switch {
			case strings.Contains(editorName, "code"):
				return "vscode"
			case strings.Contains(editorName, "cursor"):
				return "cursor"
			case strings.Contains(editorName, "zed"):
				return "zed"
			case strings.Contains(editorName, "idea") || strings.Contains(editorName, "intellij"):
				return "idea"
			case strings.Contains(editorName, "subl") || strings.Contains(editorName, "sublime"):
				return "sublime"
			case strings.Contains(editorName, "atom"):
				return "atom"
			case strings.Contains(editorName, "vim") || strings.Contains(editorName, "nvim"):
				return "none" // Terminal editors don't support URL schemes
			case strings.Contains(editorName, "nano") || strings.Contains(editorName, "emacs"):
				return "none" // Terminal editors don't support URL schemes
			}
		}
	}

	// Default to vscode as it's commonly installed
	return "vscode"
}

// getEditorScheme returns the effective editor scheme
func getEditorScheme() string {
	if editorScheme == "" {
		return detectEditor()
	}
	return editorScheme
}

// PrintTable prints the results as formatted tables using go-pretty
func PrintTable(results *checker.Results) {
	printImagesTables(results.Images)
	fmt.Println()
	printChartsTables(results.Charts)
	fmt.Println()
	printSummary(results)
}

// imagesByFile groups images by their file path
func imagesByFile(images []checker.ImageResult) map[string][]checker.ImageResult {
	grouped := make(map[string][]checker.ImageResult)
	for _, img := range images {
		path := img.Path
		if path == "" {
			path = "(unknown)"
		}
		grouped[path] = append(grouped[path], img)
	}
	return grouped
}

// chartsByFile groups charts by their file path
func chartsByFile(charts []checker.ChartResult) map[string][]checker.ChartResult {
	grouped := make(map[string][]checker.ChartResult)
	for _, chart := range charts {
		path := chart.Path
		if path == "" {
			path = "(unknown)"
		}
		grouped[path] = append(grouped[path], chart)
	}
	return grouped
}

func printImagesTables(images []checker.ImageResult) {
	if len(images) == 0 {
		fmt.Println("DOCKER IMAGES")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println("No Docker images found.")
		return
	}

	// Filter images if not verbose
	filtered := images
	if !verbose {
		filtered = make([]checker.ImageResult, 0)
		for _, img := range images {
			if img.Status == checker.StatusUpdateAvailable {
				filtered = append(filtered, img)
			}
		}
	}

	// Count updates for header
	updateCount := 0
	for _, img := range images {
		if img.Status == checker.StatusUpdateAvailable {
			updateCount++
		}
	}

	// Print header with count
	if verbose {
		fmt.Printf("DOCKER IMAGES - %d updates of %d total\n", updateCount, len(images))
	} else {
		fmt.Printf("DOCKER IMAGES - %d updates\n", updateCount)
	}
	fmt.Println(strings.Repeat("═", 80))

	if len(filtered) == 0 {
		fmt.Println("No updates available.")
		return
	}

	// Sort by file path, then line number
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Path != filtered[j].Path {
			return filtered[i].Path < filtered[j].Path
		}
		return filtered[i].Line < filtered[j].Line
	})

	// Create single table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if verbose {
		t.AppendHeader(table.Row{"Location", "Image", "Current", "Latest", "Status"})
	} else {
		t.AppendHeader(table.Row{"Location", "Image", "Current", "Latest"})
	}

	for _, img := range filtered {
		repo := img.Repository
		if img.Registry != "docker.io" && img.Registry != "" {
			repo = img.Registry + "/" + img.Repository
		}

		latest := img.Latest
		if img.Skipped {
			latest = "-"
		} else if latest != "" {
			// Add clickable link to registry
			latest = formatImageLatestLink(img.Registry, img.Repository, latest)
		}

		// Format location as relative/path:line with clickable link
		location := formatLocationLink(img.Path, img.Line)

		if verbose {
			status := formatStatus(img.Status)
			t.AppendRow(table.Row{location, repo, img.Current, latest, status})
		} else {
			t.AppendRow(table.Row{location, repo, img.Current, latest})
		}
	}

	if verbose {
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 5, Align: text.AlignCenter},
		})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
}

func printChartsTables(charts []checker.ChartResult) {
	if len(charts) == 0 {
		fmt.Println("HELM CHARTS")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println("No Helm charts found.")
		return
	}

	// Filter charts if not verbose
	filtered := charts
	if !verbose {
		filtered = make([]checker.ChartResult, 0)
		for _, chart := range charts {
			if chart.Status == checker.StatusUpdateAvailable {
				filtered = append(filtered, chart)
			}
		}
	}

	// Count updates for header
	updateCount := 0
	for _, chart := range charts {
		if chart.Status == checker.StatusUpdateAvailable {
			updateCount++
		}
	}

	// Print header with count
	if verbose {
		fmt.Printf("HELM CHARTS - %d updates of %d total\n", updateCount, len(charts))
	} else {
		fmt.Printf("HELM CHARTS - %d updates\n", updateCount)
	}
	fmt.Println(strings.Repeat("═", 80))

	if len(filtered) == 0 {
		fmt.Println("No updates available.")
		return
	}

	// Sort by file path, then line number
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Path != filtered[j].Path {
			return filtered[i].Path < filtered[j].Path
		}
		return filtered[i].Line < filtered[j].Line
	})

	// Create single table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if verbose {
		t.AppendHeader(table.Row{"Location", "Chart", "Current", "Latest", "Status"})
	} else {
		t.AppendHeader(table.Row{"Location", "Chart", "Current", "Latest"})
	}

	for _, chart := range filtered {
		latest := chart.Latest
		if chart.Status == checker.StatusSkipped {
			latest = "-"
		} else if latest != "" {
			// Add clickable link to ArtifactHub
			latest = formatChartLatestLink(chart.Name, chart.Upstream, latest)
		}

		// Format location as relative/path:line with clickable link
		location := formatLocationLink(chart.Path, chart.Line)

		if verbose {
			status := formatStatus(chart.Status)
			t.AppendRow(table.Row{location, chart.Name, chart.Current, latest, status})
		} else {
			t.AppendRow(table.Row{location, chart.Name, chart.Current, latest})
		}
	}

	if verbose {
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 5, Align: text.AlignCenter},
		})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
}

func printFileHeader(path string) {
	relPath := relativePath(path)
	absPath := path

	// Create clickable link using OSC 8 escape sequence
	scheme := getEditorScheme()
	link := makeEditorLink(absPath, 1)
	if link != "" && scheme != "none" {
		// OSC 8 hyperlink format: \e]8;;URL\e\\TEXT\e]8;;\e\\
		fmt.Printf("\033]8;;%s\033\\📄 %s\033]8;;\033\\\n", link, relPath)
	} else {
		fmt.Printf("📄 %s\n", relPath)
	}
}

func formatLineLink(path string, line int) string {
	if line <= 0 {
		return ""
	}

	lineStr := fmt.Sprintf("%d", line)

	// Create clickable link for line number
	scheme := getEditorScheme()
	link := makeEditorLink(path, line)
	if link != "" && scheme != "none" {
		// OSC 8 hyperlink format
		return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", link, lineStr)
	}

	return lineStr
}

// formatImageLatestLink creates a clickable link to the registry page for the tag
func formatImageLatestLink(registry, repository, tag string) string {
	if tag == "" || tag == "-" {
		return tag
	}

	var url string
	switch {
	case registry == "docker.io" || registry == "":
		// Docker Hub
		if strings.Contains(repository, "/") {
			url = fmt.Sprintf("https://hub.docker.com/r/%s/tags?name=%s", repository, tag)
		} else {
			// Official images
			url = fmt.Sprintf("https://hub.docker.com/_/%s/tags?name=%s", repository, tag)
		}
	case strings.Contains(registry, "quay.io"):
		url = fmt.Sprintf("https://quay.io/repository/%s?tab=tags&tag=%s", repository, tag)
	case strings.Contains(registry, "ghcr.io"):
		// GitHub Container Registry - link to package versions
		url = fmt.Sprintf("https://github.com/%s/pkgs/container/%s",
			strings.Split(repository, "/")[0],
			strings.Split(repository, "/")[len(strings.Split(repository, "/"))-1])
	case strings.Contains(registry, "gcr.io"):
		// GCR doesn't have a nice web UI for tags
		return tag
	case strings.Contains(registry, "registry.k8s.io"):
		// k8s registry doesn't have a web UI
		return tag
	default:
		return tag
	}

	// OSC 8 hyperlink format
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, tag)
}

// formatChartLatestLink creates a clickable link to ArtifactHub for the chart version
func formatChartLatestLink(name, upstream, version string) string {
	if version == "" || version == "-" {
		return version
	}

	var url string
	switch upstream {
	case "bitnami":
		url = fmt.Sprintf("https://artifacthub.io/packages/helm/bitnami/%s/%s", name, version)
	case "trinodb":
		url = fmt.Sprintf("https://artifacthub.io/packages/helm/trino/%s/%s", name, version)
	default:
		return version
	}

	// OSC 8 hyperlink format
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, version)
}

func formatLocationLink(path string, line int) string {
	relPath := relativePath(path)

	// Format as path:line
	var location string
	if line > 0 {
		location = fmt.Sprintf("%s:%d", relPath, line)
	} else {
		location = relPath
	}

	// Create clickable link
	scheme := getEditorScheme()
	link := makeEditorLink(path, line)
	if link != "" && scheme != "none" {
		// OSC 8 hyperlink format
		return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", link, location)
	}

	return location
}

func makeEditorLink(path string, line int) string {
	// Ensure absolute path
	absPath := path
	if !filepath.IsAbs(path) && baseDir != "" {
		absPath = filepath.Join(baseDir, path)
	}

	scheme := getEditorScheme()

	switch scheme {
	case "vscode":
		// vscode://file/path:line:column
		return fmt.Sprintf("vscode://file%s:%d:1", absPath, line)
	case "idea":
		// idea://open?file=/path&line=N
		return fmt.Sprintf("idea://open?file=%s&line=%d", absPath, line)
	case "sublime":
		// subl://open?url=file:///path&line=N
		return fmt.Sprintf("subl://open?url=file://%s&line=%d", absPath, line)
	case "cursor":
		// cursor://file/path:line:column
		return fmt.Sprintf("cursor://file%s:%d:1", absPath, line)
	case "zed":
		// zed://file/path:line
		return fmt.Sprintf("zed://file%s:%d", absPath, line)
	case "atom":
		// atom://open?url=file:///path&line=N
		return fmt.Sprintf("atom://open?url=file://%s&line=%d", absPath, line)
	case "none":
		return ""
	default:
		// Default to vscode
		return fmt.Sprintf("vscode://file%s:%d:1", absPath, line)
	}
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
)

func formatStatus(status checker.Status) string {
	switch status {
	case checker.StatusUpToDate:
		return colorGreen + "✓ OK" + colorReset
	case checker.StatusUpdateAvailable:
		return colorYellow + "⚠ UPDATE" + colorReset
	case checker.StatusSkipped:
		return colorGray + "⏭ SKIP" + colorReset
	case checker.StatusError:
		return colorGray + "✗ ERROR" + colorReset
	default:
		return colorGray + "? UNKNOWN" + colorReset
	}
}

func relativePath(path string) string {
	if path == "" || path == "(unknown)" {
		return path
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
	var updates, upToDate, skipped, errors, unknown int

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
		default:
			unknown++
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
		default:
			unknown++
		}
	}

	total := updates + upToDate + skipped + errors + unknown

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("SUMMARY")

	t.AppendRow(table.Row{"Updates available", colorYellow + fmt.Sprintf("%d", updates) + colorReset})
	t.AppendRow(table.Row{"Up to date", colorGreen + fmt.Sprintf("%d", upToDate) + colorReset})
	t.AppendRow(table.Row{"Skipped", colorGray + fmt.Sprintf("%d", skipped) + colorReset})
	if errors > 0 {
		t.AppendRow(table.Row{"Errors", colorGray + fmt.Sprintf("%d", errors) + colorReset})
	}
	if unknown > 0 {
		t.AppendRow(table.Row{"Unknown", colorGray + fmt.Sprintf("%d", unknown) + colorReset})
	}
	t.AppendSeparator()
	t.AppendRow(table.Row{"Total", fmt.Sprintf("%d", total)})

	t.SetStyle(table.StyleRounded)
	t.Style().Title.Align = text.AlignCenter
	t.Render()

	// Print hint about verbose mode
	if verbose {
		fmt.Printf("\n%sHint: Run without --verbose to show only updates%s\n", colorGray, colorReset)
	} else {
		fmt.Printf("\n%sHint: Run with --verbose to show all %d items%s\n", colorGray, total, colorReset)
	}
}
