package database

import (
	"testing"
)

func TestAllowedOriginsSlice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"single", "https://a.example.com", []string{"https://a.example.com"}},
		{"comma", "https://a.com, https://b.com", []string{"https://a.com", "https://b.com"}},
		{"dedup", "x, x, y", []string{"x", "y"}},
		{"trim", "  a  ,  b  ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AllowedOriginsSlice(tt.raw)
			if len(got) != len(tt.want) {
				t.Errorf("AllowedOriginsSlice(%q) length = %d, want %d", tt.raw, len(got), len(tt.want))
				return
			}
			seen := make(map[string]bool)
			for _, s := range got {
				seen[s] = true
			}
			for _, w := range tt.want {
				if !seen[w] {
					t.Errorf("AllowedOriginsSlice(%q) missing %q", tt.raw, w)
				}
			}
		})
	}
}
