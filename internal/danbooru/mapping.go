package danbooru

import "github.com/wishmatic/booru-mcp/internal/booru"

func ratingCode(rating booru.Rating) string {
	switch rating {
	case booru.RatingGeneral:
		return "g"
	case booru.RatingSensitive:
		return "s"
	case booru.RatingQuestionable:
		return "q"
	case booru.RatingExplicit:
		return "e"
	default:
		return ""
	}
}

func parseRating(value string) booru.Rating {
	switch value {
	case "g":
		return booru.RatingGeneral
	case "s":
		return booru.RatingSensitive
	case "q":
		return booru.RatingQuestionable
	case "e":
		return booru.RatingExplicit
	default:
		return booru.RatingGeneral
	}
}

// Danbooru numbers categories general 0, artist 1, copyright 3, character 4, and meta 5; 2 is skipped.
func categoryCode(category booru.TagCategory) int {
	switch category {
	case booru.CategoryArtist:
		return 1
	case booru.CategoryCopyright:
		return 3
	case booru.CategoryCharacter:
		return 4
	case booru.CategoryMeta:
		return 5
	default:
		return 0
	}
}

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
