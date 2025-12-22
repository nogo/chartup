package scanner

import (
	"fmt"
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
	Upstream   string // Known upstream source (e.g., "bitnami", "trinodb")
}

// ImageInfo holds information about a Docker image
type ImageInfo struct {
	Registry   string // e.g., "docker.io", "quay.io"
	Repository string // e.g., "trinodb/trino"
	Tag        string // e.g., "410"
	FullImage  string // Original full image string
	Path       string // File where it was found
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

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, err
	}

	images := []ImageInfo{}

	// Extract images recursively from the YAML structure
	extractImages(values, path, &images)

	// Also try regex extraction for image strings
	regexImages := extractImagesRegex(string(data), path)
	for _, img := range regexImages {
		found := false
		for _, existing := range images {
			if existing.FullImage == img.FullImage {
				found = true
				break
			}
		}
		if !found {
			images = append(images, img)
		}
	}

	return images, nil
}

func extractImages(data interface{}, path string, images *[]ImageInfo) {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check for common image patterns
		if repo, ok := v["repository"].(string); ok {
			tag := "latest"
			if t, ok := v["tag"].(string); ok {
				tag = t
			} else if t, ok := v["tag"].(int); ok {
				tag = fmt.Sprintf("%d", t)
			}
			img := parseImageString(repo+":"+tag, path)
			if img != nil {
				*images = append(*images, *img)
			}
		}

		// Check for "image" key with string value
		if imgStr, ok := v["image"].(string); ok {
			img := parseImageString(imgStr, path)
			if img != nil {
				*images = append(*images, *img)
			}
		}

		// Recurse into nested maps
		for _, val := range v {
			extractImages(val, path, images)
		}

	case []interface{}:
		for _, item := range v {
			extractImages(item, path, images)
		}
	}
}

var imageRegex = regexp.MustCompile(`(?:image:\s*["']?|repository:\s*["']?)([a-zA-Z0-9._/-]+(?::[a-zA-Z0-9._-]+)?)["']?`)

func extractImagesRegex(content, path string) []ImageInfo {
	images := []ImageInfo{}
	matches := imageRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			img := parseImageString(match[1], path)
			if img != nil {
				images = append(images, *img)
			}
		}
	}
	return images
}

func parseImageString(imageStr, path string) *ImageInfo {
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
