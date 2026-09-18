package booru

import (
	"fmt"
	"math"
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

func TagCategories() []TagCategory {
	return []TagCategory{CategoryGeneral, CategoryArtist, CategoryCopyright, CategoryCharacter, CategoryMeta}
}

func ParseTagCategory(value string) (TagCategory, error) {
	category := TagCategory(strings.ToLower(strings.TrimSpace(value)))

	for _, known := range TagCategories() {
		if category == known {
			return category, nil
		}
	}

	return "", fmt.Errorf("booru: unknown tag category %q", value)
}

func (c TagCategory) String() string {
	return string(c)
}

type Rating string

const (
	RatingGeneral      Rating = "general"
	RatingSensitive    Rating = "sensitive"
	RatingQuestionable Rating = "questionable"
	RatingExplicit     Rating = "explicit"
	RatingAll          Rating = "all"
)

func Ratings() []Rating {
	return []Rating{RatingGeneral, RatingSensitive, RatingQuestionable, RatingExplicit, RatingAll}
}

func ParseRating(value string) (Rating, error) {
	rating := Rating(strings.ToLower(strings.TrimSpace(value)))

	for _, known := range Ratings() {
		if rating == known {
			return rating, nil
		}
	}

	return "", fmt.Errorf("booru: unknown rating %q", value)
}

func (r Rating) String() string {
	return string(r)
}

// Rank orders ratings from least to most explicit. An unset rating is rank -1, meaning no constraint, and `all` sits
// above explicit so a cap of `all` never narrows anything.
func (r Rating) Rank() int {
	switch r {
	case RatingGeneral:
		return 0
	case RatingSensitive:
		return 1
	case RatingQuestionable:
		return 2
	case RatingExplicit:
		return 3
	case RatingAll:
		return 4
	default:
		return -1
	}
}

func (r Rating) IsUnset() bool {
	return r.Rank() < 0
}

type Tag struct {
	Name     string
	Category TagCategory
	Count    int
}

type TagCount struct {
	Client string
	Count  int
	Rank   int
	Pct    int
}

type FusedTag struct {
	Name     string
	Category TagCategory
	Score    float64
	Total    int
	Client   string
	Count    int
	Clients  []TagCount
}

type RelatedTag struct {
	Tag    string
	Client string
	Score  float64
	Rank   int
}

type Skipped struct {
	Client string
	Reason string
}

type ClientState string

const (
	ClientStateOK      ClientState = "ok"
	ClientStateError   ClientState = "error"
	ClientStateSkipped ClientState = "skipped"
)

// ClientStatus accounts for one requested client on a combined call. A client that succeeded with no results is
// ClientStateOK with zero Results, which is how it stays distinguishable from one that failed.
type ClientStatus struct {
	Client  string
	State   ClientState
	Results int
	Detail  string
}

func NormalizeTag(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "_")
}

func Percent(count, max int) int {
	if count <= 0 || max <= 0 {
		return 0
	}

	return int(math.Round(float64(count) / float64(max) * 100))
}
