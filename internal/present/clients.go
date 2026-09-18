package present

import (
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

// ClientStatuses renders one line per client that was queried or failed. Skipped clients are already named by Skipped,
// so they are left out here to avoid reporting them twice.
func ClientStatuses(statuses []booru.ClientStatus) string {
	lines := make([]string, 0, len(statuses))

	for _, status := range statuses {
		switch status.State {
		case booru.ClientStateOK:
			lines = append(lines, fmt.Sprintf("- %s: ok (%d results%s)", status.Client, status.Results, detailSuffix(status.Detail)))
		case booru.ClientStateError:
			lines = append(lines, fmt.Sprintf("- %s: error (%s)", status.Client, status.Detail))
		}
	}

	if len(lines) == 0 {
		return ""
	}

	return "Clients:\n" + strings.Join(lines, "\n") + "\n"
}

func detailSuffix(detail string) string {
	if detail == "" {
		return ""
	}

	return "; " + detail
}
