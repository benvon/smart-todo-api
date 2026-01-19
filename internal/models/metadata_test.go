package models

import (
	"encoding/json"
	"testing"
)

func TestMetadata_JSONSerialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata Metadata
		jsonStr  string
	}{
		{
			name: "empty metadata",
			metadata: Metadata{},
			jsonStr: `{}`,
		},
		{
			name: "with category tags",
			metadata: Metadata{
				CategoryTags: []string{"work", "urgent"},
			},
			jsonStr: `{"category_tags":["work","urgent"]}`,
		},
		{
			name: "with priority",
			metadata: Metadata{
				Priority: stringPtr("high"),
			},
			jsonStr: `{"priority":"high"}`,
		},
		{
			name: "with context",
			metadata: Metadata{
				Context: []string{"home", "evening"},
			},
			jsonStr: `{"context":["home","evening"]}`,
		},
		{
			name: "with duration",
			metadata: Metadata{
				Duration: stringPtr("30m"),
			},
			jsonStr: `{"duration":"30m"}`,
		},
		{
			name: "all fields",
			metadata: Metadata{
				CategoryTags: []string{"work"},
				Priority:     stringPtr("medium"),
				Context:      []string{"office"},
				Duration:     stringPtr("1h"),
			},
			jsonStr: `{"category_tags":["work"],"priority":"medium","context":["office"],"duration":"1h"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test marshaling
			data, err := json.Marshal(tt.metadata)
			if err != nil {
				t.Fatalf("Failed to marshal metadata: %v", err)
			}

			// Parse both JSON strings to compare
			var got map[string]any
			var want map[string]any

			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal got: %v", err)
			}

			if err := json.Unmarshal([]byte(tt.jsonStr), &want); err != nil {
				t.Fatalf("Failed to unmarshal want: %v", err)
			}

			// Compare maps
			if !mapsEqual(got, want) {
				t.Errorf("JSON mismatch: got %s, want %s", string(data), tt.jsonStr)
			}

			// Test unmarshaling
			var unmarshaled Metadata
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal back: %v", err)
			}

			if !metadataEqual(tt.metadata, unmarshaled) {
				t.Errorf("Metadata mismatch after round-trip")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !valuesEqual(v, bv) {
			return false
		}
	}
	return true
}

func valuesEqual(a, b any) bool {
	// Simple comparison for test purposes
	switch av := a.(type) {
	case []any:
		if bv, ok := b.([]any); ok {
			if len(av) != len(bv) {
				return false
			}
			for i := range av {
				if !valuesEqual(av[i], bv[i]) {
					return false
				}
			}
			return true
		}
		return false
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
		return false
	default:
		return a == b
	}
}

func metadataEqual(a, b Metadata) bool {
	if len(a.CategoryTags) != len(b.CategoryTags) {
		return false
	}
	for i := range a.CategoryTags {
		if a.CategoryTags[i] != b.CategoryTags[i] {
			return false
		}
	}

	if (a.Priority == nil) != (b.Priority == nil) {
		return false
	}
	if a.Priority != nil && b.Priority != nil && *a.Priority != *b.Priority {
		return false
	}

	if len(a.Context) != len(b.Context) {
		return false
	}
	for i := range a.Context {
		if a.Context[i] != b.Context[i] {
			return false
		}
	}

	if (a.Duration == nil) != (b.Duration == nil) {
		return false
	}
	if a.Duration != nil && b.Duration != nil && *a.Duration != *b.Duration {
		return false
	}

	return true
}
