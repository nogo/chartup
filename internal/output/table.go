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

// SetBaseDir sets the base directory for relative path display
func SetBaseDir(dir string) {
	baseDir = dir
}

// SetEditor sets the editor scheme for hyperlinks
// Supported: "vscode", "idea", "sublime", "cursor", "zed", "none", or empty for auto-detect
func SetEditor(editor string) {
	editorScheme = editor
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
	fmt.Println("DOCKER IMAGES")
	fmt.Println(strings.Repeat("═", 80))

	if len(images) == 0 {
		fmt.Println("No Docker images found.")
		return
	}

	// Group by file
	grouped := imagesByFile(images)

	// Sort file paths
	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for i, path := range paths {
		if i > 0 {
			fmt.Println()
		}

		// Print file header with link
		printFileHeader(path)

		// Sort images by line number
		imgs := grouped[path]
		sort.Slice(imgs, func(a, b int) bool {
			return imgs[a].Line < imgs[b].Line
		})

		// Create table for this file
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)

		t.AppendHeader(table.Row{"Repository", "Current", "Latest", "Status", "Line"})

		for _, img := range imgs {
			repo := img.Repository
			if img.Registry != "docker.io" && img.Registry != "" {
				repo = img.Registry + "/" + img.Repository
			}

			latest := img.Latest
			if img.Skipped {
				latest = "-"
			}

			status := formatStatus(img.Status)
			lineStr := formatLineLink(path, img.Line)

			t.AppendRow(table.Row{repo, img.Current, latest, status, lineStr})
		}

		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 1, WidthMax: 45, WidthMaxEnforcer: text.WrapSoft},
			{Number: 2, WidthMax: 22, WidthMaxEnforcer: text.WrapSoft},
			{Number: 3, WidthMax: 22, WidthMaxEnforcer: text.WrapSoft},
			{Number: 4, Align: text.AlignCenter},
			{Number: 5, Align: text.AlignRight},
		})

		t.SetStyle(table.StyleLight)
		t.Render()
	}
}

func printChartsTables(charts []checker.ChartResult) {
	fmt.Println("HELM CHARTS")
	fmt.Println(strings.Repeat("═", 80))

	if len(charts) == 0 {
		fmt.Println("No Helm charts found.")
		return
	}

	// Group by file
	grouped := chartsByFile(charts)

	// Sort file paths
	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for i, path := range paths {
		if i > 0 {
			fmt.Println()
		}

		// Print file header with link
		printFileHeader(path)

		// Sort charts by line number
		chts := grouped[path]
		sort.Slice(chts, func(a, b int) bool {
			return chts[a].Line < chts[b].Line
		})

		// Create table for this file
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)

		t.AppendHeader(table.Row{"Chart", "Upstream", "Current", "Latest", "Status"})

		for _, chart := range chts {
			upstream := chart.Upstream
			if upstream == "" {
				upstream = "(local)"
			}

			latest := chart.Latest
			if chart.Status == checker.StatusSkipped {
				latest = "-"
			}

			status := formatStatus(chart.Status)

			t.AppendRow(table.Row{chart.Name, upstream, chart.Current, latest, status})
		}

		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 1, WidthMax: 25},
			{Number: 2, WidthMax: 15},
			{Number: 3, WidthMax: 15},
			{Number: 4, WidthMax: 15},
			{Number: 5, Align: text.AlignCenter},
		})

		t.SetStyle(table.StyleLight)
		t.Render()
	}
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
