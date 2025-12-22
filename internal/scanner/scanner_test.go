package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseImageString(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRepo   string
		wantTag    string
		wantReg    string
		wantSkip   bool
		wantNil    bool
	}{
		{
			name:     "simple docker hub image",
			input:    "nginx:1.21",
			wantRepo: "nginx",
			wantTag:  "1.21",
			wantReg:  "docker.io",
		},
		{
			name:     "docker hub with org",
			input:    "bitnami/postgresql:11.14.0",
			wantRepo: "bitnami/postgresql",
			wantTag:  "11.14.0",
			wantReg:  "docker.io",
		},
		{
			name:     "quay.io image",
			input:    "quay.io/minio/minio:latest",
			wantRepo: "minio/minio",
			wantTag:  "latest",
			wantReg:  "quay.io",
		},
		{
			name:     "ghcr.io image",
			input:    "ghcr.io/owner/repo:v1.0.0",
			wantRepo: "owner/repo",
			wantTag:  "v1.0.0",
			wantReg:  "ghcr.io",
		},
		{
			name:     "registry.k8s.io image",
			input:    "registry.k8s.io/ingress-nginx/controller:v1.0.0",
			wantRepo: "ingress-nginx/controller",
			wantTag:  "v1.0.0",
			wantReg:  "registry.k8s.io",
		},
		{
			name:    "bare image name rejected",
			input:   "nginx",
			wantNil: true, // Bare names without / or : are rejected
		},
		{
			name:     "skipped thinkportgmbh image",
			input:    "thinkportgmbh/workshops:jupyter",
			wantRepo: "thinkportgmbh/workshops",
			wantTag:  "jupyter",
			wantReg:  "docker.io",
			wantSkip: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
		{
			name:    "just latest",
			input:   "latest",
			wantNil: true,
		},
		{
			name:    "path-like string",
			input:   "/var/log/app",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseImageString(tt.input, "/test/path")

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", result.Repository, tt.wantRepo)
			}
			if result.Tag != tt.wantTag {
				t.Errorf("Tag = %q, want %q", result.Tag, tt.wantTag)
			}
			if result.Registry != tt.wantReg {
				t.Errorf("Registry = %q, want %q", result.Registry, tt.wantReg)
			}
			if result.Skipped != tt.wantSkip {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkip)
			}
		})
	}
}

func TestDetectUpstream(t *testing.T) {
	tests := []struct {
		name     string
		chart    string
		path     string
		expected string
	}{
		{
			name:     "trino chart",
			chart:    "trino",
			path:     "/some/path/trino/Chart.yaml",
			expected: "trinodb",
		},
		{
			name:     "postgresql in bitnami path",
			chart:    "postgresql",
			path:     "/charts/bitnami/postgresql/Chart.yaml",
			expected: "bitnami",
		},
		{
			name:     "common in charts path",
			chart:    "common",
			path:     "/hive/charts/common/Chart.yaml",
			expected: "bitnami",
		},
		{
			name:     "custom chart",
			chart:    "my-app",
			path:     "/my-app/Chart.yaml",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectUpstream(tt.chart, tt.path)
			if result != tt.expected {
				t.Errorf("detectUpstream(%q, %q) = %q, want %q", tt.chart, tt.path, result, tt.expected)
			}
		})
	}
}

func TestScan(t *testing.T) {
	// Create temp directory with test files
	tmpDir, err := os.MkdirTemp("", "chartup-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a Chart.yaml
	chartDir := filepath.Join(tmpDir, "test-chart")
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		t.Fatal(err)
	}

	chartYAML := `name: test-chart
version: 1.0.0
appVersion: "1.0"
`
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a values.yaml with images
	valuesYAML := `image:
  repository: nginx
  tag: "1.21"

sidecar:
  image: busybox:1.35
`
	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(valuesYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Run scan
	results, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Verify charts found
	if len(results.Charts) != 1 {
		t.Errorf("expected 1 chart, got %d", len(results.Charts))
	} else {
		if results.Charts[0].Name != "test-chart" {
			t.Errorf("Chart.Name = %q, want %q", results.Charts[0].Name, "test-chart")
		}
		if results.Charts[0].Version != "1.0.0" {
			t.Errorf("Chart.Version = %q, want %q", results.Charts[0].Version, "1.0.0")
		}
	}

	// Verify images found
	if len(results.Images) < 1 {
		t.Errorf("expected at least 1 image, got %d", len(results.Images))
	}

	// Check for nginx image
	foundNginx := false
	for _, img := range results.Images {
		if img.Repository == "nginx" && img.Tag == "1.21" {
			foundNginx = true
			break
		}
	}
	if !foundNginx {
		t.Error("nginx:1.21 image not found in results")
	}
}
