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
		fmt.Fprintf(&b, "- %s (%s): %d works", tag.Name, tag.Category, tag.Count)

		if tag.AliasOf != "" {
			fmt.Fprintf(&b, " [alias of %s]", tag.AliasOf)
		}

		if len(tag.Implications) > 0 {
			fmt.Fprintf(&b, " [implies %s]", strings.Join(tag.Implications, ", "))
		}

		b.WriteString("\n")
	}

	if out.More {
		fmt.Fprintf(&b, "\nMore matches exist; continue with offset %d.\n", out.Offset+len(out.Tags))
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderEmpty(out tagsOutput) string {
	switch out.Status {
	case string(catalog.StatusNoSubstringMatch):
		return fmt.Sprintf("No tag name contains %q.", out.Search)

	case string(catalog.StatusExactNotFound):
		return fmt.Sprintf("No tag named %q exists.", out.Search)

	case string(catalog.StatusUnknown):
		return fmt.Sprintf("Could not confirm whether %q exists because the tag index was unavailable; try again.", out.Search)
	}

	return "No tags matched."
}
