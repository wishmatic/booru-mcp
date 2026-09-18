package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/booru-mcp/internal/booru"
)

type clientsInput struct {
	Clients []string `json:"clients,omitempty" jsonschema:"optional: clients to query, in order; defaults to the server's DEFAULT_CLIENTS. Unknown clients error; inactive ones are skipped and reported"`
}

type categoryInput struct {
	Category string `json:"category,omitempty" jsonschema:"optional: restrict to a tag category: general, artist, copyright, character, or meta"`
}

type ratingInput struct {
	Rating string `json:"rating,omitempty" jsonschema:"optional: the highest rating to return: general, sensitive, questionable, explicit, or all. Narrows CONTENT_RATING and can never widen it"`
}

type clientCountOutput struct {
	Client string `json:"client"`
	Count  int    `json:"count"`
	Rank   int    `json:"rank"`
	Pct    int    `json:"pct"`
}

type tagOutput struct {
	Name     string              `json:"name"`
	Category string              `json:"category"`
	Score    float64             `json:"score"`
	Count    int                 `json:"count"`
	Client   string              `json:"client,omitempty"`
	Clients  []clientCountOutput `json:"clients,omitempty"`
}

type skippedOutput struct {
	Client string `json:"client"`
	Reason string `json:"reason"`
}

type clientStatusOutput struct {
	Client  string `json:"client"`
	Status  string `json:"status"`
	Results int    `json:"results,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type postOutput struct {
	Client     string              `json:"client"`
	ID         string              `json:"id"`
	Permalink  string              `json:"permalink"`
	FileURL    string              `json:"file_url,omitempty"`
	SampleURL  string              `json:"sample_url,omitempty"`
	PreviewURL string              `json:"preview_url,omitempty"`
	Width      int                 `json:"width,omitempty"`
	Height     int                 `json:"height,omitempty"`
	Rating     string              `json:"rating,omitempty"`
	Score      int                 `json:"score,omitempty"`
	FavCount   int                 `json:"fav_count,omitempty"`
	Source     string              `json:"source,omitempty"`
	Tags       []string            `json:"tags,omitempty"`
	TagGroups  map[string][]string `json:"tag_groups,omitempty"`
	CreatedAt  string              `json:"created_at,omitempty"`
}

func toTagOutputs(tags []booru.FusedTag) []tagOutput {
	out := make([]tagOutput, 0, len(tags))

	for _, tag := range tags {
		entry := tagOutput{
			Name:     tag.Name,
			Category: tag.Category.String(),
			Score:    tag.Score,
			Count:    tag.Count,
			Client:   tag.Client,
		}

		for _, count := range tag.Clients {
			entry.Clients = append(entry.Clients, clientCountOutput{
				Client: count.Client,
				Count:  count.Count,
				Rank:   count.Rank,
				Pct:    count.Pct,
			})
		}

		out = append(out, entry)
	}

	return out
}

func toClientStatusOutputs(statuses []booru.ClientStatus) []clientStatusOutput {
	out := make([]clientStatusOutput, 0, len(statuses))

	for _, status := range statuses {
		out = append(out, clientStatusOutput{
			Client:  status.Client,
			Status:  string(status.State),
			Results: status.Results,
			Detail:  status.Detail,
		})
	}

	return out
}

func toSkippedOutputs(skipped []booru.Skipped) []skippedOutput {
	out := make([]skippedOutput, 0, len(skipped))

	for _, entry := range skipped {
		out = append(out, skippedOutput{Client: entry.Client, Reason: entry.Reason})
	}

	return out
}

func toPostOutput(post booru.Post, grouped bool) postOutput {
	out := postOutput{
		Client:     post.Client,
		ID:         post.ID,
		Permalink:  post.URL,
		FileURL:    post.FileURL,
		SampleURL:  post.SampleURL,
		PreviewURL: post.PreviewURL,
		Width:      post.Width,
		Height:     post.Height,
		Rating:     post.Rating.String(),
		Score:      post.Score,
		FavCount:   post.FavCount,
		Source:     post.Source,
	}

	if !post.CreatedAt.IsZero() {
		out.CreatedAt = post.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	if grouped {
		out.TagGroups = map[string][]string{}

		for _, category := range booru.TagCategories() {
			if tags := post.TagsByCategory()[category]; len(tags) > 0 {
				out.TagGroups[category.String()] = tags
			}
		}

		return out
	}

	for _, tag := range post.Tags {
		out.Tags = append(out.Tags, tag.Name)
	}

	return out
}

func schemaFor[T any](label string) *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("%s: infer input schema: %v", label, err))
	}

	return schema
}

func setDefault(schema *jsonschema.Schema, name string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal default for %s: %v", name, err))
	}

	schema.Properties[name].Default = raw
}

func setEnum(schema *jsonschema.Schema, name string, values []string) {
	enum := make([]any, 0, len(values))
	for _, value := range values {
		enum = append(enum, value)
	}

	schema.Properties[name].Enum = enum
}

func setCategoryEnum(schema *jsonschema.Schema) {
	values := make([]string, 0, len(booru.TagCategories()))
	for _, category := range booru.TagCategories() {
		values = append(values, category.String())
	}

	setEnum(schema, "category", values)
}

func setRatingEnum(schema *jsonschema.Schema) {
	values := make([]string, 0, len(booru.Ratings()))
	for _, rating := range booru.Ratings() {
		values = append(values, rating.String())
	}

	setEnum(schema, "rating", values)
}
