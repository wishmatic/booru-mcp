package booru

func tagCategory(code int) TagCategory {
	if category, ok := categoriesByCode[code]; ok {
		return category
	}

	return CategoryGeneral
}

type tagJSON struct {
	Name      string `json:"name"`
	Category  int    `json:"category"`
	PostCount int    `json:"post_count"`
}

func toTags(raw []tagJSON) []Tag {
	tags := make([]Tag, 0, len(raw))

	for _, item := range raw {
		name := NormalizeTag(item.Name)
		if name == "" {
			continue
		}

		tags = append(tags, Tag{Name: name, Category: tagCategory(item.Category), Count: item.PostCount})
	}

	return tags
}

// relationJSON covers both tag_aliases.json and tag_implications.json, which share these two fields.
type relationJSON struct {
	AntecedentName string `json:"antecedent_name"`
	ConsequentName string `json:"consequent_name"`
}
