# Canonical alias recall and the unknown-status fixture

Status: done. Follow-up to the tag-search fix.

## Goal

Close two gaps left by the canonical tag-search fix: the alias graph was only used for exact resolution, so a substring
search could never surface a tag's other canonical names; and the `unknown` status had unit coverage but no fixture
through the MCP contract.

## Root cause finding for the synonym gap

Read from the live canonical tables on 2026-09-24:

- `piss` → `pee` (active alias); `urine` → `pee`.
- `urination` → `peeing`; `pissing` → `peeing`; `watersports` → `peeing`.

There is no alias and no implication between the `pee` cluster and the `peeing` cluster. Danbooru's graph therefore
makes `piss` recall `pee` and `urine`, and makes `peeing`/`urination` recall each other, but it does **not** connect
`piss` to `peeing`/`urination`. Surfacing those from `piss` would need a semantic source (embeddings), which this tool
does not have and which is out of scope. Alias recall is the canonical mechanism and the one implemented.

## Implementation

- `internal/booru/relations.go`: `ImplicationIndex` becomes `RelationIndex`, which now ingests the active alias graph
  alongside the active implication graph in the same background crawl and atomic swap. It gains `Canonical` (chain
  following), `Synonyms` (the target and its sibling aliases), and `Ready` (built at least once). `AliasPage` on the
  client feeds it.
- `internal/catalog`: `RelationIndex` grows the same three methods. Substring results carry `synonyms`, computed from
  the index only so recall never adds an upstream request per search; names already returned, the search itself, and
  blocked names are dropped. Exact mode resolves the canonical through a ready index and only falls back upstream when
  the index has not been built.
- `internal/mcp`: the response gains `synonyms`; the text renderer adds "Also known as:" / "Known aliases:"; the
  description documents the field.
- `RELATION_INDEX_REFRESH_HOURS` replaces `IMPLICATION_INDEX_REFRESH_HOURS`, since the index now covers both graphs.

## Implementation units

### Unit 1: Index the alias graph

- [x] `RelationIndex` crawls `tag_aliases.json` as well as `tag_implications.json`, swapping both atomically.
- [x] `Canonical` follows a chain with a cycle guard; `Synonyms` returns the target plus sibling aliases; `Ready` and
      `BuiltAt` report build state.

Acceptance criteria:

- [x] A refresh over several pages merges aliases and implications.
- [x] A failed crawl leaves both previous graphs intact.

### Unit 2: Surface synonyms

- [x] Substring results carry `synonyms`, index-only, minus shown/blocked names.
- [x] Exact mode uses a ready index for canonical resolution and falls back upstream otherwise.

Acceptance criteria:

- [x] `piss` reports `pee` and `urine`.
- [x] `peeing` reports `pissing`, `urination`, `watersports`; `urination` reports the same cluster.
- [x] `blue hair` is unchanged (`synonyms: []`), preserving the earlier fixtures.

### Unit 3: The unknown fixture

- [x] An end-to-end session fixture drives a failing alias lookup and asserts `status: unknown` in the structured
      content.

Accepted by its own test through the MCP contract.

## Risks and follow-ups

- Synonyms are names only and are not subject to `category`, because resolving categories would add a lookup per name.
  A caller wanting a count calls the tool with `exact: true`.
- `piss` cannot reach `peeing`/`urination`; that is a property of Danbooru's graph, not a bug in this tool.
