package cache

import (
	"encoding/json"
	"os"
	"time"
)

// Cache handles JSON-based caching for version lookups
type Cache struct {
	filename  string
	ttl       time.Duration
	skipReads bool // When true, ignore cached data but still write fresh results
	data      CacheData
}

// CacheData represents the cache file structure
type CacheData struct {
	Images map[string]CacheEntry `json:"images"`
	Charts map[string]CacheEntry `json:"charts"`
}

// CacheEntry represents a single cached lookup
type CacheEntry struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
	AllTags   []string  `json:"all_tags,omitempty"`
}

// New creates a new cache instance
// When skipReads is true, cached data is ignored but fresh results are still saved
func New(filename string, ttl time.Duration, skipReads bool) *Cache {
	return &Cache{
		filename:  filename,
		ttl:       ttl,
		skipReads: skipReads,
		data: CacheData{
			Images: make(map[string]CacheEntry),
			Charts: make(map[string]CacheEntry),
		},
	}
}

// Load reads the cache from disk
func (c *Cache) Load() error {
	data, err := os.ReadFile(c.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet
		}
		return err
	}

	return json.Unmarshal(data, &c.data)
}

// Save writes the cache to disk
func (c *Cache) Save() error {
	data, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.filename, data, 0644)
}

// GetImage retrieves a cached image lookup
// Returns false if skipReads is enabled (forces fresh lookup)
func (c *Cache) GetImage(key string) (string, []string, bool) {
	if c.skipReads {
		return "", nil, false
	}

	entry, ok := c.data.Images[key]
	if !ok {
		return "", nil, false
	}

	if time.Since(entry.CheckedAt) > c.ttl {
		return "", nil, false // Cache expired
	}

	return entry.Latest, entry.AllTags, true
}

// SetImage stores an image lookup in the cache
func (c *Cache) SetImage(key, latest string, allTags []string) {
	c.data.Images[key] = CacheEntry{
		Latest:    latest,
		CheckedAt: time.Now(),
		AllTags:   allTags,
	}
}

// GetChart retrieves a cached chart lookup
// Returns false if skipReads is enabled (forces fresh lookup)
func (c *Cache) GetChart(key string) (string, bool) {
	if c.skipReads {
		return "", false
	}

	entry, ok := c.data.Charts[key]
	if !ok {
		return "", false
	}

	if time.Since(entry.CheckedAt) > c.ttl {
		return "", false // Cache expired
	}

	return entry.Latest, true
}

// SetChart stores a chart lookup in the cache
func (c *Cache) SetChart(key, latest string) {
	c.data.Charts[key] = CacheEntry{
		Latest:    latest,
		CheckedAt: time.Now(),
	}
}
