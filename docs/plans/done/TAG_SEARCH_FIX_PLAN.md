# Canonical tag search: exact mode, relations, and honest results

## Goal

Make the `tags` tool trustworthy. Today it is a live substring search over Danbooru's canonical tag table, which is
already the right source, but the query semantics are wrong: substring matches surface unusable zero-count rows, there
is no exact mode, aliases and implications are invisible, categories cannot be filtered, and an empty page is presented
as a definitive "no such tag". This plan fixes the data policy, the query surface, and the wording.

## Root cause finding (recorded before any change)

The hypothesis in the brief was that the index is built from post-level tag strings with the delimiter dropped, which
would explain compound entries like `blue_hairkneehighs` and `ass_grabeye_contact`. That hypothesis is **refuted**.

- The only data source is `GET https://danbooru.donmai.us/tags.json` with `search[name_matches]=*QUERY*`,
  `search[order]=count` and `search[name]=NAME` (`internal/booru/client.go`). There is no local index, no dump, no
  scraper, and no post-tag ingestion anywhere in the tree.
- The compound strings are genuine rows in Danbooru's **canonical** tag table. Verified live on 2026-09-24:
    - `tags.json?search[name_matches]=*ass_grabeye_contact*` returns one row, `ass_grabeye_contact`, `post_count` 0.
    - `tags.json?search[name_matches]=*zzzzzzzzzz*` returns a 200-character `z`-string row with `post_count` 0.
      Danbooru's tag table is user-editable, so punctuation-mangled, concatenated, 0-post rows are part of the canonical
      data and no choice of source removes them.

The real defects are therefore **query semantics and result policy**, not ingestion:

1. Substring matching has no notion of a real tag, so zero-count canonical junk outranks nothing and is returned as if
   it were meaningful, and it can push a real tag off a page.
2. There is no exact existence check.
3. `tag_aliases.json` and `tag_implications.json` are never consulted, so `dickgirl` reads as a bare `0 works` and
   `futanari_pov` hides its `futanari`/`pov` implications.
4. `search[category]` is available upstream but unused.
5. An empty match list is indistinguishable from an unanswerable query.
6. Ordering has no tie-break, so equal-count rows have no stable page order.

What we changed it to: keep the canonical live source (it is strictly fresher than any dump), make zero-count rows
non-results in substring mode, and add exact, alias, implication, category, and status semantics on top of it. The only
new ingested dataset is the **implication graph** (`tag_implications.json`), which is small enough (~40k active rows) to
cache and is genuinely useful per result.

## Requirements mapping

- R1: canonical sources only. Tags stay live on `/tags.json`; relations come from `/tag_aliases.json` and
  `/tag_implications.json`. No hardcoded denylist of junk strings; junk is excluded structurally by `post_count > 0`.
- R2: `exact` boolean, default `false`; exact mode never returns substring matches.
- R3: alias resolution returns `alias_of` plus the target's count.
- R4: `implications` array on every result, `[]` when unknown, never `null`.
- R5: `category` accepts `general`, `character`, `artist`, `copyright`, `meta`; applied upstream via `search[category]`.
- R6: `status` is `ok`, `no_substring_match`, `exact_not_found`, or `unknown`; every call carries `snapshot_date`.
- R7: results sorted by count descending, tie-break name ascending.
- R8: spaces/underscores equivalence, `more` affordance, offset/limit walking, graceful empties, category label kept.
- R9: description and parameter docs rewritten.
- R10: `name`, `category`, and `count` keep their names; new fields are added.

## Design

### Live tags, cached implications

`internal/booru` keeps the Danbooru client and gains the canonical query primitives: `FindTag` (`search[name]`),
`AliasTarget` (`search[antecedent_name]`), `Implications` (`search[antecedent_name]`), and `ImplicationPage` (the
paginated bulk crawl). Substring `SearchTags` gains a category filter, a deterministic sort, and drops `post_count <= 0`
rows while paging, so `more` is computed over real matches only.

`internal/booru` also gains `ImplicationIndex`, the one piece of ingested data. It crawls `tag_implications.json`
(`search[status]=active`, `only=antecedent_name,consequent_name`, `limit=1000`, page walk) into a
`map[antecedent][]consequent`, exposes `Implications(name)`, and refreshes itself on a TTL in the background. Building
and refreshing the index is a separate type from the query path: the catalog only ever reads an interface.

The index is started from `Server.Run`, not from `New`, so tests and offline construction never make network calls.

### Query policy

`internal/catalog` keeps owning policy. `TagsInput` gains `Exact` and `Categories`; `TagsResult` gains `Status`,
`AliasOf`, and `SnapshotDate`. `TagSource` grows to the three lookups the policy needs. The implication index is an
optional `RelationIndex` on the service; when present it answers substring results for free, and exact lookups fall
back to a single upstream call so an exact answer is never blocked on the index.

Substring mode drops zero-count rows (already done by the client), enriches from the index when present, and reports
`no_substring_match` when nothing real matched. Exact mode resolves an active alias first, then the tag, and reports
`exact_not_found` only when neither is a real answer; `unknown` is reserved for the case where an exact miss could not
be confirmed because the alias lookup failed.

### Contract

`tags` gains `exact` and `category`. Each result gains `implications` and `alias_of`; the response gains `status`,
`snapshot_date`, and `exact`. `snapshot_date` is the UTC date the counts were read, since the counts are live.

## Implementation units

### Unit 1: Canonical query primitives in the Danbooru client

Scope: `internal/booru`.

- [x] `Tag` gains `AliasOf` and `Implications`; `TagCategory` gains `Code` and `ParseTagCategory`.
- [x] `TagQuery` gains `Categories`; `SearchTags` sends comma-joined `search[category]`, drops zero-count rows while
      paging, and sorts by count descending then name ascending.
- [x] `FindTag`, `AliasTarget`, `Implications`, and `ImplicationPage` map `/tags.json` and `/tag_aliases.json` and
      `/tag_implications.json` with `only=` projections.

Acceptance criteria:

- [x] A substring search with two categories sends `search[category]=0,1`.
- [x] A page containing zero-count rows returns only the rows above them and sets `More` from real matches.
- [x] Equal counts come back name-ascending.
- [x] `FindTag` reports found/not-found; `AliasTarget` returns the active target; `Implications` returns sorted
      consequents.

### Unit 2: The implication index, separated from query logic

Scope: `internal/booru`, `internal/config`, `internal/server`.

- [x] `ImplicationIndex.Refresh` walks every active implication page and atomically swaps the map.
- [x] `Implications(name)` reads the map; `BuiltAt()` reports the last successful build.
- [x] `Start(ctx, interval)` refreshes in the background on the interval, immediately and then on a ticker.
- [x] `Server.Run` starts it; `New` and `newWithProvider` do not, so tests stay offline.

Acceptance criteria:

- [x] A refresh over several pages merges every page into the map.
- [x] A failed refresh leaves the previous map intact and does not error the caller.
- [x] An interval of zero disables the refresh.

### Unit 3: Catalog query semantics

Scope: `internal/catalog`.

- [x] `TagsInput` gains `Exact` and `Categories`; `TagsResult` gains `Status`, `AliasOf`, `SnapshotDate`.
- [x] Substring results are enriched from the `RelationIndex` when set, labelled `no_substring_match` when empty.
- [x] Exact results resolve aliases, return the alias row plus the canonical row, and label misses.
- [x] Categories are validated before any upstream call.

Acceptance criteria:

- [x] An exact alias query returns `alias_of` and the target's count, not a bare zero.
- [x] An exact miss returns `exact_not_found` and no tags.
- [x] A substring miss returns `no_substring_match` and no tags.
- [x] Every result carries `implications`, `[]` rather than `null`.

### Unit 4: The `tags` MCP contract

Scope: `internal/mcp`.

- [x] Schema adds `exact` (default false) and `category` (array of the five names); `search`, `offset`, `limit` docs
      rewritten.
- [x] Output adds `status`, `snapshot_date`, `exact`, and per-tag `implications`/`alias_of`.
- [x] Description states substring-by-default, snapshot counts, zero-count unreliability, `exact` for existence, and
      that aliases and implications are surfaced when known.
- [x] The text renderer names the status and the alias.

Acceptance criteria:

- [x] `tags` lists exactly `search`, `offset`, `limit`, `exact`, `category`.
- [x] An invalid category is rejected with a message naming the valid values.
- [x] An empty result renders the status, not an authoritative "No tags matched."

### Unit 5: Configuration and documentation

Scope: `internal/config`, `.env.example`, `README.md`, `AGENTS.md`.

- [x] `IMPLICATION_INDEX_REFRESH_HOURS` (default 24, 0 disables) on `Config`.
- [x] README documents the new arguments, fields, and the snapshot caveat.
- [x] `AGENTS.md` prose names the implication index.
- [ ] `.env.example` documents `IMPLICATION_INDEX_REFRESH_HOURS`; the file is guarded by the editor's
      `private_files` setting, so the envar is documented in the README instead and the example file is left to a human.

Acceptance criteria:

- [x] A zero-hour setting disables the index and substring results still work.

### Unit 6: Tests and the acceptance fixtures

Scope: all touched packages, plus a live fixture run.

- [x] Unit tests cover categories, zero-count filtering, ordering, alias, implications, statuses, and the schema.
- [x] Unit tests for `ImplicationIndex` refresh, failure, and disable.
- [x] The eleven acceptance fixtures were run against live Danbooru on 2026-09-24 (counts drift as documented below).

Acceptance criteria:

- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race` pass.
- [x] The eleven fixtures behave as specified, counts subject to live drift.

## Risks and follow-ups

- Counts drift between the plan's figures and any live run; the fixtures assert ordering and shape, not exact numbers.
- `zzzzzzzzzz` matches a handful of real low-count artist tags, so it is not empty; it must not return the 200-character
  zero-count row, which is the actual regression the fixture guards.
- Building the implication index shares the request limiter, so it briefly contends with live queries at startup.
