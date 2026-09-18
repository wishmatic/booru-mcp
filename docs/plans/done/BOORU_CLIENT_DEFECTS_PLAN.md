# Booru client defects: transport diagnostics, per-client status, tier caps, and client mappings

## Goal

Fix every defect reproduced against the live booru services that lives inside this repository's surface: silent
per-client failures, undiagnosed transport faults, Danbooru's tag cap, Rule34 credentials and host, Safebooru's array
shape, Xbooru's HTML body, Danbooru's related-tag object shape, Gelbooru's popular ranking, missing image URLs, and
inconsistent random support.

## Scope

In scope: `internal/fetch`, `internal/booru`, `internal/danbooru`, `internal/gelbooru`, `internal/moebooru`,
`internal/catalog`, `internal/mcp`, `internal/present`, `internal/config`, `internal/server`, and their tests, plus
README updates.

Out of scope (not present in this repository): the Neo image tools. Findings P2-9 (`examples` model validation) and
P2-10 (`examples` seed/dimension persistence) describe tools that do not exist here; there is no `examples` tool, no
model list, and no image-generation client. They cannot be implemented or tested against this codebase.

Deliberately unchanged: Gelbooru `get` tagging everything `general` (upstream limitation), Moebooru zero-count alias
stubs, and the `bgkill` image-quality limitation.

## Findings and root causes

Verified against live services on 2026-09-18:

- Safebooru `s=post&json=1` returns a bare JSON array, not the Gelbooru envelope. Its `s=tag` returns XML
  (`text/xml`, `<tags type="array">`) even with `json=1`. The whole sanitised-tag search path therefore fails on a
  decode error.
- Danbooru `/related_tag.json` returns `related_tags[].tag` as an object and `frequency` as a float (for example
  `0.4818`), and no longer returns `co_occurrence`. The current struct wants a string and an int, so the call fails
  twice over.
- `api.rule34.xxx` returns HTTP 200, `application/json`, with a bare JSON string body
  `"Missing authentication. Go to api.rule34.xxx for more information"`. `rule34.xxx` and `api.rule34.xxx` require
  `user_id` + `api_key`, and the API lives on `api.rule34.xxx` now.
- `xbooru.com` returns an HTML region/age-restriction page (first byte `<`). `api.xbooru.com` does not resolve
  (`No such host`), so switching host is not a fix; the defect is that the HTML body is reported as a Go type error.
- Yandere honours `order:random` (a live call returned different posts than the ordered query). The Moebooru client
  never sends it, so random is silently ignored.
- Gelbooru's `orderby=count` tag list genuinely contains, in count order, an empty-named row (14,115,639) and
  deprecated alias rows (`1firl` 8,540,770, `1gir`, `1girls`, `exposed_breats`, `metallic_breasts`) whose type is
  `6` (deprecated). The public tag listing shows the identical rows and numbers, so `count` is the right field; the
  defect is that `popular` ranks deprecated alias stubs and an unusable empty name instead of real tags.

## Design

### Typed transport errors

`internal/fetch` gains a `BodyError` returned when the HTTP status is 200 but the body cannot be a JSON document:
HTML/XML (`<` first byte), a bare JSON string (an upstream message, as Rule34 sends), any other non-JSON first byte,
or a body that fails to unmarshal into the requested shape. It carries the client label, path, status code,
content type, a bounded body excerpt, and a reason. Non-200 responses keep using `HTTPError`.

### Per-client status

`internal/booru` gains `ClientState` (`ok`, `error`, `skipped`) and `ClientStatus{Client, State, Results, Detail}`.
Every catalog result carries `Clients []ClientStatus` covering every requested client, including inactive ones. A
client that succeeded with zero results reports `ok (0 results)` and is thereby distinct from one that failed.

### Danbooru tier cap

`DANBOORU_TIER` (`auto` default) resolves to a tag cap: anonymous and member 2, gold 6, platinum and builder
unlimited. `auto` is anonymous without credentials and gold with them, matching the existing credential hint.
`internal/danbooru` validates the counted terms (everything except `rating:` terms, since Danbooru does not count
those) before sending, and returns an actionable error naming the cap. No negative tags are ever auto-appended;
configured `BLOCKED_TAGS` remain the only negation, and they still count toward the cap.

### Client mappings

- Gelbooru `postResponse` unmarshals both `{"post":[...]}` and a bare `[...]`.
- Gelbooru drops empty-named tags everywhere and drops deprecated (type 6) tags from `popular`.
- Rule34 points at `https://api.rule34.xxx`, and the Gelbooru client translates a `BodyError` whose body is an
  authentication message into one naming the required credential envars.
- Danbooru related tags decode `tag` as either an object or a string, and `frequency` as a float.
- Moebooru advertises `Capabilities.Random` and appends `order:random`; clients that cannot randomise are reported as
  `random ordering not applied` in their status detail.

### Presentation

`present.Clients` renders one line per non-skipped client. Search and get emit an explicit
`(file URL unavailable for this post)` marker instead of omitting the line.

## Implementation units

### Unit 1: Transport body validation and typed errors

Scope: `internal/fetch`.

- [x] `BodyError` type with label, path, status, content type, bounded excerpt, and reason; `Error()` is
      human-readable.
- [x] `GetJSON`/`GetJSONWithHeaders` read a bounded body, reject empty, HTML/XML, bare JSON string, and other
      non-JSON bodies before unmarshalling, and return `BodyError` on unmarshal failure.

Acceptance criteria:

- [x] A 200 response with an HTML body yields a `*BodyError` naming client, path, status 200, and an excerpt.
- [x] A 200 response with a bare JSON string yields a `*BodyError` whose error text contains the string content.
- [x] A 200 response with valid but wrong-shaped JSON (for example a JSON array for an object target) yields a
      `*BodyError`, not a bare `json` error.
- [x] A 200 response with valid JSON still decodes, and non-200 handling and retry behaviour are unchanged.

### Unit 2: Per-client status on every combined tool

Scope: `internal/booru`, `internal/catalog`, `internal/mcp`, `internal/present`.

- [x] `ClientState`, `ClientStatus`, and catalog status helpers.
- [x] `Search`, `Tags`, `Popular`, and `Related` populate `Clients` for every requested client, in request order,
      including inactive clients, rating-excluded clients, and clients without a related-tag API.
- [x] MCP outputs carry `clients`; the text result names each non-skipped client with state, result count, and reason
      for failure.

Acceptance criteria:

- [x] A three-client search where one client errors returns the other two groups and a `Clients` entry naming the
      failing client with its reason.
- [x] A client that returns zero results reports `ok (0 results)`, distinguishable from `error`.
- [x] Partial-failure calls still succeed; an all-fail call still returns an error.

### Unit 3: Danbooru tier cap and negative-tag hygiene

Scope: `internal/config`, `internal/danbooru`, `internal/server`.

- [x] `DANBOORU_TIER` config with `auto`, `anonymous`, `member`, `gold`, `platinum`, `builder`; validated at startup.
- [x] Danbooru client counts search terms (excluding `rating:` terms) and rejects over-cap searches before sending,
      with an actionable message naming the cap and the terms sent.
- [x] No automatic negative tags are appended to any search.

Acceptance criteria:

- [x] A query at exactly the cap succeeds; one term over fails with a message naming the cap.
- [x] The terms sent for a plain single-tag query are exactly that tag, with no `-` terms.

### Unit 4: Rule34 credentials and Host, Safebooru array shape, Gelbooru ranking

Scope: `internal/config`, `internal/gelbooru`.

- [x] Rule34 default URL becomes `https://api.rule34.xxx`.
- [x] `postResponse` accepts the Gelbooru envelope and a bare array.
- [x] Empty-named tags are dropped; deprecated (type 6) tags are dropped from `popular`.
- [x] Authentication-message bodies produce an error naming the required credential envars.

Acceptance criteria:

- [x] Searching Safebooru against a bare-array body maps every field.
- [x] A 200 bare-string authentication body yields an error naming the client and its credential envars.
- [x] Popular output excludes deprecated and empty-named tags; tag search excludes empty names.

### Unit 5: Danbooru related-tag shape

Scope: `internal/danbooru`, `internal/booru`, `internal/store`.

- [x] `tag` decodes from an object or a string; `frequency` decodes as a float; `RelatedTag.Score` becomes a float.
- [x] Store schema uses a REAL score column; scanning tolerates existing databases.

Acceptance criteria:

- [x] The live-shaped related payload parses without error and preserves names, order, and scores.
- [x] The legacy string-tag payload still parses.

### Unit 6: Random support and unavailable-URL markers

Scope: `internal/moebooru`, `internal/catalog`, `internal/present`.

- [x] Moebooru sends `order:random` and advertises the capability.
- [x] Clients without random support report `random ordering not applied` in their status detail when random is
      requested.
- [x] Search and get print an explicit unavailable marker for a missing file, sample, or preview URL.

Acceptance criteria:

- [x] A Moebooru search with `random=true` sends `order:random`.
- [x] A random search against a non-randomising client reports the limitation.
- [x] A post with no file or preview URL renders a marker, not silence.

### Unit 7: Xbooru HTML diagnosis

Scope: `internal/gelbooru`, documentation.

- [x] Xbooru's HTML body surfaces as a typed, human-readable error naming the client and the HTML body.
- [x] Record that `api.xbooru.com` does not resolve and that the block is region-based, so no host change is made.

Acceptance criteria:

- [x] A 200 HTML body from an Xbooru query yields a typed error naming Xbooru and identifying the body as HTML.

### Unit 8: Regression tests

Scope: all touched packages.

- [x] Multi-client search with one failing client returns the other groups and names the failure.
- [x] Each client queried alone against a non-JSON body yields a typed, human-readable error.
- [x] Danbooru at the cap succeeds; one over fails actionably.
- [x] No auto-appended negative tags appear in the terms sent.
- [x] Get on a known id returns file, sample, preview, and categories.
- [x] Related on a known tag parses.
- [x] Random is applied or reported.

Human verification (cannot be automated here without operator credentials and a non-blocked region):

- [ ] Gelbooru `popular` top rows against the public tag listing. (Gelbooru dapi requires operator credentials; the
      public tag listing was checked directly and the deprecated and empty rows were confirmed, which the filter
      addresses.)
- [ ] Rule34 with real credentials returns posts.
- [ ] Xbooru from an unrestricted region or with a proxy.
