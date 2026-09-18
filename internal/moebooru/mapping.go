package moebooru

import (
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func ratingTerm(rating booru.Rating) string {
	switch rating {
	case booru.RatingGeneral:
		return "rating:safe"
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

// Moebooru tag types are general 0, artist 1, studio 2, copyright 3, character 4, and circle 5; studio and circle fold
// into artist and copyright. Anything unrecognized degrades to general rather than failing the call.
func tagCategory(code int) booru.TagCategory {
	switch code {
	case 1, 2:
		return booru.CategoryArtist
	case 3, 5:
		return booru.CategoryCopyright
	case 4:
		return booru.CategoryCharacter
	default:
		return booru.CategoryGeneral
	}
}

func parseTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}

	return time.Unix(value, 0).UTC()
}
