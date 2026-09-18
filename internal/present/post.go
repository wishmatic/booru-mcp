package present

import (
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func Posts(posts []booru.Post) string {
	if len(posts) == 0 {
		return "No posts matched."
	}

	var b strings.Builder

	for i, post := range posts {
		if i > 0 {
			b.WriteString("\n")
		}

		fmt.Fprintf(&b, "- %s\n", post.Ref())
		fmt.Fprintf(&b, "  - permalink: %s\n", post.URL)
		fmt.Fprintf(&b, "  - file: %s\n", fileLine(post.FileURL))
		fmt.Fprintf(&b, "  - preview: %s\n", previewLine(post.PreviewURL))

		fmt.Fprintf(&b, "  - %dx%d, rating %s, score %d\n", post.Width, post.Height, post.Rating, post.Score)

		if len(post.Tags) > 0 {
			fmt.Fprintf(&b, "  - tags: %s\n", strings.Join(tagNames(post), " "))
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func Post(post booru.Post) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", post.Ref())
	fmt.Fprintf(&b, "- permalink: %s\n", post.URL)
	fmt.Fprintf(&b, "- file: %s\n", fileLine(post.FileURL))
	fmt.Fprintf(&b, "- sample: %s\n", sampleLine(post.SampleURL))
	fmt.Fprintf(&b, "- preview: %s\n", previewLine(post.PreviewURL))

	fmt.Fprintf(&b, "- %dx%d\n", post.Width, post.Height)
	fmt.Fprintf(&b, "- rating: %s\n", post.Rating)
	fmt.Fprintf(&b, "- score: %d, favourites: %d\n", post.Score, post.FavCount)

	if post.Source != "" {
		fmt.Fprintf(&b, "- source: %s\n", post.Source)
	}

	if !post.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "- created: %s\n", post.CreatedAt.Format("2006-01-02"))
	}

	grouped := post.TagsByCategory()

	for _, category := range booru.TagCategories() {
		if tags := grouped[category]; len(tags) > 0 {
			fmt.Fprintf(&b, "\n## %s\n\n%s\n", category, strings.Join(tags, " "))
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func fileLine(url string) string {
	if url == "" {
		return "(file URL unavailable for this post)"
	}

	return url
}

func sampleLine(url string) string {
	if url == "" {
		return "(sample URL unavailable for this post)"
	}

	return url
}

func previewLine(url string) string {
	if url == "" {
		return "(preview URL unavailable for this post)"
	}

	return url
}

func tagNames(post booru.Post) []string {
	names := make([]string, 0, len(post.Tags))
	for _, tag := range post.Tags {
		names = append(names, tag.Name)
	}

	return names
}
