package present

import (
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func Tags(tags []booru.FusedTag) string {
	if len(tags) == 0 {
		return "No tags matched."
	}

	var b strings.Builder

	for _, tag := range tags {
		fmt.Fprintf(&b, "- %s (%s): popularity %d%%, %d works on %s\n",
			tag.Name, tag.Category, int(tag.Score), tag.Count, tag.Client)
	}

	return strings.TrimRight(b.String(), "\n")
}

func Popular(tags []booru.FusedTag) string {
	if len(tags) == 0 {
		return "No popular tags were returned."
	}

	var b strings.Builder

	for _, tag := range tags {
		fmt.Fprintf(&b, "- %s (%s)\n", tag.Name, tag.Category)

		for _, count := range tag.Clients {
			fmt.Fprintf(&b, "  - %s: %d works, rank %d, %d%%\n",
				count.Client, count.Count, count.Rank, count.Pct)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func Related(tags []booru.RelatedTag, skipped []booru.Skipped) string {
	var b strings.Builder

	if len(tags) == 0 {
		b.WriteString("No related tags were returned. Related tags are only available from clients with a related-tag " +
			"API, so an empty or short result is expected rather than a failure.\n")
	} else {
		for _, tag := range tags {
			fmt.Fprintf(&b, "- %s (%s): score %d\n", tag.Tag, tag.Client, tag.Score)
		}

		b.WriteString("Related tags are only available from clients with a related-tag API, so this list may be shorter " +
			"than requested.\n")
	}

	if note := Skipped(skipped); note != "" {
		b.WriteString(note)
	}

	return strings.TrimRight(b.String(), "\n")
}

func Skipped(skipped []booru.Skipped) string {
	if len(skipped) == 0 {
		return ""
	}

	parts := make([]string, 0, len(skipped))
	for _, entry := range skipped {
		parts = append(parts, fmt.Sprintf("%s (%s)", entry.Client, entry.Reason))
	}

	return "Skipped clients: " + strings.Join(parts, "; ") + ".\n"
}
