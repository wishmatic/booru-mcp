package booru

import (
	"strings"
)

type TagCategory string

const (
	CategoryGeneral   TagCategory = "general"
	CategoryArtist    TagCategory = "artist"
	CategoryCopyright TagCategory = "copyright"
	CategoryCharacter TagCategory = "character"
	CategoryMeta      TagCategory = "meta"
)

// danbooruCategories is Danbooru's numeric encoding: general 0, artist 1, copyright 3, character 4, meta 5. The value 2
// is skipped.
var danbooruCategories = map[TagCategory]int{
	CategoryGeneral:   0,
	CategoryArtist:    1,
	CategoryCopyright: 3,
	CategoryCharacter: 4,
	CategoryMeta:      5,
}

var categoriesByCode = map[int]TagCategory{
	0: CategoryGeneral,
	1: CategoryArtist,
	3: CategoryCopyright,
	4: CategoryCharacter,
	5: CategoryMeta,
}

var categoryNames = []TagCategory{CategoryGeneral, CategoryArtist, CategoryCopyright, CategoryCharacter, CategoryMeta}

func (c TagCategory) String() string {
	return string(c)
}

func (c TagCategory) Code() int {
	return danbooruCategories[c]
}

func ParseTagCategory(value string) (TagCategory, bool) {
	category := TagCategory(strings.ToLower(strings.TrimSpace(value)))
	if _, ok := danbooruCategories[category]; !ok {
		return "", false
	}

	return category, true
}

func CategoryNames() []TagCategory {
	out := make([]TagCategory, len(categoryNames))
	copy(out, categoryNames)

	return out
}

func CategoryList() string {
	names := make([]string, 0, len(categoryNames))

	for _, category := range categoryNames {
		names = append(names, category.String())
	}

	return strings.Join(names, ", ")
}

type Tag struct {
	Name         string
	Category     TagCategory
	Count        int
	AliasOf      string
	Implications []string
}

func NormalizeTag(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "_")
}
