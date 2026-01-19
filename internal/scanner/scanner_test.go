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
			result := parseImageString(tt.input, "/test/path", 42)

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

func TestIsDockerfile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		// Exact matches
		{"Dockerfile", true},
		{"dockerfile", true},
		{"DOCKERFILE", true},

		// Pattern: *.dockerfile
		{"app.dockerfile", true},
		{"build.Dockerfile", true},
		{"my-service.DOCKERFILE", true},

		// Pattern: Dockerfile.*
		{"Dockerfile.prod", true},
		{"Dockerfile.dev", true},
		{"dockerfile.test", true},

		// Non-matches
		{"docker-compose.yml", false},
		{"Dockerignore", false},
		{"README.md", false},
		{"values.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := isDockerfile(tt.filename)
			if got != tt.want {
				t.Errorf("isDockerfile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestParseDockerfile(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantImages []struct {
			repo string
			tag  string
			line int
		}
	}{
		{
			name:    "simple FROM",
			content: "FROM nginx:1.25\n",
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"nginx", "1.25", 1},
			},
		},
		{
			name: "multi-stage build",
			content: `FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o main .

FROM alpine:3.19
COPY --from=builder /app/main /main
CMD ["/main"]
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"golang", "1.21", 1},
				{"alpine", "3.19", 6},
			},
		},
		{
			name: "skip alias reference",
			content: `FROM golang:1.21 AS builder
FROM builder AS test
FROM alpine:3.19
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"golang", "1.21", 1},
				{"alpine", "3.19", 3},
			},
		},
		{
			name: "skip scratch",
			content: `FROM golang:1.21 AS builder
FROM scratch
COPY --from=builder /app/main /main
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"golang", "1.21", 1},
			},
		},
		{
			name: "ARG with default value",
			content: `ARG BASE_IMAGE=nginx:1.25
FROM $BASE_IMAGE
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"nginx", "1.25", 2},
			},
		},
		{
			name: "ARG with braces",
			content: `ARG VERSION=1.25
FROM nginx:${VERSION}
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"nginx", "1.25", 2},
			},
		},
		{
			name: "ARG with default fallback syntax",
			content: `FROM ${BASE:-alpine:3.19}
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"alpine", "3.19", 1},
			},
		},
		{
			name: "skip ARG without default",
			content: `ARG BASE_IMAGE
FROM $BASE_IMAGE
FROM alpine:3.19
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"alpine", "3.19", 3},
			},
		},
		{
			name: "with comments and empty lines",
			content: `# Build stage
FROM golang:1.21 AS builder

# Runtime stage
FROM alpine:3.19
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"golang", "1.21", 2},
				{"alpine", "3.19", 5},
			},
		},
		{
			name: "full registry URLs",
			content: `FROM gcr.io/distroless/static:nonroot
FROM ghcr.io/owner/repo:v1.0.0
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"distroless/static", "nonroot", 1},
				{"owner/repo", "v1.0.0", 2},
			},
		},
		{
			name: "quoted ARG value",
			content: `ARG BASE="nginx:1.25"
FROM $BASE
`,
			wantImages: []struct {
				repo string
				tag  string
				line int
			}{
				{"nginx", "1.25", 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "Dockerfile-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}
			tmpFile.Close()

			images, err := parseDockerfile(tmpFile.Name())
			if err != nil {
				t.Fatalf("parseDockerfile() error = %v", err)
			}

			if len(images) != len(tt.wantImages) {
				t.Errorf("got %d images, want %d", len(images), len(tt.wantImages))
				for i, img := range images {
					t.Logf("  [%d] %s:%s (line %d)", i, img.Repository, img.Tag, img.Line)
				}
				return
			}

			for i, want := range tt.wantImages {
				got := images[i]
				if got.Repository != want.repo {
					t.Errorf("image[%d].Repository = %q, want %q", i, got.Repository, want.repo)
				}
				if got.Tag != want.tag {
					t.Errorf("image[%d].Tag = %q, want %q", i, got.Tag, want.tag)
				}
				if got.Line != want.line {
					t.Errorf("image[%d].Line = %d, want %d", i, got.Line, want.line)
				}
			}
		})
	}
}

func TestScanWithDockerfile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "chartup-dockerfile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test Dockerfiles
	dockerfile1 := `FROM golang:1.21 AS builder
FROM alpine:3.19
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile1), 0644); err != nil {
		t.Fatal(err)
	}

	dockerfile2 := `FROM nginx:1.25
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile.prod"), []byte(dockerfile2), 0644); err != nil {
		t.Fatal(err)
	}

	dockerfile3 := `FROM python:3.12
`
	if err := os.WriteFile(filepath.Join(tmpDir, "app.dockerfile"), []byte(dockerfile3), 0644); err != nil {
		t.Fatal(err)
	}

	// Run scan
	results, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should find 4 unique images
	expectedImages := map[string]bool{
		"golang:1.21":  false,
		"alpine:3.19":  false,
		"nginx:1.25":   false,
		"python:3.12":  false,
	}

	for _, img := range results.Images {
		key := img.Repository + ":" + img.Tag
		if _, exists := expectedImages[key]; exists {
			expectedImages[key] = true
		}
	}

	for img, found := range expectedImages {
		if !found {
			t.Errorf("expected image %s not found", img)
		}
	}
}
