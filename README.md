# Booru MCP

An MCP server, written in Go, that gives image generation models real booru tags. It exposes one tool, `tags`, which
searches tag names on Danbooru and returns the matches with their category and work count.

## The `tags` tool

| Argument | Required | Default | Notes                                                                                   |
| -------- | -------- | ------- | --------------------------------------------------------------------------------------- |
| `search` | yes      |         | a literal substring of the tag name, so `blue hair` and `blue_hair` are the same search |
| `offset` | no       | `0`     | how many matching tags to skip                                                          |
| `limit`  | no       | `25`    | how many matching tags to return                                                        |

Results come back ordered by work count as one page of the match list:

```json
{
    "search": "blue_hair",
    "offset": 0,
    "limit": 25,
    "more": true,
    "tags": [{ "name": "blue_hair", "category": "general", "count": 482913 }]
}
```

Walk the list with `offset` and stop when `more` is false. An empty `tags` array with `more` false means no tags match,
which is a definitive answer rather than a failure. Every call is live; nothing is cached or stored, and an upstream
failure is reported as an error rather than as an empty page.

## Usage

Deploy as a Docker image:

```sh
docker run -d \
  -p 8080:8080 \
  -e API_KEY=change-me \
  ghcr.io/wishmatic/booru-mcp:latest
```

The MCP endpoint is served at `/mcp`. `API_KEY` is the only required setting; everything else is optional and is
documented in [.env.example](.env.example).

### Authentication

`API_KEY` is required on every request, sent as `Authorization: Bearer <API_KEY>`.

### Danbooru credentials

Danbooru answers anonymously, so the server works with nothing configured. Set `DANBOORU_LOGIN` and `DANBOORU_API_KEY`
to raise the anonymous rate limit and make a 429 less likely.

### Content policy

`BLOCKED_TAGS` is a comma-separated denylist; a tag on it is omitted from the results. It ships empty, and no list ships
with this repository.

## Warning

This server queries a third-party, NSFW-first API.

- NSFW is the default in the sense that no blocklist ships with this repository.
- Filtering is best-effort, and `BLOCKED_TAGS` is yours to fill.
- Tag names are third-party and unreviewed. This project neither controls nor vets any tag it returns.

Also this was, at time of writing, nearly 100% vibe coded with a decent planning phase. Diligence and testing along
with a small surface area of functionality make it safe to use, in our opinion.

## License

Booru MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by Danbooru or any other third-party service. Those names are
trademarks of their respective owners and are used here only to describe compatibility.
