package models

// TagSource represents the source of a tag (user or AI)
type TagSource string

const (
	TagSourceUser TagSource = "user"
	TagSourceAI   TagSource = "ai"
)

// Metadata contains additional tags and information about a todo
type Metadata struct {
	CategoryTags []string           `json:"category_tags,omitempty"`
	TagSources   map[string]TagSource `json:"tag_sources,omitempty"` // Maps tag name to its source
	Priority     *string            `json:"priority,omitempty"`
	Context      []string           `json:"context,omitempty"`
	Duration     *string            `json:"duration,omitempty"`
}
