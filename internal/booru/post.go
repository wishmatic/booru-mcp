package booru

import "time"

type Post struct {
	Client     string
	ID         string
	URL        string
	FileURL    string
	SampleURL  string
	PreviewURL string
	Width      int
	Height     int
	Rating     Rating
	Score      int
	FavCount   int
	Source     string
	Tags       []Tag
	CreatedAt  time.Time
}

func (p Post) Ref() string {
	return p.Client + ":" + p.ID
}

func (p Post) TagsByCategory() map[TagCategory][]string {
	grouped := make(map[TagCategory][]string, len(TagCategories()))

	for _, category := range TagCategories() {
		grouped[category] = nil
	}

	for _, tag := range p.Tags {
		grouped[tag.Category] = append(grouped[tag.Category], tag.Name)
	}

	return grouped
}
