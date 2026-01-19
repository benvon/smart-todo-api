package models

// Metadata contains additional tags and information about a todo
type Metadata struct {
	CategoryTags []string `json:"category_tags,omitempty"`
	Priority     *string  `json:"priority,omitempty"`
	Context      []string `json:"context,omitempty"`
	Duration     *string  `json:"duration,omitempty"`
}
