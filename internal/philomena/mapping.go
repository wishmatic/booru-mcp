package philomena

import (
	"strings"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

// Philomena posts carry no rating field; the API filters by named filters instead, so every post is reported as
// explicit and a Deployment that caps below explicit excludes these clients entirely.
func postTags(names []string) []booru.Tag {
	tags := make([]booru.Tag, 0, len(names))

	for _, name := range names {
		category, bare := tagNamespace(name)
		if normalized := booru.NormalizeTag(bare); normalized != "" {
			tags = append(tags, booru.Tag{Name: normalized, Category: category})
		}
	}

	return tags
}

func toTags(raw []tagJSON) []booru.Tag {
	tags := make([]booru.Tag, 0, len(raw))

	for _, item := range raw {
		category, bare := tagNamespace(item.Name)
		if normalized := booru.NormalizeTag(bare); normalized != "" {
			tags = append(tags, booru.Tag{Name: normalized, Category: category, Count: item.total()})
		}
	}

	return tags
}

func tagNamespace(value string) (booru.TagCategory, string) {
	namespace, bare, found := strings.Cut(value, ":")
	if !found {
		return booru.CategoryGeneral, value
	}

	switch namespace {
	case "artist":
		return booru.CategoryArtist, bare
	case "character", "species":
		return booru.CategoryCharacter, bare
	case "origin":
		return booru.CategoryCopyright, bare
	case "oc":
		return booru.CategoryMeta, bare
	default:
		return booru.CategoryGeneral, bare
	}
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}

	return parsed.UTC()
}
