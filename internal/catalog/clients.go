package catalog

import (
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

type resolution struct {
	active  []booru.Entry
	skipped []booru.Skipped
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
