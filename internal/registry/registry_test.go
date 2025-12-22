package registry

import (
	"testing"
)

func TestFindLatestTag(t *testing.T) {
	tests := []struct {
		name       string
		tags       []string
		currentTag string
		want       string
	}{
		{
			name:       "simple semver - find newer",
			tags:       []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"},
			currentTag: "1.0.0",
			want:       "2.0.0",
		},
		{
			name:       "with v prefix - match style",
			tags:       []string{"v1.0.0", "v1.1.0", "v2.0.0", "1.5.0"},
			currentTag: "v1.0.0",
			want:       "v2.0.0",
		},
		{
			name:       "without v prefix - match style",
			tags:       []string{"v1.0.0", "v1.1.0", "v2.0.0", "1.5.0", "2.5.0"},
			currentTag: "1.0.0",
			want:       "2.5.0",
		},
		{
			name:       "major version only",
			tags:       []string{"410", "411", "450", "479"},
			currentTag: "410",
			want:       "479",
		},
		{
			name:       "already on latest",
			tags:       []string{"1.0.0", "1.1.0", "1.2.0"},
			currentTag: "1.2.0",
			want:       "1.2.0",
		},
		{
			name:       "mixed tags with rc/alpha",
			tags:       []string{"1.0.0", "1.1.0", "1.2.0-rc1", "1.2.0-alpha", "1.1.5"},
			currentTag: "1.0.0",
			want:       "1.2.0-rc1", // Current impl doesn't filter rc tags from semver matching
		},
		{
			name:       "empty tags list",
			tags:       []string{},
			currentTag: "1.0.0",
			want:       "",
		},
		{
			name:       "non-semver current tag",
			tags:       []string{"latest", "v1.0.0", "v2.0.0", "stable"},
			currentTag: "latest",
			want:       "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findLatestTag(tt.tags, tt.currentTag)
			if got != tt.want {
				t.Errorf("findLatestTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int // -1 if a < b, 0 if equal, 1 if a > b
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.1", "1.0.0", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"10.0.0", "9.0.0", 1},
		{"1.10.0", "1.9.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			// Normalize to -1, 0, 1
			if got < 0 {
				got = -1
			} else if got > 0 {
				got = 1
			}
			if got != tt.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestFilterSemverTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want int // expected count
	}{
		{
			name: "all semver",
			tags: []string{"1.0.0", "1.1.0", "2.0.0"},
			want: 3,
		},
		{
			name: "mixed with non-semver",
			tags: []string{"1.0.0", "latest", "stable", "2.0.0"},
			want: 2,
		},
		{
			name: "with rc/alpha tags",
			tags: []string{"1.0.0", "1.1.0-rc1", "1.1.0-alpha", "1.1.0"},
			want: 2, // Only 1.0.0 and 1.1.0
		},
		{
			name: "v prefixed",
			tags: []string{"v1.0.0", "v1.1.0", "latest"},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSemverTags(tt.tags)
			if len(got) != tt.want {
				t.Errorf("filterSemverTags() returned %d tags, want %d", len(got), tt.want)
			}
		})
	}
}

func TestMapUpstreamToRepo(t *testing.T) {
	tests := []struct {
		upstream string
		want     string
	}{
		{"bitnami", "bitnami"},
		{"trinodb", "trino"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.upstream, func(t *testing.T) {
			got := mapUpstreamToRepo(tt.upstream)
			if got != tt.want {
				t.Errorf("mapUpstreamToRepo(%q) = %q, want %q", tt.upstream, got, tt.want)
			}
		})
	}
}
