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

	b.WriteString("| tag | category | popularity | works | client |\n|---|---|---|---|---|\n")

	for _, tag := range tags {
		fmt.Fprintf(&b, "| %s | %s | %d%% | %d | %s |\n",
			tag.Name, tag.Category, int(tag.Score), tag.Count, tag.Client)
	}

	return strings.TrimRight(b.String(), "\n")
}

func Related(tags []booru.RelatedTag, skipped []booru.Skipped) string {
	var b strings.Builder

	if len(tags) == 0 {
		b.WriteString("No related tags were returned. Related tags are only available from clients with a related-tag " +
			"API, so an empty or short result is expected rather than a failure.\n")
	} else {
		b.WriteString("| tag | client | score |\n|---|---|---|\n")

		for _, tag := range tags {
			fmt.Fprintf(&b, "| %s | %s | %d |\n", tag.Tag, tag.Client, tag.Score)
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
