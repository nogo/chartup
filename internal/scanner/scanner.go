package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartInfo holds information about a Helm chart
type ChartInfo struct {
	Name       string
	Version    string
	AppVersion string
	Path       string
	Line       int    // Line number in file
	Upstream   string // Known upstream source (e.g., "bitnami", "trinodb")
}

// ImageInfo holds information about a Docker image
type ImageInfo struct {
	Registry   string // e.g., "docker.io", "quay.io"
	Repository string // e.g., "trinodb/trino"
	Tag        string // e.g., "410"
	FullImage  string // Original full image string
	Path       string // File where it was found
	Line       int    // Line number in file
	Skipped    bool   // True for images we don't check (e.g., thinkportgmbh)
}

// ScanResults holds all discovered charts and images
type ScanResults struct {
	Charts []ChartInfo
	Images []ImageInfo
}

// Chart.yaml structure
type chartYAML struct {
	Name         string            `yaml:"name"`
	Version      string            `yaml:"version"`
	AppVersion   string            `yaml:"appVersion"`
	Dependencies []chartDependency `yaml:"dependencies"`
}

type chartDependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
}

// Scan recursively scans a directory for Helm charts and Docker images
func Scan(root string) (*ScanResults, error) {
	results := &ScanResults{
		Charts: []ChartInfo{},
		Images: []ImageInfo{},
	}

	seenImages := make(map[string]bool)
	seenCharts := make(map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if info.IsDir() {
			return nil
		}

		filename := info.Name()

		// Parse Chart.yaml files
		if filename == "Chart.yaml" {
			charts, err := parseChartYAML(path)
			if err == nil {
				for _, c := range charts {
					key := c.Name + "@" + c.Version
					if !seenCharts[key] {
						seenCharts[key] = true
						results.Charts = append(results.Charts, c)
					}
				}
			}
		}

		// Parse values.yaml files for images
		if filename == "values.yaml" {
			images, err := parseValuesYAML(path)
			if err == nil {
				for _, img := range images {
					if !seenImages[img.FullImage] {
						seenImages[img.FullImage] = true
						results.Images = append(results.Images, img)
					}
				}
			}
		}

		// Parse Dockerfiles for images
		if isDockerfile(filename) {
			images, err := parseDockerfile(path)
			if err == nil {
				for _, img := range images {
					if !seenImages[img.FullImage] {
						seenImages[img.FullImage] = true
						results.Images = append(results.Images, img)
					}
				}
			}
		}

		return nil
	})

	return results, err
}

func parseChartYAML(path string) ([]ChartInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var chart chartYAML
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, err
	}

	charts := []ChartInfo{}

	// Add main chart with upstream detection
	mainChart := ChartInfo{
		Name:       chart.Name,
		Version:    chart.Version,
		AppVersion: chart.AppVersion,
		Path:       path,
		Upstream:   detectUpstream(chart.Name, path),
	}
	charts = append(charts, mainChart)

	// Add dependencies with their upstreams
	for _, dep := range chart.Dependencies {
		upstream := ""
		if strings.Contains(dep.Repository, "bitnami") {
			upstream = "bitnami"
		}
		charts = append(charts, ChartInfo{
			Name:     dep.Name,
			Version:  dep.Version,
			Path:     path,
			Upstream: upstream,
		})
	}

	return charts, nil
}

// detectUpstream tries to identify known upstream sources for a chart
func detectUpstream(name, path string) string {
	nameLower := strings.ToLower(name)
	pathLower := strings.ToLower(path)

	// Known upstreams
	switch {
	case nameLower == "trino":
		return "trinodb"
	case nameLower == "postgresql" && strings.Contains(pathLower, "bitnami"):
		return "bitnami"
	case nameLower == "common" && strings.Contains(pathLower, "bitnami"):
		return "bitnami"
	case strings.Contains(pathLower, "/charts/postgresql"):
		return "bitnami"
	case strings.Contains(pathLower, "/charts/common"):
		return "bitnami"
	}

	return "" // Local/custom chart
}

func parseValuesYAML(path string) ([]ImageInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Use yaml.Node to preserve line numbers
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	images := []ImageInfo{}

	// Extract images from YAML nodes (preserves line numbers)
	if len(root.Content) > 0 {
		extractImagesFromNode(root.Content[0], path, &images)
	}

	return images, nil
}

// extractImagesFromNode extracts images from yaml.Node tree, preserving line numbers
func extractImagesFromNode(node *yaml.Node, path string, images *[]ImageInfo) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		// Process key-value pairs
		for i := 0; i < len(node.Content)-1; i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			// Check for repository/tag pattern
			if keyNode.Value == "repository" && valueNode.Kind == yaml.ScalarNode {
				repo := valueNode.Value
				tag := "latest"
				line := valueNode.Line

				// Look for sibling "tag" key
				for j := 0; j < len(node.Content)-1; j += 2 {
					if node.Content[j].Value == "tag" {
						tagNode := node.Content[j+1]
						if tagNode.Kind == yaml.ScalarNode && tagNode.Value != "" {
							tag = tagNode.Value
						}
						break
					}
				}

				img := parseImageString(repo+":"+tag, path, line)
				if img != nil {
					*images = append(*images, *img)
				}
			}

			// Check for "image" key with string value
			if keyNode.Value == "image" && valueNode.Kind == yaml.ScalarNode {
				img := parseImageString(valueNode.Value, path, valueNode.Line)
				if img != nil {
					*images = append(*images, *img)
				}
			}

			// Recurse into value nodes
			extractImagesFromNode(valueNode, path, images)
		}

	case yaml.SequenceNode:
		for _, item := range node.Content {
			extractImagesFromNode(item, path, images)
		}

	case yaml.DocumentNode:
		for _, item := range node.Content {
			extractImagesFromNode(item, path, images)
		}
	}
}

func parseImageString(imageStr, path string, line int) *ImageInfo {
	imageStr = strings.TrimSpace(imageStr)
	if imageStr == "" || imageStr == "latest" {
		return nil
	}

	// Skip common non-image values
	if strings.HasPrefix(imageStr, "/") || strings.HasPrefix(imageStr, ".") {
		return nil
	}
	if !strings.Contains(imageStr, "/") && !strings.Contains(imageStr, ":") {
		return nil
	}

	img := &ImageInfo{
		FullImage: imageStr,
		Path:      path,
		Line:      line,
		Registry:  "docker.io",
	}

	// Parse registry
	parts := strings.SplitN(imageStr, "/", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		img.Registry = parts[0]
		imageStr = parts[1]
	}

	// Parse repository and tag
	if strings.Contains(imageStr, ":") {
		tagParts := strings.SplitN(imageStr, ":", 2)
		img.Repository = tagParts[0]
		img.Tag = tagParts[1]
	} else {
		img.Repository = imageStr
		img.Tag = "latest"
	}

	// Mark skipped images
	if strings.Contains(img.Repository, "thinkportgmbh") {
		img.Skipped = true
	}

	return img
}

// isDockerfile checks if a filename is a Dockerfile
// Matches: Dockerfile, *.dockerfile, Dockerfile.*
func isDockerfile(filename string) bool {
	lower := strings.ToLower(filename)

	// Exact match: Dockerfile (case-insensitive)
	if lower == "dockerfile" {
		return true
	}

	// Pattern: *.dockerfile (e.g., app.dockerfile)
	if strings.HasSuffix(lower, ".dockerfile") {
		return true
	}

	// Pattern: Dockerfile.* (e.g., Dockerfile.prod, Dockerfile.dev)
	if strings.HasPrefix(lower, "dockerfile.") {
		return true
	}

	return false
}

// parseDockerfile extracts images from FROM instructions in a Dockerfile
func parseDockerfile(path string) ([]ImageInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var images []ImageInfo
	args := make(map[string]string)    // ARG name -> default value
	aliases := make(map[string]bool)   // Stage aliases (FROM ... AS name)

	// Regex patterns
	argPattern := regexp.MustCompile(`^\s*ARG\s+(\w+)(?:=(.*))?$`)
	fromPattern := regexp.MustCompile(`^\s*FROM\s+(\S+)(?:\s+AS\s+(\w+))?`)
	varPattern := regexp.MustCompile(`\$\{?(\w+)(?::-([^}]*))?\}?`)

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse ARG instructions
		if matches := argPattern.FindStringSubmatch(line); matches != nil {
			argName := matches[1]
			argValue := ""
			if len(matches) > 2 {
				argValue = strings.TrimSpace(matches[2])
				// Remove surrounding quotes if present
				argValue = strings.Trim(argValue, `"'`)
			}
			if argValue != "" {
				args[argName] = argValue
			}
			continue
		}

		// Parse FROM instructions
		if matches := fromPattern.FindStringSubmatch(line); matches != nil {
			imageRef := matches[1]

			// Track stage alias if present (FROM ... AS name)
			if len(matches) > 2 && matches[2] != "" {
				aliases[strings.ToLower(matches[2])] = true
			}

			// Resolve variables in the image reference
			resolved := resolveDockerfileVars(imageRef, args, varPattern)

			// Skip if unresolved (still contains $)
			if strings.Contains(resolved, "$") {
				continue
			}

			// Skip scratch
			if strings.ToLower(resolved) == "scratch" {
				continue
			}

			// Skip stage aliases
			if aliases[strings.ToLower(resolved)] {
				continue
			}

			// Parse and add the image
			img := parseImageString(resolved, path, lineNum)
			if img != nil {
				images = append(images, *img)
			}
		}
	}

	return images, scanner.Err()
}

// resolveDockerfileVars resolves variables in a Dockerfile image reference
// Supports: $VAR, ${VAR}, ${VAR:-default}
func resolveDockerfileVars(imageRef string, args map[string]string, varPattern *regexp.Regexp) string {
	return varPattern.ReplaceAllStringFunc(imageRef, func(match string) string {
		submatches := varPattern.FindStringSubmatch(match)
		if submatches == nil {
			return match
		}

		varName := submatches[1]
		defaultVal := ""
		if len(submatches) > 2 {
			defaultVal = submatches[2]
		}

		// Check if we have a value for this ARG
		if val, ok := args[varName]; ok && val != "" {
			return val
		}

		// Use default value if specified (${VAR:-default})
		if defaultVal != "" {
			return defaultVal
		}

		// Return original (unresolved)
		return match
	})
}
