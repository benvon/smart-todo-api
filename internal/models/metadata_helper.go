package models

// MergeTags merges AI tags with user tags, with user tags taking precedence
// Returns the merged tags and updated tag sources map
func (m *Metadata) MergeTags(aiTags []string, userTags []string) {
	// Initialize tag sources if nil
	if m.TagSources == nil {
		m.TagSources = make(map[string]TagSource)
	}
	
	// Start with all tags from AI
	for _, tag := range aiTags {
		// Only add if not already present as user tag
		if !contains(userTags, tag) {
			m.CategoryTags = appendIfNotExists(m.CategoryTags, tag)
			m.TagSources[tag] = TagSourceAI
		}
	}
	
	// Add all user tags (they override AI tags)
	for _, tag := range userTags {
		m.CategoryTags = appendIfNotExists(m.CategoryTags, tag)
		m.TagSources[tag] = TagSourceUser
	}
}

// SetUserTags sets tags as user-defined
func (m *Metadata) SetUserTags(tags []string) {
	if m.TagSources == nil {
		m.TagSources = make(map[string]TagSource)
	}
	
	m.CategoryTags = tags
	for _, tag := range tags {
		m.TagSources[tag] = TagSourceUser
	}
}

// RemoveTag removes a tag from the metadata
func (m *Metadata) RemoveTag(tag string) {
	// Remove from category tags
	newTags := make([]string, 0, len(m.CategoryTags))
	for _, t := range m.CategoryTags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	m.CategoryTags = newTags
	
	// Remove from tag sources
	if m.TagSources != nil {
		delete(m.TagSources, tag)
	}
}

// AddTag adds a tag with the specified source
func (m *Metadata) AddTag(tag string, source TagSource) {
	if m.TagSources == nil {
		m.TagSources = make(map[string]TagSource)
	}
	
	// Add to category tags if not already present
	m.CategoryTags = appendIfNotExists(m.CategoryTags, tag)
	m.TagSources[tag] = source
}

// GetUserTags returns only tags that are user-defined
func (m *Metadata) GetUserTags() []string {
	if m.TagSources == nil {
		return nil
	}
	
	userTags := make([]string, 0)
	for _, tag := range m.CategoryTags {
		if m.TagSources[tag] == TagSourceUser {
			userTags = append(userTags, tag)
		}
	}
	
	return userTags
}

// GetAITags returns only tags that are AI-generated
func (m *Metadata) GetAITags() []string {
	if m.TagSources == nil {
		return nil
	}
	
	aiTags := make([]string, 0)
	for _, tag := range m.CategoryTags {
		if m.TagSources[tag] == TagSourceAI {
			aiTags = append(aiTags, tag)
		}
	}
	
	return aiTags
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func appendIfNotExists(slice []string, item string) []string {
	if !contains(slice, item) {
		return append(slice, item)
	}
	return slice
}
