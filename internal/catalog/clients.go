package catalog

import (
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

type resolution struct {
	active  []booru.Entry
	skipped []booru.Skipped
}

func okStatus(client string, results int, detail string) booru.ClientStatus {
	return booru.ClientStatus{Client: client, State: booru.ClientStateOK, Results: results, Detail: detail}
}

func errorStatus(client string, err error) booru.ClientStatus {
	return booru.ClientStatus{Client: client, State: booru.ClientStateError, Detail: err.Error()}
}

func skippedStatus(client, reason string) booru.ClientStatus {
	return booru.ClientStatus{Client: client, State: booru.ClientStateSkipped, Detail: reason}
}

func skippedStatuses(skipped []booru.Skipped) []booru.ClientStatus {
	out := make([]booru.ClientStatus, 0, len(skipped))

	for _, entry := range skipped {
		out = append(out, skippedStatus(entry.Client, entry.Reason))
	}

	return out
}

func (s *Service) resolve(requested []string) (resolution, error) {
	names := requested
	if len(names) == 0 {
		names = s.opts.DefaultClients
	}

	var out resolution
	seen := make(map[string]bool, len(names))

	for _, name := range names {
		if seen[name] {
			continue
		}

		seen[name] = true

		entry, err := s.registry.Get(name)
		if err != nil {
			return resolution{}, fmt.Errorf("catalog: %w", err)
		}

		if !entry.Active {
			out.skipped = append(out.skipped, booru.Skipped{Client: name, Reason: entry.Reason})

			continue
		}

		out.active = append(out.active, entry)
	}

	return out, nil
}
