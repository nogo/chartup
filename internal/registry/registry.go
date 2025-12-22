package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

var ErrRateLimit = errors.New("rate limit exceeded")

// Client is a registry client for checking image tags
type Client struct {
	httpClient *http.Client
}

// New creates a new registry client
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TagInfo holds information about an image tag
type TagInfo struct {
	Name      string
	Latest    string
	AllTags   []string
	FromCache bool
}

// GetLatestTag fetches the latest tag for an image from the appropriate registry
func (c *Client) GetLatestTag(registry, repository, currentTag string) (*TagInfo, error) {
	switch {
	case registry == "docker.io" || registry == "":
		return c.getDockerHubTags(repository, currentTag)
	case strings.Contains(registry, "quay.io"):
		return c.getQuayTags(repository, currentTag)
	case strings.Contains(registry, "ghcr.io"):
		return c.getOCITags("ghcr.io", repository, currentTag)
	case strings.Contains(registry, "gcr.io"):
		return c.getOCITags("gcr.io", repository, currentTag)
	case strings.Contains(registry, "registry.k8s.io"):
		return c.getOCITags("registry.k8s.io", repository, currentTag)
	default:
		return nil, fmt.Errorf("unsupported registry: %s", registry)
	}
}

// Docker Hub API response structures
type dockerHubTagsResponse struct {
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
	Next string `json:"next"`
}

func (c *Client) getDockerHubTags(repository, currentTag string) (*TagInfo, error) {
	// Handle official images (e.g., "postgres" -> "library/postgres")
	if !strings.Contains(repository, "/") {
		repository = "library/" + repository
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags?page_size=100", repository)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Docker Hub API returned status %d", resp.StatusCode)
	}

	var tagsResp dockerHubTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(tagsResp.Results))
	for _, t := range tagsResp.Results {
		tags = append(tags, t.Name)
	}

	latest := findLatestTag(tags, currentTag)

	return &TagInfo{
		Name:    repository,
		Latest:  latest,
		AllTags: tags,
	}, nil
}

// Quay.io API response structures
type quayTagsResponse struct {
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

func (c *Client) getQuayTags(repository, currentTag string) (*TagInfo, error) {
	url := fmt.Sprintf("https://quay.io/api/v1/repository/%s/tag/?limit=100", repository)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Quay.io API returned status %d", resp.StatusCode)
	}

	var tagsResp quayTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(tagsResp.Tags))
	for _, t := range tagsResp.Tags {
		tags = append(tags, t.Name)
	}

	latest := findLatestTag(tags, currentTag)

	return &TagInfo{
		Name:    repository,
		Latest:  latest,
		AllTags: tags,
	}, nil
}

// OCI Registry API response structures (used by ghcr.io, gcr.io, registry.k8s.io)
type ociTokenResponse struct {
	Token string `json:"token"`
}

type ociTagsResponse struct {
	Tags []string `json:"tags"`
}

func (c *Client) getOCITags(registry, repository, currentTag string) (*TagInfo, error) {
	// Step 1: Get anonymous token
	token, err := c.getOCIToken(registry, repository)
	if err != nil {
		return nil, err
	}

	// Step 2: List tags using the token
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", registry, repository)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("%s requires authentication", registry)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s API returned status %d", registry, resp.StatusCode)
	}

	var tagsResp ociTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, err
	}

	latest := findLatestTag(tagsResp.Tags, currentTag)

	return &TagInfo{
		Name:    repository,
		Latest:  latest,
		AllTags: tagsResp.Tags,
	}, nil
}

func (c *Client) getOCIToken(registry, repository string) (string, error) {
	// Different registries have different token endpoints
	var tokenURL string

	switch registry {
	case "ghcr.io":
		tokenURL = fmt.Sprintf("https://ghcr.io/token?scope=repository:%s:pull", repository)
	case "gcr.io":
		tokenURL = fmt.Sprintf("https://gcr.io/v2/token?scope=repository:%s:pull", repository)
	case "registry.k8s.io":
		// registry.k8s.io may not require a token for public images, try without
		return "", nil
	default:
		return "", nil
	}

	req, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", ErrRateLimit
	}

	if resp.StatusCode != 200 {
		// Token endpoint failed, try without token
		return "", nil
	}

	var tokenResp ociTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", nil // Ignore decode errors, try without token
	}

	return tokenResp.Token, nil
}

// semverRegex matches semantic version patterns
var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

// findLatestTag finds the latest tag that matches the pattern of the current tag
func findLatestTag(tags []string, currentTag string) string {
	if len(tags) == 0 {
		return ""
	}

	// Determine the type of current tag
	currentMatch := semverRegex.FindStringSubmatch(currentTag)

	// If current tag is not semver-like, just return the newest semver tag
	if currentMatch == nil {
		// Filter to semver-like tags and return highest
		semverTags := filterSemverTags(tags)
		if len(semverTags) > 0 {
			sort.Sort(sort.Reverse(semverSlice(semverTags)))
			return semverTags[0]
		}
		return tags[0] // Return first tag as fallback
	}

	// Check if current tag has 'v' prefix
	hasVPrefix := strings.HasPrefix(currentTag, "v")

	// Filter tags that match the same pattern (v prefix or not)
	matchingTags := []string{}
	for _, tag := range tags {
		if semverRegex.MatchString(tag) {
			tagHasV := strings.HasPrefix(tag, "v")
			if tagHasV == hasVPrefix {
				matchingTags = append(matchingTags, tag)
			}
		}
	}

	if len(matchingTags) == 0 {
		return currentTag
	}

	// Sort by semver and return highest
	sort.Sort(sort.Reverse(semverSlice(matchingTags)))
	return matchingTags[0]
}

func filterSemverTags(tags []string) []string {
	result := []string{}
	for _, tag := range tags {
		if semverRegex.MatchString(tag) {
			// Skip tags with extra suffixes like -rc, -alpha, -beta unless simple
			if !strings.Contains(tag, "-") || isSimpleVersion(tag) {
				result = append(result, tag)
			}
		}
	}
	return result
}

func isSimpleVersion(tag string) bool {
	// Match patterns like "1.0.0", "v1.0.0", "1.0", "410"
	return semverRegex.MatchString(tag) && !strings.Contains(tag, "-")
}

// semverSlice implements sort.Interface for semver-like strings
type semverSlice []string

func (s semverSlice) Len() int      { return len(s) }
func (s semverSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s semverSlice) Less(i, j int) bool {
	return compareSemver(s[i], s[j]) < 0
}

func compareSemver(a, b string) int {
	matchA := semverRegex.FindStringSubmatch(a)
	matchB := semverRegex.FindStringSubmatch(b)

	if matchA == nil || matchB == nil {
		return strings.Compare(a, b)
	}

	for i := 1; i <= 3; i++ {
		var numA, numB int
		if i < len(matchA) && matchA[i] != "" {
			fmt.Sscanf(matchA[i], "%d", &numA)
		}
		if i < len(matchB) && matchB[i] != "" {
			fmt.Sscanf(matchB[i], "%d", &numB)
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}
