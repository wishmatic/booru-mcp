# Booru MCP

An MCP server, written in Go, that gives image generation models real booru tags and real booru images from platforms
that publish a JSON API. It is NSFW-first: adult clients are enabled by default and results are uncapped by default.

## Features

- `popular` returns popular tags combined across the requested clients. Rankings are fused with reciprocal rank
  fusion, and every tag carries each contributing client's own count, rank, and within-query percentage. Raw counts are
  never summed across clients, because they come from different corpora.
- `tags` searches tags and returns them sorted by popularity. Popularity is a percentage within the query: each
  client's strongest match is 100%, which is what makes results comparable across sites. Same-name results collapse to
  the highest-percentage instance.
- `related` returns tags that co-occur with a tag. This is a partial-coverage capability: only the Danbooru and e621
  families publish a related-tag API, the rest are skipped and reported, and a short or empty result is expected.
- `search` searches posts by a booru tag string and returns permalinks and file URLs, grouped in the requested client
  order.
- `get` returns one post with every tag grouped by category, its permalink, source, score, and dimensions.
- Images are always returned as permalinks and URLs; no image bytes are ever fetched or returned.
- Tag data is cached in SQLite with a 30 day TTL. Within the TTL it is served from cache; a stale entry is re-fetched
  only when it is a popular tag, otherwise the stale value is served. `refresh` on a call forces a re-fetch, and
  `CACHE_TTL_DAYS=0` disables caching.
- Every outbound API call passes through one global rate limiter, one call per second by default.

### Clients

| Family    | Clients                                                                        |
| --------- | ------------------------------------------------------------------------------ |
| Danbooru  | `danbooru`, `aibooru`                                                           |
| Gelbooru  | `gelbooru`, `rule34`, `realbooru`, `xbooru`, `tbib`, `safebooru`                |
| Moebooru  | `yandere`, `konachan`, `sakugabooru`                                            |
| e621      | `e621`, `e926`                                                                  |
| Philomena | `derpibooru`, `twibooru`, `furbooru`                                            |

A client whose credentials are unset never fails startup; it is inactive and contributes nothing, and a call that names
it returns an empty result with the reason.

## Usage

Deploy as a Docker image:

```sh
docker run -d \
  -p 8080:8080 \
  -e API_KEY=change-me \
  -v /mnt/user/appdata/booru-mcp:/data \
  ghcr.io/wishmatic/booru-mcp:latest
```

The MCP endpoint is served at `/mcp`. `/data` holds the SQLite cache, so bind-mount a host directory there; the
container runs as uid 65532, so that directory must be writable by it.

All other configuration is optional but recommended; see [.env.example](.env.example).

### Authentication

`API_KEY` is required on every request, sent as `Authorization: Bearer <API_KEY>`.

### Content policy

`CONTENT_RATING` caps what any tool returns and defaults to `all`. `BLOCKED_TAGS` is a comma-separated denylist applied
as negated query terms and again as a post-fetch filter. No list ships with this repository.

## Warning

This server aggregates third-party NSFW APIs. The defaults are convenient, not safe.

- NSFW is the default.
- You are responsible for what you serve.
    - Adult material and age verification are regulated per jurisdiction, and `BLOCKED_TAGS` ships empty by design.
- Filtering is best-effort.
    - Applied as query terms where supported and as a post-fetch filter otherwise.
    - Clients that publish no rating metadata are excluded wholesale, not filtered.
- Content is third-party and unreviewed.
    - This project neither controls nor vets any tag, post, or URL it returns.

Also this was, at time of writing, nearly 100% vibe coded with a decent planning phase. Diligence and testing along
with a small surface area of functionality make it safe to use, in our opinion. Work may be done to go over the code
to comphrend, improve, and re-architect it.

## License

Booru MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by Danbooru, Aibooru, Gelbooru, rule34.xxx, Realbooru, Xbooru, TBIB,
Safebooru, yande.re, Konachan, Sakugabooru, e621, e926, Derpibooru, Twibooru, Furbooru, or any other third-party
service. Those names are trademarks of their respective owners and are used here only to describe compatibility.
