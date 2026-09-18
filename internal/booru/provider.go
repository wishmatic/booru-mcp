package booru

import (
	"context"
	"fmt"
	"strings"
)

type Capabilities struct {
	Rating bool
	Random bool
}

type SearchParams struct {
	Tags    string
	Limit   int
	Page    int
	Rating  Rating
	Exclude []string
	Random  bool
}

type TagQuery struct {
	Query    string
	Category TagCategory
	Limit    int
	Rating   Rating
}

type PopularQuery struct {
	Category TagCategory
	Limit    int
}

type RelatedQuery struct {
	Tag   string
	Limit int
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	Search(ctx context.Context, params SearchParams) ([]Post, error)
	Post(ctx context.Context, id string) (Post, error)
	SearchTags(ctx context.Context, query TagQuery) ([]Tag, error)
	PopularTags(ctx context.Context, query PopularQuery) ([]Tag, error)
}

type RelatedTagProvider interface {
	RelatedTags(ctx context.Context, query RelatedQuery) ([]RelatedTag, error)
}

type Entry struct {
	Name     string
	Provider Provider
	Active   bool
	Reason   string
}

type Registry struct {
	entries []Entry
	byName  map[string]int
}

func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]int)}
}

func (r *Registry) Register(name string, provider Provider, inactiveReason string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("booru: client name is required")
	}

	if _, ok := r.byName[name]; ok {
		return fmt.Errorf("booru: client %q is already registered", name)
	}

	if provider == nil && inactiveReason == "" {
		return fmt.Errorf("booru: client %q has no provider and no reason", name)
	}

	entry := Entry{Name: name, Provider: provider, Active: inactiveReason == "", Reason: inactiveReason}

	r.byName[name] = len(r.entries)
	r.entries = append(r.entries, entry)

	return nil
}

func (r *Registry) Get(name string) (Entry, error) {
	index, ok := r.byName[name]
	if !ok {
		return Entry{}, fmt.Errorf("booru: unknown client %q", name)
	}

	return r.entries[index], nil
}

func (r *Registry) Active() []Entry {
	out := make([]Entry, 0, len(r.entries))

	for _, entry := range r.entries {
		if entry.Active {
			out = append(out, entry)
		}
	}

	return out
}

func (r *Registry) All() []Entry {
	out := make([]Entry, len(r.entries))
	copy(out, r.entries)

	return out
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.entries))

	for _, entry := range r.entries {
		out = append(out, entry.Name)
	}

	return out
}
