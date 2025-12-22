package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_ImageOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chartup-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheFile := filepath.Join(tmpDir, "test-cache.json")
	c := New(cacheFile, 1*time.Hour, false)

	// Test SetImage and GetImage
	c.SetImage("docker.io/nginx", "1.21.0", []string{"1.20.0", "1.21.0", "latest"})

	latest, tags, ok := c.GetImage("docker.io/nginx")
	if !ok {
		t.Error("expected to find cached image")
	}
	if latest != "1.21.0" {
		t.Errorf("Latest = %q, want %q", latest, "1.21.0")
	}
	if len(tags) != 3 {
		t.Errorf("got %d tags, want 3", len(tags))
	}

	// Test non-existent key
	_, _, ok = c.GetImage("docker.io/nonexistent")
	if ok {
		t.Error("expected not to find non-existent image")
	}
}

func TestCache_ChartOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chartup-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheFile := filepath.Join(tmpDir, "test-cache.json")
	c := New(cacheFile, 1*time.Hour, false)

	// Test SetChart and GetChart
	c.SetChart("bitnami/postgresql", "14.0.0")

	latest, ok := c.GetChart("bitnami/postgresql")
	if !ok {
		t.Error("expected to find cached chart")
	}
	if latest != "14.0.0" {
		t.Errorf("Latest = %q, want %q", latest, "14.0.0")
	}

	// Test non-existent key
	_, ok = c.GetChart("bitnami/nonexistent")
	if ok {
		t.Error("expected not to find non-existent chart")
	}
}

func TestCache_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chartup-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheFile := filepath.Join(tmpDir, "test-cache.json")

	// Create and save cache
	c1 := New(cacheFile, 1*time.Hour, false)
	c1.SetImage("docker.io/nginx", "1.21.0", nil)
	c1.SetChart("bitnami/postgresql", "14.0.0")
	if err := c1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load in new cache instance
	c2 := New(cacheFile, 1*time.Hour, false)
	if err := c2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify data persisted
	latest, _, ok := c2.GetImage("docker.io/nginx")
	if !ok {
		t.Error("expected to find persisted image")
	}
	if latest != "1.21.0" {
		t.Errorf("Image Latest = %q, want %q", latest, "1.21.0")
	}

	chartLatest, ok := c2.GetChart("bitnami/postgresql")
	if !ok {
		t.Error("expected to find persisted chart")
	}
	if chartLatest != "14.0.0" {
		t.Errorf("Chart Latest = %q, want %q", chartLatest, "14.0.0")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chartup-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheFile := filepath.Join(tmpDir, "test-cache.json")

	// Create cache with very short TTL
	c := New(cacheFile, 1*time.Millisecond, false)
	c.SetImage("docker.io/nginx", "1.21.0", nil)

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Should not find expired entry
	_, _, ok := c.GetImage("docker.io/nginx")
	if ok {
		t.Error("expected expired entry to not be found")
	}
}

func TestCache_Disabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chartup-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheFile := filepath.Join(tmpDir, "test-cache.json")

	// Create disabled cache
	c := New(cacheFile, 1*time.Hour, true)
	c.SetImage("docker.io/nginx", "1.21.0", nil)

	// Should not find anything when disabled
	_, _, ok := c.GetImage("docker.io/nginx")
	if ok {
		t.Error("expected disabled cache to not return values")
	}

	// Save should be a no-op
	if err := c.Save(); err != nil {
		t.Errorf("Save() on disabled cache error = %v", err)
	}

	// Cache file should not exist
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("expected cache file to not exist when disabled")
	}
}

func TestCache_LoadNonExistent(t *testing.T) {
	c := New("/nonexistent/path/cache.json", 1*time.Hour, false)

	// Should not error on non-existent file
	if err := c.Load(); err != nil {
		t.Errorf("Load() on non-existent file error = %v", err)
	}
}
