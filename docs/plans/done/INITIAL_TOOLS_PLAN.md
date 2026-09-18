# Initial tools: popular tags, tag search, related tags, image search, and get

Status: Done. All fourteen units are implemented and the suite passes with `-race`. The human live-upstream checks
under Verification are still outstanding: they need real sites and operator accounts, so they cannot be run here.
Depends on: none
Related: `AGENTS.md`, `.env.example`.

## Goal

Give image generation models real booru tags and real booru images from the popular platforms that publish a JSON API.
The server is NSFW-first: adult clients are enabled by default and results are uncapped by default. Five tools are
exposed: `popular`, `tags`, `related`, `search`, and `get`. Tag data is cached in SQLite with a long TTL so
it survives restarts and is refreshed cheaply. Every outbound API call passes through one global rate limiter so the
server stays polite to upstreams it does not own.

## Requirements recap

Stated by the owner:

- The repo exists to help image generation models by providing actual tags.
- A caching layer with a TTL envar defaulting to 30 days.
- When the cache is invalidated, re-fetches happen only for popular tags.
- `popular` returns popular tags from all clients, combined.
- Clients are anything with an API; scraping is out of scope.
- Every tool takes a list of clients, defaulting to `rule34`, `danbooru`, and `gelbooru`.
- `tags` searches tags instead of uploads, sorted by popularity, with the work count shown for each tag.
- `related` is in scope.
- Images are always returned as permalinks; never as inline bytes.
- A client whose credentials are unset must not fail the server; it contributes nothing.
- All API calls go through a manual global calls-per-second limiter with an envar defaulting to 1 call per second.
- The primary use case is NSFW, so the client list covers every popular API-only platform, named by site.
- No objectionable tag names are committed to this repository.

## Non-goals

- Scraping any site, or automating a browser. Only documented JSON APIs.
- Returning image bytes, base64, or MCP image content. Every image reference is a URL.
- Caching posts, post searches, or image bytes. Only tag data is cached.
- Storing or mirroring the images themselves.
- Letting a caller supply an arbitrary URL for the server to fetch. Providers call only their own configured base URL.
- Multi-user accounts, per-user credentials, or per-user rate budgets. One operator, one credential set per site.
- Shipping a blocked-tag list. See Content policy.

## Design

### Clients

A client is one named site, backed by one of five API-family adapters. The family owns the protocol; the name is the
unit of configuration, enablement, output, and caching. `danbooru` and `rule34` are separate clients even though they
share an adapter, because they are separate corpora with separate credentials.

**Danbooru family** (`internal/danbooru`). Endpoints `/posts.json`, `/posts/<id>.json`, `/tags.json`,
`/related_tag.json`. Exact work counts via `search[order]=count`.

| Client    | Default URL                  | Credentials                                    | Notes                                               |
| --------- | ---------------------------- | ---------------------------------------------- | --------------------------------------------------- |
| `danbooru` | `https://danbooru.donmai.us` | optional `DANBOORU_LOGIN` + `DANBOORU_API_KEY` | Mixed SFW and NSFW; the richest tag API.            |
| `aibooru`  | `https://aibooru.online`     | optional `AIBOORU_LOGIN` + `AIBOORU_API_KEY`   | AI-generated corpus; excluded from the default set. |

**Gelbooru family** (`internal/gelbooru`), also known as Hashbooru. Endpoints
`index.php?page=dapi&s=post&q=index&json=1` and `...&s=tag&q=index&json=1&orderby=count`. The credential requirement is
a property of each site, not the protocol, so each client carries its own key.

| Client      | Default URL             | Credentials                                | Notes                                                       |
| ----------- | ----------------------- | ------------------------------------------ | ----------------------------------------------------------- |
| `gelbooru`  | `https://gelbooru.com`  | `GELBOORU_API_KEY` + `GELBOORU_USER_ID`    | Largest general booru; both required.                       |
| `rule34`    | `https://rule34.xxx`    | `RULE34_API_KEY` + `RULE34_USER_ID`        | Adult-only; both required.                                  |
| `realbooru` | `https://realbooru.com` | `REALBOORU_API_KEY` + `REALBOORU_USER_ID`  | Photo-oriented adult; both required.                        |
| `xbooru`    | `https://xbooru.com`    | `XBOORU_API_KEY` + `XBOORU_USER_ID`        | Adult; requirement confirmed in Unit 6.                     |
| `tbib`      | `https://tbib.org`      | `TBIB_API_KEY` + `TBIB_USER_ID`            | The Big ImageBoard; requirement confirmed in Unit 6.        |
| `safebooru` | `https://safebooru.org` | none                                       | SFW; the family's only keyless member.                      |

**Moebooru family** (`internal/moebooru`). Endpoints `/post.json`, `/tag.json`, plus `/post/popular_recent.json`.
Keyless.

| Client        | Default URL              | Credentials | Notes                                                        |
| ------------- | ------------------------ | ----------- | ------------------------------------------------------------ |
| `yandere`     | `https://yande.re`       | none        | High-resolution anime art.                                    |
| `konachan`    | `https://konachan.com`   | none        | Questionable and explicit content. `konachan.net` is the SFW mirror; set `KONACHAN_URL` to use it instead. |
| `sakugabooru` | `https://sakugabooru.com` | none       | Video clips with rich tags; SFW.                              |

**e621 family** (`internal/e621`). Endpoints `/posts.json`, `/tags.json`, `/related_tag.json`. Requires a descriptive
`USER_AGENT`; the site rejects requests without one.

| Client | Default URL        | Credentials                                                     | Notes                                              |
| ------ | ------------------ | -------------------------------------------------------------- | -------------------------------------------------- |
| `e621` | `https://e621.net` | optional `E621_LOGIN` + `E621_API_KEY`; `USER_AGENT` required   | Furry; adult by default.                            |
| `e926` | `https://e926.net` | optional `E926_LOGIN` + `E926_API_KEY`; `USER_AGENT` required   | SFW mirror of `e621`; excluded from the default set because combining mirrors double counts. |

**Philomena family** (`internal/philomena`). Endpoints `/api/v1/json/search/posts?q=`, `/api/v1/json/search/tags?q=`.
Tag records carry image counts, which map onto the same work-count field as the other families.

| Client       | Default URL              | Credentials          | Notes                                         |
| ------------ | ------------------------ | -------------------- | --------------------------------------------- |
| `derpibooru` | `https://derpibooru.org` | `DERPIBOORU_API_KEY` | Pony; uses named filters rather than ratings.  |
| `twibooru`   | `https://twibooru.org`   | `TWIBOORU_API_KEY`   | Philomena fork; adult content allowed.         |
| `furbooru`   | `https://furbooru.org`   | `FURBOORU_API_KEY`   | Furry.                                         |

**Deliberately excluded**, with the reason, so the list is defensible and complete:

| Site                            | Reason                                                                    |
| ------------------------------- | ------------------------------------------------------------------------- |
| `chan.sankakucomplex.com`       | API is auth-gated and not openly documented; revisit only if that changes. |
| `rule34.paheal.net`             | Shimmie engine; no JSON API.                                              |
| `www.zerochan.net`              | No public API.                                                            |
| `e-hentai.org` / `exhentai.org` | No API; access is cookie and session based.                               |
| `nhentai.net`                   | No official API; only unofficial clients.                                 |
| `www.pixiv.net`                 | No public API, and it is not a booru.                                     |

Every client is named, registered, referenced, cached, and reported under its site key. Adding a Hashbooru site is a
one-line addition to the family's client table, not a new adapter.

**Inactive clients.** A client whose requirements are unmet is registered but inactive. It is skipped in every combined
call and contributes nothing, which is what keeps a missing credential from failing a `popular` across six clients. A
call that names an inactive client explicitly returns an empty result with the reason, not an error. Requirements are
credentials for the sites that need them, and a non-default `USER_AGENT` for `e621` and `e926`. Absence is logged once
at startup per client and never again.

**Requirements to confirm at implementation time.** Credential requirements drift. Each provider unit verifies its
site's current policy against live documentation before the adapter is considered done, and records the outcome in
`.env.example`. A site that turns out to need credentials the operator cannot obtain stays inert, exactly like any
other inactive client.

### Canonical tag model

Sites disagree about spelling and categories, so everything is normalized on the way in and rendered canonically out.

- Names are lowercased, trimmed, internal whitespace collapsed to `_`, and rendered with underscores.
  Danbooru already uses underscores; Gelbooru returns spaces inside a single tags string. Normalizing here is what
  makes cross-client matching possible.
- `TagCategory` is `general`, `artist`, `copyright`, `character`, or `meta`. Danbooru's numeric categories map
  directly; Gelbooru matches except it uses `6` for deprecated; Moebooru's studio and circle types fold into `artist`
  and `copyright` respectively; Philomena's `origin`, `species`, and `oc` types fold into `copyright`, `character`, and
  `meta`. Every non-obvious mapping is documented at the mapping site.
- `Rating` is `general`, `sensitive`, `questionable`, or `explicit`. Danbooru uses `g/s/q/e`, Gelbooru spells them out,
  Moebooru and e621 use `s/q/e`, and Philomena has no rating field at all. The Philomena case is called out because it
  is the one family that cannot answer a rating query.
- A post is identified canonically as `<client>:<id>`, because ids collide across sites.

### Tool surface

Every tool takes `clients`, an optional list of client names. When omitted, the default list is used, which is
`rule34,danbooru,gelbooru` out of the box. Clients that are unknown error; clients that are known but inactive are
skipped in a combined call and reported when named alone.

| Tool      | Purpose                              | Cache               | Clients                    |
| --------- | ------------------------------------ | ------------------- | -------------------------- |
| `popular` | Popular tags from many clients       | tag cache           | list, default set          |
| `tags`    | Tag search sorted by popularity      | tag cache           | list, default set          |
| `related` | Tags that co-occur with a tag        | related-tag cache   | capable clients only       |
| `search`  | Post (image) search by tags          | none                | list, default set          |
| `get`     | One post with everything on it       | none                | single client, `danbooru`  |

`popular` inputs: `clients`, `category` (optional enum), `limit` (optional, default 25), `refresh` (optional bool).
Output: each tag with its category, fused score, and a per-client breakdown of count, rank, and percentage.

`tags` inputs: `query` (required, substring match), `clients`, `category` (optional), `limit` (optional, default 25),
`refresh` (optional bool). Results are sorted by popularity descending, and each shows its work count and a percentage.
Same-name results across clients collapse to the single highest-percentage instance, and the output names the client it
came from. Different names are never merged, even when they look similar.

Popularity here is a percentage within the query, not an absolute count: for each client, that client's highest count
in its own result set is 100%, and every other tag is `count / highest * 100`, rounded to a whole number. So 100% means
"the strongest match for this query on that client", never "the most popular tag on the site". It is what makes counts
from different corpora comparable enough to rank and deduplicate, and close enough is the deliberate standard.

`related` inputs: `tag` (required), `clients`, `limit` (optional, default 25), `refresh` (optional bool). Output: tags
that co-occur with the input, ordered by the client's relatedness score, each with its client. `clients` is restricted
to the clients that actually implement a related-tag API, and defaults to those intersected with the default client
list.

`related` is the one tool that is expected to return less than asked for, so its tool description, its schema field
descriptions, its structured output, and its text output all carry the same wording, and none of them presents a
partial result as a failure:

- related tags only exist on clients that implement a related-tag API, which today is the Danbooru family
  (`danbooru`, `aibooru`) and the e621 family (`e621`, `e926`);
- the `clients` schema is an enum of exactly those clients, so a value outside it is rejected before any call is made;
- any other client that reaches the handler anyway is skipped, and the result names each skipped client with the reason;
- fewer results than `limit` is normal, and an empty result with a reason is normal;
- an empty result does not mean the tag has no relatives, only that no requested client could answer.

The `clients` field description repeats the restriction. The tool description states outright that an empty or short
result is expected behaviour, that it should not be retried, and that it does not mean the tool is broken.

`search` inputs: `tags` (required, booru tag string), `clients`, `limit` (optional, default 20), `page` (optional,
default 1), `rating` (optional, narrows only), `random` (optional bool). Output: posts with their permalink, file,
sample, and preview URLs, dimensions, rating, score, file type, and a truncated tag list. Results are grouped in the
requested client order, each group preserving that client's native relevance order, because ordering across corpora is
not comparable. Interleaving across clients is deliberately not attempted; the within-query percentage used for tags
does not apply to post ordering.

`get` inputs: `id` (required), `client` (optional). A post id is only meaningful within one site, so `get` resolves
exactly one client: `client` when given, otherwise `danbooru`. Output: the full post, with tags grouped by category,
permalink, source, score, favourite count, dimensions, rating, and the file, sample, and preview URLs. Every image
reference is a URL; no bytes are ever fetched or returned.

### Caching

SQLite via `modernc.org/sqlite`, matching the reference project's storage choice. Three tables:

```sql
CREATE TABLE tags (
    client     TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    category   TEXT    NOT NULL,
    count      INTEGER NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (client, name)
);

CREATE TABLE popular_tags (
    client     TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    rank       INTEGER NOT NULL,
    count      INTEGER NOT NULL,
    category   TEXT    NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (client, name)
);

CREATE TABLE related_tags (
    client     TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    related    TEXT    NOT NULL,
    score      INTEGER NOT NULL,
    rank       INTEGER NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (client, name, related)
);
```

`CACHE_TTL_DAYS` (default 30) decides staleness. A row is fresh when `fetched_at` is within the TTL. The refresh rule
is the crux of the requirement, so it is stated precisely:

1. Fresh row: serve from cache, no upstream call.
2. Never-seen tag (miss): fetch upstream, store, serve. A miss is not a re-fetch.
3. Stale row for a name present in `popular_tags`: re-fetch upstream, replace, serve.
4. Stale row for a name not present in `popular_tags`: serve the stale value and do **not** re-fetch.
5. `refresh: true` on a call, or a `popular` call whose snapshot is stale: re-fetch regardless of rule 4.

So long-tail counts can go stale indefinitely, which is the intended trade, confirmed by the owner: the tail is
fetched once and only refreshed if it turns out to be popular or a caller explicitly asks.

`CACHE_TTL_DAYS=0` disables caching entirely: every tag read goes upstream (still rate limited), and nothing is served
stale. This is the escape hatch for debugging and for operators who would rather not persist.

### Combining popular tags across clients

Counts are not comparable across corpora. Danbooru's `blue_eyes` count and yande.re's are different populations, so
summing them is meaningless and picking a "total works" number would be a lie. `popular` instead fuses rankings:

- Each client returns its top `M` tags by count, where `M = min(MAX_LIMIT, limit * 4)`, so the merge has headroom.
- For every tag name, the fused score is reciprocal rank fusion: `sum over clients of 1 / (60 + rank)`, with rank
  1-based. This is confirmed by the owner.
- Ties break on summed count, then on name, for determinism.
- Output keeps each client's own count, rank, and percentage alongside the fused score, so a consumer can see where a
  tag is strong.

`related` is per-client by nature, so it is not fused: results are merged into one list across the requested clients,
ordered by each client's own rank, with the client named on every row, and same-name rows from different clients kept
separately because a co-occurrence score has no cross-site meaning.

`tags` ranks and deduplicates on the within-query percentage defined above rather than on raw counts, so a tag that is
the standout match on one client beats a tag that is a weak match on another even when the raw counts are not
comparable. The same name on two clients survives once, on the client where it scores the higher percentage, and that
client's raw count is still shown. Similar but differently named tags are left alone.

### Rate limiting, retries, and timeouts

One `golang.org/x/time/rate` limiter is shared by every client, constructed once in `internal/server` and injected into
each provider's transport. `RATE_LIMIT_RPS` (default `1`) is the sustained rate, `RATE_LIMIT_BURST` (default `1`) the
burst. With the default and every client active, a cold `popular` fans out to roughly one call per client and therefore
takes about as many seconds as there are clients. That is the cost of staying under one request per second, and it is
paid once per TTL because the result is cached.

The transport:

- Acquires a token before every attempt, including retries.
- Retries `429` and `5xx` up to three attempts total, honouring `Retry-After` when present (seconds or HTTP date),
  otherwise a short capped exponential backoff.
- Applies `REQUEST_TIMEOUT_SECONDS` (default `20`) per attempt.
- Sets `USER_AGENT` on every request.

### Content policy

The server is NSFW-first, so the default is uncapped and the adult-only clients are enabled. Operators who want less
narrow it; the mechanism is the same in both directions.

- `CONTENT_RATING` (default `all`) caps what any tool returns. `general`, `sensitive`, `questionable`, and `explicit`
  are the other accepted values, each meaning "up to and including".
- Per-call `rating` on `search` may narrow, never widen beyond `CONTENT_RATING`.
- Clients that cannot answer a rating query are excluded from the result set when `CONTENT_RATING` is narrower than
  `explicit`, rather than being filtered within it, because filtering there would be a false claim.
- Blocked tags are an operator concern, not a repository one. `BLOCKED_TAGS` takes a comma-separated list, ships empty,
  and nothing is committed to the repository. When set, the list is applied twice: as negated query terms where the
  client supports them (`-tag`), and as a post-fetch filter over tags and tag names, so a client that ignores negations
  cannot leak blocked content.

### Configuration

Envar naming is `<SITE>_*`, uppercased from the client name, for every named client. Each client also accepts a
`<SITE>_URL` override; the defaults are the URLs in the client tables. Credentials are per site and never logged.

| Envar                     | Default                        | Meaning                                                          |
| ------------------------- | ------------------------------ | ---------------------------------------------------------------- |
| `BOORU_CLIENTS`           | see below                      | Comma-separated clients to enable, or `all` for every client.    |
| `DEFAULT_CLIENTS`         | `rule34,danbooru,gelbooru`     | Clients used when a call does not name any.                      |
| `USER_AGENT`              | `booru-mcp/0.1.0`              | Sent on every upstream request. A non-default value is required for `e621` and `e926`. |
| `RATE_LIMIT_RPS`          | `1`                            | Global outbound calls per second, shared by all clients.          |
| `RATE_LIMIT_BURST`        | `1`                            | Burst above the sustained rate.                                  |
| `REQUEST_TIMEOUT_SECONDS` | `20`                           | Per-attempt upstream timeout.                                    |
| `MAX_LIMIT`               | `100`                          | Hard cap on results returned by any call.                        |
| `CACHE_TTL_DAYS`          | `30`                           | Tag staleness threshold in days; `0` disables caching.           |
| `DB_PATH`                 | `booru-mcp.db`                 | SQLite path; `/data/booru-mcp.db` in Docker.                     |
| `CONTENT_RATING`          | `all`                          | Highest rating any tool may return; `all` means uncapped.        |
| `BLOCKED_TAGS`            | unset                          | Comma-separated tags excluded from queries and results; ships empty. |
| `DANBOORU_LOGIN`, `DANBOORU_API_KEY`     | unset | Danbooru credentials; both or neither.               |
| `AIBOORU_LOGIN`, `AIBOORU_API_KEY`       | unset | Aibooru credentials; both or neither.                |
| `GELBOORU_API_KEY`, `GELBOORU_USER_ID`   | unset | Gelbooru credentials; both required.                 |
| `RULE34_API_KEY`, `RULE34_USER_ID`       | unset | Rule34 credentials; both required.                   |
| `REALBOORU_API_KEY`, `REALBOORU_USER_ID` | unset | Realbooru credentials; both required.                |
| `XBOORU_API_KEY`, `XBOORU_USER_ID`       | unset | Xbooru credentials; requirement confirmed in Unit 6. |
| `TBIB_API_KEY`, `TBIB_USER_ID`           | unset | TBIB credentials; requirement confirmed in Unit 6.   |
| `E621_LOGIN`, `E621_API_KEY`             | unset | e621 credentials; `USER_AGENT` is the hard requirement. |
| `E926_LOGIN`, `E926_API_KEY`             | unset | e926 credentials; `USER_AGENT` is the hard requirement. |
| `DERPIBOORU_API_KEY`                     | unset | Derpibooru API key.                                  |
| `TWIBOORU_API_KEY`                       | unset | Twibooru API key.                                    |
| `FURBOORU_API_KEY`                       | unset | Furbooru API key.                                    |

Default `BOORU_CLIENTS` is every non-mirror client: `danbooru`, `gelbooru`, `rule34`, `realbooru`, `xbooru`,
`tbib`, `safebooru`, `yandere`, `konachan`, `sakugabooru`, `e621`, `derpibooru`, `twibooru`, `furbooru`.
`all` additionally enables the mirror and side corpora: `aibooru`, `e926`.

Startup validation errors on things that are certainly wrong: an unparseable URL, `RATE_LIMIT_RPS <= 0`,
`CACHE_TTL_DAYS < 0`, an unparseable `CONTENT_RATING`, and an unknown client name. It never errors on an unmet client
requirement; those clients are inactive with a logged reason.
If no client is active at all, startup still succeeds with a warning, and every tool returns an empty result with the
reason. Credentials are never logged.

## Recorded decisions

Owner decisions from the plan review, recorded so later changes do not relitigate them:

1. The client list is confirmed as written, with the default set being every non-mirror client.
2. No objectionable tag names are committed to this repository. The blocked-tag mechanism ships, but the list does not,
   and an operator supplies it through the `BLOCKED_TAGS` envar.
3. A client with unmet requirements never fails startup and never panics. It is inactive and contributes nothing.
   Naming it alone yields an empty result with the reason.
4. Reciprocal rank fusion is the combined ranking for `popular`.
5. Every tool takes a client list per call, defaulting to `rule34,danbooru,gelbooru`. Similar tags with different names
   are never combined. Same-name tags across clients are deduplicated in `tags` by keeping the higher within-query
   percentage, and fused in `popular`.
6. Images are always returned as permalinks. No inline bytes, base64, or MCP image content.
7. Stale non-popular tags are served stale rather than re-fetched.
8. `get` keeps its name.
9. `related` is in scope for this plan. `suggest` was considered and dropped as unnecessary.
10. Tag popularity is compared approximately, by a within-query percentage rather than a raw count: for each client,
    that client's highest count in its own result set is 100%, and every other tag is `count / highest * 100`, rounded
    to a whole number. `tags` sorts and deduplicates on that percentage and still shows the raw count, `popular` shows
    it per client alongside the reciprocal rank fusion score, and raw counts are never summed across clients. Close
    enough is the deliberate standard.
11. `get` defaults to `danbooru` when no client is named. This is standard for the domain and does not depend on which
    clients are active.
12. `related`'s schema restricts `clients` to an enum of the clients that implement a related-tag API, which today is
    `danbooru`, `aibooru`, `e621`, and `e926`. Its default is that set intersected with the default client list, which
    is `danbooru` and `e621`. Its description, schema, and output all state that it is a partial-coverage tool, so a
    short or empty result is expected rather than a failure.
13. `search` results stay grouped in the requested client order and are not interleaved. Post ordering is each client's
    own relevance or recency order, not a comparable popularity score, so the percentage normalization does not apply
    to it.
14. The default client list degrading to `danbooru` alone when no credentials are configured is the intended
    behaviour.

## Proposed additions

Offered as riffs, not commitments. Each is cheap to add later because the pieces above already exist.

| Addition                       | Shape                                                                            | Why it serves image generation                                                                  |
| ------------------------------ | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Pooling and popularity windows | Moebooru `/post/popular_recent.json` and Danbooru's rank-based orderings          | A recency-weighted view of popularity that is not the same as all-time counts.                    |
| Hashbooru sibling coverage     | Additional sites in the existing family table, no new adapter                     | The family already covers the protocol, so coverage grows at near-zero cost.                      |
| Offline tag cache snapshot     | Optional JSON export and import of `tags`                                          | Lets a deployment seed the cache without a cold, 1 rps warm-up burst across a dozen clients.      |
| Parent and child post chains   | Danbooru `/posts/<id>.json` parent fields, surfaced as related posts              | Lets a model follow a chain of edits without a second search.                                     |
| `sankaku` client               | Only if a token-based API becomes documented                                       | It is popular, but it is excluded today because it would mean scraping or reverse engineering.    |

## Architecture impact

Update the mermaid diagram in `AGENTS.md`:

- add `catalog`, `booru`, `present`, `danbooru`, `gelbooru`, `moebooru`, `e621`, `philomena`, `fetch`, and `store`;
- `booru` holds the canonical types and the provider contract, and is imported by every provider and by `catalog`;
- `catalog` owns orchestration and the cache policy, and depends on `booru` and `store`, never on a concrete provider;
- providers depend on `booru` and `fetch`; `fetch` depends on `utils`; `store` depends on `utils`;
- `mcp` depends on `catalog`, `booru`, and `present`;
- `server` constructs the concrete providers and injects them into `catalog`, keeping `catalog` provider-agnostic.

`booru.Provider` stays small. Related tags are an optional interface (`RelatedTagProvider`) that `catalog`
type-asserts, so a family that cannot serve them does not implement a dead method. The
rule that only `internal/server` may import `internal/mcp` is unchanged, and no new package imports the MCP SDK except
`internal/mcp`.

## Implementation units

Units 5 to 9 are independent and may land in any order. Unit 1 is a prerequisite for all providers. Units 15 and 16
are additive and can land last.

### Unit 1: shared outbound transport and global limiter

Scope: `internal/fetch`, one HTTP path every provider uses. No booru knowledge.

Status: Done. `internal/fetch/fetch.go`, `internal/fetch/limiter.go`, and `internal/fetch/fetch_test.go`.

Deliverables:

- `internal/fetch/fetch.go`: `Limiter` interface (`Wait(context.Context) error`); `Config` carrying `BaseURL`,
  `UserAgent`, `Timeout`, `Limiter`, and `VerboseErrors`; `New(Config) (*Client, error)` rejecting a relative or
  non-http base URL and a nil limiter; `GetJSON[T any](ctx, label, path string, query url.Values) (T, error)`.
- `internal/fetch/limiter.go`: `NewLimiter(rps float64, burst int) *rate.Limiter`.
- Retry-on-`429`/`5xx` with `Retry-After` support, a bounded body snippet in errors, and verbose bodies when
  `ERROR_DETAIL` is `verbose`.
- `internal/fetch/fetch_test.go`: table-driven tests with `httptest`.

Acceptance criteria:

- [x] Every request carries the configured `User-Agent` and an `Accept: application/json` header.
- [x] `Wait` is called exactly once per attempt, including retries, proven with a counting fake limiter.
- [x] A `429` with `Retry-After: 0` is retried and the eventual `200` is returned.
- [x] Four consecutive `503`s produce one error naming the client, path, and final status; the body snippet is present
      and bounded to the shared limit.
- [x] A context deadline aborts the call and the returned error wraps `context.DeadlineExceeded`.
- [x] `New` rejects `example.com/path`, `file:///x`, and a nil limiter, and accepts an absolute `https` base.
- [x] `go test ./internal/fetch/... -race -count=1` passes, with no test relying on wall-clock sleeps beyond the
      retry backoff.

### Unit 2: canonical domain model and provider contract

Scope: `internal/booru`, types only. No HTTP, no cache.

Status: Done. `internal/booru/tag.go`, `internal/booru/post.go`, `internal/booru/provider.go`, and `internal/booru/booru_test.go`.

Deliverables:

- `internal/booru/tag.go`: `TagCategory` and `Rating` enums with `Parse` and `String`; `Tag{Name, Category, Count}`;
  `FusedTag{Name, Category, Score, Clients []TagCount}`; `TagCount{Client string, Count, Rank, Pct int}`;
  `RelatedTag{Tag, Client string, Score, Rank int}`; `NormalizeTag(string) string`;
  `Percent(count, max int) int` returning `round(count / max * 100)` and `0` for a non-positive max.
- `internal/booru/post.go`: `Post` with `Client`, `ID`, `URL`, `FileURL`, `PreviewURL`, `SampleURL`, `Width`, `Height`,
  `Rating`, `Score`, `FavCount`, `Source`, `Tags`, and `CreatedAt`.
- `internal/booru/provider.go`: the `Provider` interface (`Name`, `Capabilities`, `Search`, `Post`, `SearchTags`,
  `PopularTags`) with its request structs (`SearchParams`, `TagQuery`, `PopularQuery`, `RelatedQuery`), the optional
  `RelatedTagProvider` interface, `Capabilities{Rating, Random bool}`, and a `Registry` of
  `Entry{Name, Provider, Active, Reason}` with `Get`, `Active`, `All`, and `Names`.
- `internal/booru/booru_test.go`.

Acceptance criteria:

- [x] `NormalizeTag` lowercases, trims, collapses runs of whitespace to a single `_`, and leaves existing underscores
      alone.
- [x] `ParseTagCategory` and `ParseRating` accept the canonical spellings and reject anything else with an error.
- [x] `Registry.Get` returns an error naming the unknown client; registration rejects an empty or duplicate name.
- [x] `Registry.Active` omits inactive entries, and an inactive entry's `Reason` is non-empty.
- [x] A provider that does not implement `RelatedTagProvider` still satisfies `Provider`.
- [x] `Percent(50, 200)` is `25`, `Percent(200, 200)` is `100`, and a zero or negative max yields `0` rather than a
      panic or a division by zero.
- [x] `go test ./internal/booru/... -race -count=1` passes.

### Unit 3: configuration

Scope: `internal/config` only. Every new envar, its default, and its validation.

Status: Done. `internal/config/config.go` and `internal/config/config_test.go`. `BLOCKED_TAGS` ships no default
content, and tests use neutral placeholder words only.

Deliverables:

- A declarative client table: name, family, default URL, URL override envar, credential envars, credential requirement,
  and whether the client is in the default set.
- Global envars from the Configuration table, including `DEFAULT_CLIENTS` and `BLOCKED_TAGS`.
- Helpers: `EnabledClients() []string`, `DefaultClients() []string`, `BlockedTags() []string` parsing and normalizing
  the envar, `CacheTTL() time.Duration`, `ContentRating() (booru.Rating, error)`,
  `ClientActive(name string) (bool, string)`.
- `Config.Validate()` enforcing the certainly-wrong rules, called by `server.New`.
- `internal/config/config_test.go`.

Acceptance criteria:

- [x] `BOORU_CLIENTS=all` yields every client, including `aibooru` and `e926`; the default yields every non-mirror
      client and excludes those two.
- [x] `DEFAULT_CLIENTS` defaults to `rule34,danbooru,gelbooru`; an unknown name in it or in `BOORU_CLIENTS` fails with
      an error naming the offending name.
- [x] `RATE_LIMIT_RPS=0` and a negative value each fail validation with an error naming `RATE_LIMIT_RPS`.
- [x] `CONTENT_RATING=nsfw` fails with an error naming `CONTENT_RATING` and listing the accepted values; `all` is
      accepted and is the default.
- [x] `CACHE_TTL_DAYS=30` yields a 720 hour duration; `0` yields zero, meaning caching is disabled.
- [x] `BLOCKED_TAGS` splits on commas, trims, drops blanks, and normalizes each tag; an unset value yields an empty
      list, and no file is read for it.
- [x] No tag name appears anywhere under `docs/`, `.env.example`, or test fixtures as a shipped default denylist.
- [x] A client missing its credentials is inactive with a reason naming the missing envars, and never causes an error.
- [x] `e621` and `e926` are inactive with a reason naming `USER_AGENT` while it holds its default value.
- [x] `ClientActive` never returns a reason containing a credential value.
- [x] `go test ./internal/config/... -race -count=1` passes.

### Unit 4: SQLite tag cache

Scope: `internal/store`, persistence only. No policy, no HTTP.

Status: Done. `internal/store/store.go`, `tags.go`, `popular.go`, `related.go`, and one test file each. One deviation:
`related.go` exposes `RelatedFetchedAt` instead of `IsRelatedFresh`, so the caller injects its own clock and catalog
tests never sleep.

Deliverables:

- `internal/store/store.go`: `New(path) (*Client, error)` opening SQLite in WAL mode and creating the schema if absent;
  `Close()`.
- `internal/store/tags.go`: `UpsertTags(ctx, client string, tags []booru.Tag, at time.Time) error`;
  `TagsByClient(ctx, client, names []string) ([]CachedTag, error)`;
  `SearchTags(ctx, query TagFilter) ([]CachedTag, error)`, where `TagFilter` carries substring, category, client, and
  limit.
- `internal/store/popular.go`: `ReplacePopular(ctx, client string, tags []booru.Tag, at time.Time) error`;
  `Popular(ctx, client string) ([]CachedTag, error)`; `IsPopular(ctx, client, name string) (bool, error)`.
- `internal/store/related.go`: `ReplaceRelated(ctx, client, name string, tags []booru.RelatedTag, at time.Time) error`;
  `Related(ctx, client, name string) ([]booru.RelatedTag, error)`;
  `RelatedFetchedAt(ctx, client, name string) (time.Time, bool, error)`.
- `CachedTag` exposes `Tag`, `FetchedAt`, and `IsPopular`.
- `internal/store/store_test.go`, `internal/store/tags_test.go`, `internal/store/popular_test.go`,
  `internal/store/related_test.go`.

Acceptance criteria:

- [x] A tag round-trips with its category, count, and `fetched_at`.
- [x] Upserting the same `(client, name)` replaces count and `fetched_at` instead of inserting a duplicate.
- [x] `SearchTags` orders by count descending and honours limit and category filters.
- [x] `ReplacePopular` replaces one client's snapshot and leaves other clients' rows untouched.
- [x] `IsPopular` is true only for a tag in the current snapshot, and false after the snapshot is replaced without it.
- [x] `Related` round-trips score and rank, and `ReplaceRelated` replaces one `(client, name)` set without touching
      others.
- [x] `CACHE_TTL_DAYS` is never consulted here; store always returns `fetched_at` and lets the caller decide.
- [x] The schema is created on a fresh temp path and an existing file reopens with its data intact.
- [x] `go test ./internal/store/... -race -count=1` passes and tests use `t.TempDir()`.

### Unit 5: Danbooru-family provider

Scope: `internal/danbooru`, registering `danbooru` and `aibooru`.

Status: Done. `internal/danbooru/client.go`, `mapping.go`, and `client_test.go`. The provider is name-parameterized;
registration of both names is asserted in Unit 14.

Deliverables:

- `internal/danbooru/client.go`: `New(Config) *Client` taking base URL, login, API key, and an `*fetch.Client`;
  `Name() string`; `Capabilities()` reporting rating and random support.
- `Search` (`/posts.json`), `Post` (`/posts/<id>.json`), `SearchTags` (`/tags.json` with `search[name_matches]`,
  `search[order]=count`, `search[category]`), `PopularTags` (`/tags.json` with `search[order]=count`), and
  `RelatedTags` (`/related_tag.json`) implementing the optional interface.
- Mapping from `tag_string_*` fields into grouped canonical tags, and `rating` into `booru.Rating`; permalink
  `<base>/posts/<id>`.
- `login` and `api_key` query parameters added only when both are set.
- `internal/danbooru/client_test.go` with `httptest` fixtures.

Acceptance criteria:

- [x] `Search` sends `tags`, `limit`, and `page`, and adds `rating:` and `-` negations from the policy inputs.
- [x] `Search` never requests more than `MAX_LIMIT` per page.
- [x] `SearchTags` matches on substring and requests count ordering, mapping category numbers to `TagCategory`.
- [x] `PopularTags` preserves counts and returns them ordered as the API does.
- [x] `RelatedTags` maps the relatedness score and rank, ordered as the API returns.
- [x] `Post` maps `g/s/q/e` to the canonical ratings and produces `<base>/posts/<id>`.
- [x] `api_key` appears in the query only when `login` and key are both configured, and never appears in logs.
- [x] The provider is name-parameterized, so `danbooru` and `aibooru` reuse it; Unit 14 asserts both registrations.
- [x] A malformed upstream body produces an error naming the client and path, not a panic.
- [x] `go test ./internal/danbooru/... -race -count=1` passes.

### Unit 6: Gelbooru-family provider

Scope: `internal/gelbooru`, registering `gelbooru`, `rule34`, `realbooru`, `xbooru`, `tbib`, and `safebooru`.

Status: Done. `internal/gelbooru/client.go`, `mapping.go`, and `client_test.go`. Amendment: a Gelbooru post carries one
space-separated tag string with no categories, so post tags are recorded as `general`; category-accurate results come
from the tag search and popular endpoints. The live-credential-requirement check for `xbooru`, `tbib`, and `safebooru`
is recorded in `.env.example` in Unit 14, since it cannot be done without operator accounts.

Deliverables:

- `internal/gelbooru/client.go` implementing the interface over API v2
  (`index.php?page=dapi&s=post|tag&q=index&json=1`), sending credentials only when configured.
- Tag counts from `s=tag&q=index&orderby=count`; post searches with `pid` for paging. No related-tag support.
- Post permalink `index.php?page=post&s=view&id=<id>`.
- Credential requirements verified against live documentation for `xbooru`, `tbib`, and `safebooru`, and the outcome
  recorded in `.env.example`.
- `internal/gelbooru/client_test.go`.

Acceptance criteria:

- [x] The provider is name-parameterized, so all six sites reuse it with their own credentials; Unit 14 asserts the
      registrations.
- [x] Every request for a credential-configured client carries both `api_key` and `user_id`, asserted on the received
      request.
- [x] A keyless `safebooru` sends neither parameter.
- [x] A credential-required client without credentials is inactive via `Config.ClientActive`, covered by Unit 3, and so
      makes no request in a combined call.
- [x] The post `tags` field, one space-separated string, is split into canonical `general` tags; tag search and popular
      results carry their real categories, mapped from the Hashbooru type numbers.
- [x] Rating strings map to canonical ratings.
- [x] `pid` is `page - 1`, and `limit` is capped at `MAX_LIMIT`.
- [x] A `401` from upstream surfaces as an error naming the client and its credential envars, without echoing the key.
- [x] The live-credential check for `xbooru`, `tbib`, and `safebooru` is deferred to Unit 14 and recorded in
      `.env.example`.
- [x] `go test ./internal/gelbooru/... -race -count=1` passes.

### Unit 7: Moebooru-family provider

Scope: `internal/moebooru`, registering `yandere`, `konachan`, and `sakugabooru`.

Status: Done. `internal/moebooru/client.go`, `mapping.go`, and `client_test.go`. Post `tags` are one uncategorized
space-separated string, so post tags are `general`; tag search and popular carry real categories.

Deliverables:

- `internal/moebooru/client.go` implementing the interface over `/post.json`, `/tag.json`, and count ordering.
- Rating mapping from `s/q/e`; Moebooru tag type numbers mapped to canonical categories, with the non-obvious types
  documented at the mapping site. No related-tag support.
- `internal/moebooru/client_test.go`.

Acceptance criteria:

- [x] The provider is name-parameterized, so all three sites reuse it; Unit 14 asserts the registrations.
- [x] `SearchTags` requests count ordering and maps Moebooru tag types to canonical categories, with studio and circle
      folding into artist and copyright.
- [x] `Search` sends the tag string, limit, and page.
- [x] `Post` maps ratings and produces a permalink of `<base>/post/show/<id>`.
- [x] A response with an unexpected tag type maps to `general` rather than failing the call.
- [x] `go test ./internal/moebooru/... -race -count=1` passes.

### Unit 8: e621-family provider

Scope: `internal/e621`, registering `e621` and `e926`.

Status: Done. `internal/e621/client.go`, `mapping.go`, and `client_test.go`. `fetch.GetJSONWithHeaders` was added so the
provider can send HTTP basic credentials without putting them in the URL.

Deliverables:

- `internal/e621/client.go` implementing the interface over `/posts.json`, `/tags.json`, and `/related_tag.json`, using
  the shared `USER_AGENT` and optional HTTP basic credentials.
- Tag counts from `/tags.json` with count ordering; `s/q/e` rating mapping; related tags as an optional interface.
- `internal/e621/client_test.go`.

Acceptance criteria:

- [x] The provider is name-parameterized, so both sites reuse it; Unit 14 asserts the registrations.
- [x] Every request carries the configured `USER_AGENT`, asserted on the received request.
- [x] With login and key configured, credentials are sent as HTTP basic auth and never appear in a URL or a log line.
- [x] Rating `s/q/e` maps to canonical ratings, including e621's `s` meaning `general`.
- [x] `Search` sends the tag string, limit, and page, and adds the rating and negation terms.
- [x] `RelatedTags` maps the relatedness score and rank, and tolerates both the object and array response shapes.
- [x] An upstream `403` produced by a missing User-Agent surfaces an error that names `USER_AGENT` as the likely cause.
- [x] `go test ./internal/e621/... -race -count=1` passes.

### Unit 9: Philomena-family provider

Scope: `internal/philomena`, registering `derpibooru`, `twibooru`, and `furbooru`.

Status: Done. `internal/philomena/client.go`, `mapping.go`, and `client_test.go`. The provider is name-parameterized
and its API key is sent only when configured; registration and inactivity are covered by Units 3 and 14.

Deliverables:

- `internal/philomena/client.go` implementing the interface over `/api/v1/json/search/posts?q=` and
  `/api/v1/json/search/tags?q=`, sending the API key as the `key` query parameter.
- Tag counts mapped from Philomena's `images` count onto `Tag.Count`.
- Tag type mapping for origin, species, and oc into canonical categories.
- `Capabilities()` reporting no rating support and no related-tag support.
- `internal/philomena/client_test.go`.

Acceptance criteria:

- [x] The provider is name-parameterized, so all three sites reuse it with their own API keys; Unit 14 asserts the
      registrations.
- [x] The key is sent only when configured, and never appears in a log line.
- [x] Tag `images` counts map to `Tag.Count`, and origin, species, and oc map to `copyright`, `character`, and `meta`.
- [x] `Capabilities().Rating` is false, and a rating-constrained call excludes these clients rather than querying them,
      as implemented by the catalog policy in Unit 10.
- [x] `Search` sends the tag string and limit.
- [x] `Post` maps a permalink of `<base>/images/<id>`.
- [x] An unconfigured API key leaves the client inactive via `Config.ClientActive`, covered by Unit 3.
- [x] `go test ./internal/philomena/... -race -count=1` passes.

### Unit 10: catalog post search and get

Scope: `internal/catalog`, the uncached half: `search` and `get`, plus the content policy both share.

Status: Done. `catalog.go`, `clients.go`, `policy.go`, `search.go`, `get.go`, and `catalog_test.go`.

Deliverables:

- `internal/catalog/catalog.go`: `New(registry *booru.Registry, store *store.Client, cfg Options) *Service`;
  `Options` carrying default clients, cache TTL, content rating, blocked tags, and `MAX_LIMIT`.
- `internal/catalog/clients.go`: resolving a requested `clients` list into the active clients to query, preserving
  request order, skipping inactive ones, and reporting unknown names as errors and named-but-inactive ones as reasons.
- `internal/catalog/search.go`: `Search(ctx, SearchParams) ([]booru.Post, error)`, capping the limit, applying policy,
  and grouping results in the requested client order.
- `internal/catalog/get.go`: `Get(ctx, client, id string) (booru.Post, error)` resolving exactly one client,
  defaulting to `danbooru` when `client` is empty.
- `internal/catalog/policy.go`: `RatingFor(call)`, `Exclusions()`, `FilterPosts`, and the exclusion of clients whose
  `Capabilities().Rating` is false when a rating cap is narrower than `explicit`.
- `internal/catalog/catalog_test.go` with fake providers.

Acceptance criteria:

- [x] An unknown client in `clients` errors with a message naming it; an unset list uses `DEFAULT_CLIENTS`.
- [x] An inactive client is skipped in a combined call with no error and no provider call, and naming only inactive
      clients yields an empty result with the reasons.
- [x] Results are grouped in the requested client order, and a reversed list reverses the groups.
- [x] A `limit` above `MAX_LIMIT` is capped before the provider is called, asserted on the fake's received params.
- [x] A per-call `rating` wider than `CONTENT_RATING` is narrowed, not honoured.
- [x] A client with `Capabilities().Rating` false is excluded, not queried, when a rating cap is narrower than
      `explicit`.
- [x] `BLOCKED_TAGS` appear as negated terms in the provider request and are removed from returned posts.
- [x] `get` returns a post with a permalink and file, sample, and preview URLs; no code path fetches image bytes, so
      that is structural rather than merely tested.
- [x] `get` uses the named client, defaults to `danbooru` when none is named, and errors clearly when the id is not
      found.
- [x] `go test ./internal/catalog/... -race -count=1` passes.

### Unit 11: catalog tag search and cache policy

Scope: `internal/catalog`, the cached half for `tags`. This is the unit that implements the refresh rules.

Status: Done. `tags.go` and `tags_test.go`; `fuse.go` provides the percentage merge.

Deliverables:

- `internal/catalog/tags.go`: `Tags(ctx, TagQuery) ([]FusedTag, error)` implementing rules 1 to 5 from the Caching
  section: fresh hit, miss, stale-and-popular refresh, stale-and-not-popular serve, explicit refresh.
- Cross-client deduplication of the same name on the within-query percentage, with the surviving client named and its
  raw count kept.
- `internal/catalog/cache.go`: `Now func() time.Time` (injectable for tests) and the freshness predicate.
- `internal/catalog/tags_test.go` with fake providers that count calls.

Acceptance criteria:

- [x] A fresh cache entry is served with zero provider calls.
- [x] An unseen tag causes one provider call per targeted client and is stored with a `fetched_at`.
- [x] A stale entry present in `popular_tags` causes a re-fetch; the same entry after removal from the snapshot does
      not, and the stale value is returned.
- [x] `refresh: true` forces a re-fetch even for a fresh, non-popular entry.
- [x] `CACHE_TTL_DAYS=0` causes every read to call the provider and writes nothing.
- [x] Percentages are relative to the query: a client's top tag is `100`, and a tag at half that client's top count is
      `50`.
- [x] A name present on two clients survives once, on the client where its percentage is higher, with that client's raw
      count shown.
- [x] A name at 90% of a small client's top count beats the same name at 60% of a large client's top count, proving the
      comparison uses percentages and not raw counts.
- [x] Similar tags with different names both survive.
- [x] `go test ./internal/catalog/... -race -count=1` passes and no test sleeps for the TTL.

### Unit 12: popular aggregation

Scope: `internal/catalog`, the `popular` tool and the fusion that `tags` also uses.

Status: Done. `popular.go`, `fuse.go`, `popular_test.go`; `mergeTagSets` was added alongside `Fuse` for the percentage
merge `tags` needs.

Deliverables:

- `internal/catalog/popular.go`: `Popular(ctx, PopularQuery) (PopularResult, error)`; `PopularResult` carrying the
  fused tags, the per-client snapshots, the skipped clients and reasons, and `Warnings []string`.
- `internal/catalog/fuse.go`: `Fuse(perClient map[string][]booru.Tag) []booru.FusedTag` implementing reciprocal rank
  fusion with k=60, deterministic tie-breaking, and the `min(MAX_LIMIT, limit*4)` fan-out.
- Per-client upsert into `tags` and `popular_tags` with a shared timestamp so TTLs move together.
- Per-client percentages computed with `booru.Percent` over each client's own returned set, carried on every
  `TagCount` in the fused output.
- Partial failure handling: an error from one client yields a warning and the remaining clients' results.
- `internal/catalog/popular_test.go`, `internal/catalog/fuse_test.go`.

Acceptance criteria:

- [x] A fresh snapshot within the TTL is served with zero provider calls.
- [x] A stale snapshot refetches from every active requested client.
- [x] Inactive clients are absent from the refetch, listed with their reasons, and never fail the call.
- [x] `Fuse` ranks a tag that is top-3 on two clients above a tag that is top-1 on one client, given equal inputs
      beyond that.
- [x] Fused tags carry per-client counts, ranks, and percentages for every client that returned the tag.
- [x] A provider error produces a warning naming the client and does not fail the call when another client succeeds.
- [x] All clients failing returns an error, not an empty success.
- [x] The returned list is capped at `limit`, and `limit` is capped at `MAX_LIMIT`.
- [x] `go test ./internal/catalog/... -race -count=1` passes.

### Unit 13: MCP tools and presentation

Scope: `internal/mcp` and `internal/present`.

Status: Done. `shared.go`, `handlers.go`, `server.go`, one file per tool, `server_test.go`, `tools_test.go`, and
`internal/present/{tags,post}.go` with `present_test.go`.

Deliverables:

- `internal/present/tags.go`, `related.go`, and `post.go`: markdown tables and blocks for fused tags, tag search
  results, related tags, post lists, and a single post, with tags grouped by category; tag tables show the raw count and
  the within-query percentage.
- `internal/mcp/handlers.go` and `shared.go`: the handler struct carrying the catalog service and logger, matching the
  reference project's shape, plus the shared `clients` array input field.
- `internal/mcp/popular.go`, `tags.go`, `related.go`, `search.go`, `get.go`: one file per tool, with
  JSON-schema inputs built by `jsonschema.For`, defaults and enums set explicitly (`clients`, `limit`, `category`,
  `rating`, `page`, `random`, `refresh`), and structured outputs.
- `internal/mcp/server.go`: `Deps` carrying the catalog service and options, keeping the existing `New` signature
  change minimal; all five tools register unconditionally since the catalog exists whenever the server does.
- `internal/mcp/*_test.go` using the session helper pattern from the reference project; `internal/present/*_test.go`.

Acceptance criteria:

- [x] All five tools appear in the server's tool list, with names `popular`, `tags`, `related`, `search`, and `get`.
- [x] Every tool except `get` exposes an optional `clients` array; `get` exposes an optional `client` only, documented
      as defaulting to `danbooru`.
- [x] `tags` and `popular` schemas carry the category enum and the documented defaults. The original wording also named
      `related`, which has no category field; corrected here.
- [x] No tool exposes an `image`, `bytes`, or base64 input, and no output contains inline image content.
- [x] Calling `tags` returns text containing each tag's work count and percentage, with structured output matching the
      text.
- [x] The `related` tool's description states that related tags are a partial-coverage capability, that any client
      without a related-tag API is skipped and reported, and that a short or empty result is expected rather than a
      failure.
- [x] `related`'s `clients` schema is an enum of the related-capable clients only, and its default is that set
      intersected with the default client list.
- [x] Calling `get` returns permalink and file URLs as text and makes no image request.
- [x] Every handler failure is prefixed with the tool name, as the reference project does.
- [x] No API key or credential appears in any log line emitted during a handler test; handlers log only tool inputs,
      and the startup logging path is covered by Unit 14.
- [x] `go test ./internal/mcp/... ./internal/present/... -race -count=1` passes.

### Unit 14: wiring, docs, and deployment

Scope: `internal/server`, `cmd/server`, and repository documentation.

Status: Done. `internal/server/server.go` and `server_test.go`, plus `.env.example`, `README.md`, `Dockerfile`,
`AGENTS.md`, and `.gitignore`.

Deliverables:

- `internal/server/server.go`: build the limiter, one `fetch` transport per client, the providers, the store, the
  registry, the catalog, and the MCP server; call `Config.Validate()` and fail only on certainly-wrong config; log the
  active clients, the inactive clients with reasons, the cache path, the TTL, and the effective rate, without
  credentials.
- `internal/server/server_test.go`: startup validation, per-client registration, inactive-client logging, and a runtime
  smoke test through `httptest` against fake upstreams for two families at once.
- `cmd/server/main.go`: unchanged in shape; `Shutdown` also closes the store.
- `.env.example`: every global envar and every client block, grouped by family, with the credential requirements
  confirmed in the provider units, and no tag denylist content.
- `README.md`: a short Features section covering the five tools, the client list by family, the cache and its TTL, the
  global rate limit, and the NSFW-forward content policy; keep the existing Usage and Authentication sections.
- `Dockerfile`: recreate `/data` owned by the runtime user, and document the `DB_PATH` volume in the README.
- `AGENTS.md`: the updated architecture diagram from Architecture impact.
- `.gitignore`: add `*.db-wal` and `*.db-shm`.

Acceptance criteria:

- [x] `BOORU_CLIENTS=all` registers every client and marks the unmet ones inactive with a distinct reason each.
- [x] Every certainly-wrong config from Unit 3 fails `server.New` with an error naming the envar.
- [x] No client credential causes a startup failure, and no startup log line contains a credential.
- [x] With every client inactive, the server starts, logs one warning, and the tools return empty results with reasons.
- [x] The SQLite file is created at `DB_PATH`; durability across a close and reopen is covered by the store tests.
- [x] A runtime smoke test with fake upstreams for `danbooru` and `rule34` returns a combined `popular` result through
      the MCP handler.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.
- [x] README and `.env.example` name every client and every global envar from the Configuration table.

### Unit 15: related tags

Scope: the `related` tool end to end, over the optional provider interface.

Status: Done. `internal/catalog/related.go`, `related_test.go`, `internal/mcp/related.go`.

Deliverables:

- `internal/catalog/related.go`: `Related(ctx, RelatedQuery) ([]booru.RelatedTag, error)` querying only providers that
  implement `RelatedTagProvider`, merging per client, caching in `related_tags` under the same TTL and the same
  popular-refresh rule, and skipping the rest with a reason.
- `internal/mcp/related.go` and its schema: `tag` required, `limit`, `refresh`, and a `clients` field whose enum is
  built from the registry's related-capable clients and whose default is that set intersected with the default client
  list; the tool `Description` and the `clients` field description both state that only those clients can answer, that
  any other client that reaches the handler is skipped and named with a reason, and that a short or empty result is
  expected rather than an error.
- `internal/present/related.go`: names every skipped client and its reason, and states in the text that an empty result
  is expected, so a partial result never reads as a failure.
- `internal/catalog/related_test.go`, `internal/mcp/related_test.go`.

Acceptance criteria:

- [x] A client that does not implement `RelatedTagProvider` is skipped with a reason and never queried.
- [x] With none of the requested clients able to answer, the result is empty and the reason is present.
- [x] The registered `related` tool's description states that only some clients support related tags, that the rest are
      skipped and reported, and that an empty result is expected rather than a failure.
- [x] The `clients` schema enum contains exactly the clients implementing `RelatedTagProvider`, and no others.
- [x] The default `clients` is those clients intersected with the default client list, which is `danbooru` and `e621`
      with the stock configuration.
- [x] A `clients` value outside the enum is rejected by schema validation before any provider call is made.
- [x] A partial result and a wholly empty result both return a successful tool call, with the skipped clients and their
      reasons in the text output, so neither reads as a broken tool.
- [x] A fresh cached result serves with zero provider calls; a stale one re-fetches per the popular-refresh rule.
- [x] `danbooru` and `e621` both contribute, and their rows are kept separate rather than fused.
- [x] The result is capped at `limit`, and `limit` is capped at `MAX_LIMIT`.
- [x] `go test ./internal/catalog/... ./internal/mcp/... -race -count=1` passes.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`.
- Human, live upstreams, keyless clients: run with `BOORU_CLIENTS=danbooru,yandere,safebooru,sakugabooru` and the
  default `RATE_LIMIT_RPS=1`, call `popular`, and confirm it takes roughly the number of clients in seconds and that
  the combined list looks plausible.
- Human, live upstreams, credentialed clients: configure `gelbooru`, `rule34`, `realbooru`, `e621`, and `derpibooru`,
  call `popular`, and confirm each appears in the per-client breakdown. This is the check that the credential and
  User-Agent handling is real, not just fixture-shaped.
- Human, inactive clients: remove the `RULE34_*` credentials and confirm `rule34` contributes nothing to a combined
  `popular` and returns a clear reason when named alone, with no startup failure.
- Human, live upstreams: call `tags` for a common tag and an obscure one, restart the server, and repeat; confirm the
  second run is served from cache with no upstream delay.
- Human, live upstreams: set `CACHE_TTL_DAYS=0`, repeat a `tags` call, and confirm it goes upstream every time.
- Human, live upstreams: call `search` and `get` on a real post from a keyless client and a credentialed one, and
  confirm the permalink opens, the tags match the site, and no image bytes are returned.
- Human, live upstreams: call `tags` across several clients and confirm the strongest match per client shows 100%, that
  the list is ordered by percentage, and that a name duplicated across clients appears once with the higher-percentage
  client named.
- Human, live upstreams: call `related` with `clients` set to a client outside the enum and confirm the call is rejected
  by schema validation, then with `danbooru` and `e621` and confirm both contribute.
- Human, NSFW: with the default `CONTENT_RATING=all`, confirm `search` returns explicit-rated results where the client
  supports ratings, and with `CONTENT_RATING=general` confirm they disappear and the Philomena clients drop out.
- Human, e621: confirm a request without `USER_AGENT` fails and one with contact information succeeds.
- Human, blocked tags: set `BLOCKED_TAGS` to a tag that appears in a `tags` result and confirm it is absent from the
  output and present as a negation in the upstream query.

## Risks and follow-ups

- Upstream APIs change and credential policies drift fastest of all. Each provider has fixture-based tests, but
  fixtures do not catch a live schema or policy change, so the human live checks are the only early warning.
- Every credentialed client adds a signup, a key rotation, and a place for a leak. Keys are per site, never logged, and
  never placed in a URL except where the site itself requires it (`gelbooru` family and Philomena).
- A global 1 rps limiter is deliberately slow, and a cold `popular` across the default active client set is a burst of
  a dozen plus requests that takes that many seconds. It is paid once per TTL, and `RATE_LIMIT_RPS` and per-call
  `clients` are the tuning knobs. Operators raising the rate own the consequences with upstreams.
- Mirrors double count. `e926` mirrors `e621`, and `konachan.net` mirrors `konachan.com`, so combining a pair inflates
  the fused score for shared tags. Mirrors are excluded from the default set for exactly this reason.
- Philomena clients cannot answer rating queries. The exclusion path is deliberate, and the cost is that
  `CONTENT_RATING` below `explicit` removes three clients from every combined result.
- `related` has partial client coverage by nature. Only the Danbooru and e621 families publish a related-tag API, so a
  caller expecting every client will sometimes see fewer results and sometimes none. The schema enum, the description,
  and the output all say so explicitly, because a silent partial result would read as a bug or as "this tag has no
  relatives".
- Same-name deduplication in `tags` compares within-query percentages, which is a deliberate approximation rather than
  a true cross-corpus measure. A small client's standout can outrank the same tag on a large client, and that is
  accepted.
- The SQLite cache is a single writable file. WAL mode is set, but two containers sharing one `DB_PATH` is unsupported.
- Long-tail tag counts can be stale indefinitely by design. If a consumer needs fresh counts for a specific tag,
  `refresh: true` is the supported escape.
- Blocked tags are filtered twice because query-side negation is not universal; a future client must implement both
  paths or the filter is one-sided.
