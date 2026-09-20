package booru

import "strings"

type TagCategory string

const (
	CategoryGeneral   TagCategory = "general"
	CategoryArtist    TagCategory = "artist"
	CategoryCopyright TagCategory = "copyright"
	CategoryCharacter TagCategory = "character"
	CategoryMeta      TagCategory = "meta"
)

func (c TagCategory) String() string {
	return string(c)
}

type Tag struct {
	Name     string
	Category TagCategory
	Count    int
}

func NormalizeTag(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "_")
}
