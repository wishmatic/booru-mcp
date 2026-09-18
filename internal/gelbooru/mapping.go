package gelbooru

import (
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func ratingTerm(rating booru.Rating) string {
	switch rating {
	case booru.RatingGeneral:
		return "rating:general"
	case booru.RatingSensitive:
		return "rating:sensitive"
	case booru.RatingQuestionable:
		return "rating:questionable"
	case booru.RatingExplicit:
		return "rating:explicit"
	default:
		return ""
	}
}

func parseRating(value string) booru.Rating {
	switch value {
	case "general", "safe", "s", "g":
		return booru.RatingGeneral
	case "sensitive":
		return booru.RatingSensitive
	case "questionable", "q":
		return booru.RatingQuestionable
	case "explicit", "e":
		return booru.RatingExplicit
	default:
		return booru.RatingGeneral
	}
}

// Hashbooru tag types match Danbooru's numbering for the shared categories, with 6 for deprecated.
func tagCategory(code int) booru.TagCategory {
	switch code {
	case 1:
		return booru.CategoryArtist
	case 3:
		return booru.CategoryCopyright
	case 4:
		return booru.CategoryCharacter
	case 5:
		return booru.CategoryMeta
	default:
		return booru.CategoryGeneral
	}
}

func parseTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}
