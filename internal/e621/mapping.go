package e621

import (
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func ratingTerm(rating booru.Rating) string {
	switch rating {
	case booru.RatingGeneral:
		return "rating:s"
	case booru.RatingQuestionable:
		return "rating:q"
	case booru.RatingExplicit:
		return "rating:e"
	default:
		return ""
	}
}

func parseRating(value string) booru.Rating {
	switch value {
	case "s":
		return booru.RatingGeneral
	case "q":
		return booru.RatingQuestionable
	case "e":
		return booru.RatingExplicit
	default:
		return booru.RatingGeneral
	}
}

// e621 categories are general 0, artist 1, copyright 3, character 4, species 5, invalid 6, meta 7, and lore 8. Species
// folds into character and lore into meta, and invalid degrades to general.
func tagCategory(code int) booru.TagCategory {
	switch code {
	case 1:
		return booru.CategoryArtist
	case 3:
		return booru.CategoryCopyright
	case 4, 5:
		return booru.CategoryCharacter
	case 7, 8:
		return booru.CategoryMeta
	default:
		return booru.CategoryGeneral
	}
}

func postTags(groups tagGroups) []booru.Tag {
	grouped := []struct {
		category booru.TagCategory
		values   []string
	}{
		{booru.CategoryGeneral, groups.General},
		{booru.CategoryGeneral, groups.Invalid},
		{booru.CategoryArtist, groups.Artist},
		{booru.CategoryCopyright, groups.Copyright},
		{booru.CategoryCharacter, groups.Character},
		{booru.CategoryCharacter, groups.Species},
		{booru.CategoryMeta, groups.Meta},
		{booru.CategoryMeta, groups.Lore},
	}

	var tags []booru.Tag

	for _, entry := range grouped {
		for _, name := range entry.values {
			if normalized := booru.NormalizeTag(name); normalized != "" {
				tags = append(tags, booru.Tag{Name: normalized, Category: entry.category})
			}
		}
	}

	return tags
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}

	return parsed.UTC()
}
