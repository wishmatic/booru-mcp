<img src="docs/images/logo.webp" alt="Neo MCP Logo" width="128">

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
- `related` returns tags that co-occur with a tag. This is a partial-coverage capability: only the Danbooru family
  publishes a related-tag API, the rest are skipped and reported, and a short or empty result is expected.
- `search` searches posts by a booru tag string and returns permalinks and file URLs, grouped in the requested client
  order.
- Every combined tool reports a per-client status: which client answered, how many results it returned, and the reason
  it failed. A client that returned nothing is `ok (0 results)` and stays distinct from one that errored.
- `get` returns one post with every tag grouped by category, its permalink, source, score, and dimensions.
- Images are always returned as permalinks and URLs; no image bytes are ever fetched or returned.
- Tag data is cached in SQLite with a 30 day TTL. Within the TTL it is served from cache; a stale entry is re-fetched
  only when it is a popular tag, otherwise the stale value is served. `refresh` on a call forces a re-fetch, and
  `CACHE_TTL_DAYS=0` disables caching.
- Every outbound API call passes through one global rate limiter, one call per second by default.

### Clients

| Family   | Clients                                     |
| -------- | ------------------------------------------- |
| Danbooru | `danbooru`                                  |
| Gelbooru | `gelbooru`, `rule34`, `xbooru`, `safebooru` |
| Moebooru | `yandere`, `konachan`, `sakugabooru`        |

A client whose credentials are unset never fails startup; it is inactive and contributes nothing, and a call that names
it returns an empty result with the reason.

`rule34` uses the API host `api.rule34.xxx` and requires `RULE34_API_KEY` + `RULE34_USER_ID`; without them it is
inactive. `rule34` and `xbooru` apply country and region restrictions, so either may refuse requests or be unreachable
depending on where the server runs; a refusal is reported as an HTML body rather than a decode error.

`danbooru` caps tags per search by account tier: anonymous and member 2, gold 6, platinum and builder unlimited.
Negated tags and `order:random` count toward the limit while `rating:` does not. `DANBOORU_TIER` defaults to `auto`,
which is anonymous without credentials and gold with them. An over-cap search is rejected before it is sent, with the
cap named. No negative tags are ever added to a query automatically; `BLOCKED_TAGS` is the only source of negation.

`random` uses `order:random` on `danbooru` and the Moebooru clients. A client that cannot randomise reports
`random ordering not applied` in its status instead of silently returning ordered results.

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
- Content is third-party and unreviewed.
    - This project neither controls nor vets any tag, post, or URL it returns.

Also this was, at time of writing, nearly 100% vibe coded with a decent planning phase. Diligence and testing along
with a small surface area of functionality make it safe to use, in our opinion. Work may be done to go over the code
to comphrend, improve, and re-architect it.

## License

Booru MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by Danbooru, Gelbooru, rule34.xxx, Xbooru, Safebooru, yande.re,
Konachan, Sakugabooru, or any other third-party service. Those names are trademarks of their respective owners and are
used here only to describe compatibility.
