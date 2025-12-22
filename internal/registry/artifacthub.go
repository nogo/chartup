package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ArtifactHub API response structures
type artifactHubPackage struct {
	Version    string `json:"version"`
	AppVersion string `json:"app_version"`
	Name       string `json:"name"`
}

type artifactHubSearchResponse struct {
	Packages []struct {
		Version    string `json:"version"`
		Name       string `json:"name"`
		Repository struct {
			Name string `json:"name"`
		} `json:"repository"`
	} `json:"packages"`
}

// ChartVersionInfo holds information about a Helm chart version
type ChartVersionInfo struct {
	Name          string
	LatestVersion string
	AppVersion    string
	FromCache     bool
}

// GetChartVersion fetches the latest version of a Helm chart from ArtifactHub
func (c *Client) GetChartVersion(chartName, upstream string) (*ChartVersionInfo, error) {
	if upstream == "" {
		return nil, fmt.Errorf("no upstream configured for chart %s", chartName)
	}

	// Map upstream to ArtifactHub repo names
	repoName := mapUpstreamToRepo(upstream)

	// Try direct package lookup first
	url := fmt.Sprintf("https://artifacthub.io/api/v1/packages/helm/%s/%s", repoName, chartName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	if resp.StatusCode == 200 {
		var pkg artifactHubPackage
		if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
			return nil, err
		}

		return &ChartVersionInfo{
			Name:          chartName,
			LatestVersion: pkg.Version,
			AppVersion:    pkg.AppVersion,
		}, nil
	}

	// If direct lookup fails, try search
	return c.searchChart(chartName, upstream)
}

func (c *Client) searchChart(chartName, upstream string) (*ChartVersionInfo, error) {
	repoName := mapUpstreamToRepo(upstream)
	url := fmt.Sprintf("https://artifacthub.io/api/v1/packages/search?ts_query_web=%s&kind=0&limit=10", chartName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ArtifactHub API returned status %d", resp.StatusCode)
	}

	var searchResp artifactHubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	// Find matching package from the correct repo
	for _, pkg := range searchResp.Packages {
		if pkg.Name == chartName && pkg.Repository.Name == repoName {
			return &ChartVersionInfo{
				Name:          chartName,
				LatestVersion: pkg.Version,
			}, nil
		}
	}

	// Try any matching package name
	for _, pkg := range searchResp.Packages {
		if pkg.Name == chartName {
			return &ChartVersionInfo{
				Name:          chartName,
				LatestVersion: pkg.Version,
			}, nil
		}
	}

	return nil, fmt.Errorf("chart %s not found on ArtifactHub", chartName)
}

func mapUpstreamToRepo(upstream string) string {
	switch upstream {
	case "bitnami":
		return "bitnami"
	case "trinodb":
		return "trino"
	default:
		return upstream
	}
}
