package mcp

import (
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/catalog"
)

func renderTags(out tagsOutput) string {
	if len(out.Tags) == 0 {
		return renderEmpty(out)
	}

	var b strings.Builder

	for _, tag := range out.Tags {
		fmt.Fprint(&b, renderTag(tag))

		if len(tag.Implications) > 0 {
			fmt.Fprintf(&b, " [implies %s]", strings.Join(tag.Implications, ", "))
		}

		b.WriteString("\n")
	}

	if len(out.Synonyms) > 0 {
		fmt.Fprintf(&b, "\nAlso known as: %s.\n", strings.Join(out.Synonyms, ", "))
	}

	if out.More {
		fmt.Fprintf(&b, "\nMore matches exist; continue with offset %d.\n", out.Offset+len(out.Tags))
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderTag(tag tagOutput) string {
	if tag.AliasOf == "" {
		return fmt.Sprintf("- %s (%s): %d works", tag.Name, tag.Category, tag.Count)
	}

	if tag.CountIsTarget {
		return fmt.Sprintf("- %s (%s): alias of %s (%d works there)", tag.Name, tag.Category, tag.AliasOf, tag.Count)
	}

	return fmt.Sprintf("- %s (%s): alias of %s", tag.Name, tag.Category, tag.AliasOf)
}

func renderEmpty(out tagsOutput) string {
	switch out.Status {
	case string(catalog.StatusNoSubstringMatch):
		return renderNoSubstringMatch(out)

	case string(catalog.StatusOffsetPastEnd):
		return fmt.Sprintf(
			"Offset %d is past the last result for %q; there is no further page. Try a smaller offset.",
			out.Offset, out.Search,
		) + renderSynonyms(out)

	case string(catalog.StatusExactNotFound):
		return fmt.Sprintf("No tag named %q exists.", out.Search)

	case string(catalog.StatusUnknown):
		return fmt.Sprintf("Could not confirm whether %q exists because the tag index was unavailable; try again.", out.Search)
	}

	return "No tags matched."
}

func renderSynonyms(out tagsOutput) string {
	if len(out.Synonyms) == 0 {
		return ""
	}

	return fmt.Sprintf(" Known aliases: %s.", strings.Join(out.Synonyms, ", "))
}

// renderNoSubstringMatch names the filter that emptied the page. A withheld canonical tag exists; saying only that the
// substring matched nothing would assert that it does not.
func renderNoSubstringMatch(out tagsOutput) string {
	if out.Withheld == 0 {
		return fmt.Sprintf("No tag name contains %q.", out.Search) + renderSynonyms(out)
	}

	if out.Withheld == 1 {
		return fmt.Sprintf(
			"No tag name contains %q with at least %d works; one canonical match with %d works was hidden by the floor. "+
				"Set min_count=0 to include it.",
			out.Search, out.MinCount, out.WithheldBest,
		) + renderSynonyms(out)
	}

	return fmt.Sprintf(
		"No tag name contains %q with at least %d works; %d canonical matches were hidden by the floor (the best has %d "+
			"works). Set min_count=0 to include them.",
		out.Search, out.MinCount, out.Withheld, out.WithheldBest,
	) + renderSynonyms(out)
}
