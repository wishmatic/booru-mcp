package mcp

import (
	"fmt"
	"strings"
)

func renderTags(out tagsOutput) string {
	if len(out.Tags) == 0 {
		return "No tags matched."
	}

	var b strings.Builder

	for _, tag := range out.Tags {
		fmt.Fprintf(&b, "- %s (%s): %d works\n", tag.Name, tag.Category, tag.Count)
	}

	if out.More {
		fmt.Fprintf(&b, "\nMore matches exist; continue with offset %d.\n", out.Offset+len(out.Tags))
	}

	return strings.TrimRight(b.String(), "\n")
}
