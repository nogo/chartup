package checker

import (
	"errors"
	"fmt"

	"github.com/nogo/chartup/internal/cache"
	"github.com/nogo/chartup/internal/registry"
	"github.com/nogo/chartup/internal/scanner"
)

// Checker performs version checks for images and charts
type Checker struct {
	cache    *cache.Cache
	registry *registry.Client
}

// ImageResult holds the result of an image version check
type ImageResult struct {
	Repository string
	Registry   string
	Current    string
	Latest     string
	Status     Status
	Skipped    bool
	Error      string
	Path       string // File where this image was found
	Line       int    // Line number in file (0 if unknown)
}

// ChartResult holds the result of a chart version check
type ChartResult struct {
	Name     string
	Current  string
	Latest   string
	Upstream string
	Status   Status
	Error    string
	Path     string // File where this chart was found
	Line     int    // Line number in file (0 if unknown)
}

// Status represents the update status
type Status int

const (
	StatusUnknown Status = iota
	StatusUpToDate
	StatusUpdateAvailable
	StatusSkipped
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusUpToDate:
		return "OK"
	case StatusUpdateAvailable:
		return "UPDATE"
	case StatusSkipped:
		return "SKIPPED"
	case StatusError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Results holds all check results
type Results struct {
	Images []ImageResult
	Charts []ChartResult
}

// New creates a new Checker
func New(c *cache.Cache) *Checker {
	return &Checker{
		cache:    c,
		registry: registry.New(),
	}
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	return errors.Is(err, registry.ErrRateLimit)
}

// CheckAll checks all images and charts for updates
func (c *Checker) CheckAll(scan *scanner.ScanResults) (*Results, error) {
	results := &Results{
		Images: make([]ImageResult, 0, len(scan.Images)),
		Charts: make([]ChartResult, 0, len(scan.Charts)),
	}

	var rateLimitHit bool

	// Check images
	for _, img := range scan.Images {
		if rateLimitHit {
			results.Images = append(results.Images, ImageResult{
				Repository: img.Repository,
				Registry:   img.Registry,
				Current:    img.Tag,
				Status:     StatusError,
				Error:      "rate limit hit",
				Path:       img.Path,
				Line:       img.Line,
			})
			continue
		}

		result := c.checkImage(img)
		results.Images = append(results.Images, result)

		if result.Error == "rate limit exceeded" {
			rateLimitHit = true
		}
	}

	// Check charts
	for _, chart := range scan.Charts {
		if rateLimitHit {
			results.Charts = append(results.Charts, ChartResult{
				Name:     chart.Name,
				Current:  chart.Version,
				Upstream: chart.Upstream,
				Status:   StatusError,
				Error:    "rate limit hit",
				Path:     chart.Path,
				Line:     chart.Line,
			})
			continue
		}

		result := c.checkChart(chart)
		results.Charts = append(results.Charts, result)

		if result.Error == "rate limit exceeded" {
			rateLimitHit = true
		}
	}

	if rateLimitHit {
		return results, registry.ErrRateLimit
	}

	return results, nil
}

func (c *Checker) checkImage(img scanner.ImageInfo) ImageResult {
	result := ImageResult{
		Repository: img.Repository,
		Registry:   img.Registry,
		Current:    img.Tag,
		Path:       img.Path,
		Line:       img.Line,
	}

	if img.Skipped {
		result.Status = StatusSkipped
		result.Skipped = true
		return result
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s", img.Registry, img.Repository)
	if latest, _, ok := c.cache.GetImage(cacheKey); ok {
		result.Latest = latest
		result.Status = determineStatus(img.Tag, latest)
		return result
	}

	// Fetch from registry
	tagInfo, err := c.registry.GetLatestTag(img.Registry, img.Repository, img.Tag)
	if err != nil {
		if errors.Is(err, registry.ErrRateLimit) {
			result.Status = StatusError
			result.Error = "rate limit exceeded"
		} else {
			result.Status = StatusError
			result.Error = err.Error()
		}
		return result
	}

	// Update cache
	c.cache.SetImage(cacheKey, tagInfo.Latest, tagInfo.AllTags)

	result.Latest = tagInfo.Latest
	result.Status = determineStatus(img.Tag, tagInfo.Latest)
	return result
}

func (c *Checker) checkChart(chart scanner.ChartInfo) ChartResult {
	result := ChartResult{
		Name:     chart.Name,
		Current:  chart.Version,
		Upstream: chart.Upstream,
		Path:     chart.Path,
		Line:     chart.Line,
	}

	// Skip charts without known upstreams
	if chart.Upstream == "" {
		result.Status = StatusSkipped
		return result
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s", chart.Upstream, chart.Name)
	if latest, ok := c.cache.GetChart(cacheKey); ok {
		result.Latest = latest
		result.Status = determineStatus(chart.Version, latest)
		return result
	}

	// Fetch from ArtifactHub
	versionInfo, err := c.registry.GetChartVersion(chart.Name, chart.Upstream)
	if err != nil {
		if errors.Is(err, registry.ErrRateLimit) {
			result.Status = StatusError
			result.Error = "rate limit exceeded"
		} else {
			result.Status = StatusError
			result.Error = err.Error()
		}
		return result
	}

	// Update cache
	c.cache.SetChart(cacheKey, versionInfo.LatestVersion)

	result.Latest = versionInfo.LatestVersion
	result.Status = determineStatus(chart.Version, versionInfo.LatestVersion)
	return result
}

func determineStatus(current, latest string) Status {
	if current == latest {
		return StatusUpToDate
	}
	if latest == "" {
		return StatusUnknown
	}
	return StatusUpdateAvailable
}
