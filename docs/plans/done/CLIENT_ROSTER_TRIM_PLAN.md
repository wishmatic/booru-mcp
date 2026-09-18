# Client roster trim: Realbooru, TBIB, the e621 family, and the Philomena family

## Goal

Shrink the client roster to the sites worth querying. Remove `realbooru` and `tbib` (poor APIs), the whole e621 family
(`e621`, `e926`), and the whole Philomena family (`derpibooru`, `twibooru`, `furbooru`). Deleting every client of a
family makes that family's provider package unreachable, so delete the provider packages too rather than leave dead
code. Consolidate the config fields that only existed to serve the removed clients.

## Scope

In scope: `internal/config`, `internal/server`, deletion of `internal/e621` and `internal/philomena`, the affected
tests, `README.md`, `.env.example`, and the architecture diagram in `AGENTS.md`.

Out of scope: the remaining Danbooru, Gelbooru, and Moebooru providers; the canonical tag model; the catalog's generic
rating policy (see Design); historical plans under `docs/plans/done`.

## Design

### Resulting roster

| Family   | Clients                                     | Default |
| -------- | ------------------------------------------- | ------- |
| Danbooru | `danbooru`                                  | yes     |
| Gelbooru | `gelbooru`, `rule34`, `xbooru`, `safebooru` | yes     |
| Moebooru | `yandere`, `konachan`, `sakugabooru`        | yes     |

Eight clients remain, all in the default set.

### Config consolidation

After `e926` is removed, no client has `InDefault: false`, so the field is uniformly true and `EnabledClients()` can
default to every spec when `BOORU_CLIENTS` is empty. Drop the field and fold `defaultClientNames()` into
`allClientNames()`.

After the e621 family is removed, no client sets `RequiresUserAgent`, so that field, the `ClientActive` check that uses
it, and the `DefaultUserAgent` constant (unused once the check is gone) are all dead. Drop them. `USER_AGENT` still
defaults to `booru-mcp/0.1.0` and is still sent on every request; only the "e621 rejects the default value" rule
disappears.

### Provider switch

`buildProvider` currently handles the Philomena family in a `default` branch. With both e621 and Philomena gone, every
remaining family has an explicit case and the branch has nothing valid to do. Change `buildProvider` to return
`(booru.Provider, error)` and have `default` return an error naming the unsupported family; `buildClients` propagates
it. This keeps the impossible case explicit instead of registering a nil provider.

### Deliberately kept

`booru.Capabilities.Rating` and the catalog rule that a provider without rating support is only allowed at
`explicit` and above are a generic part of the provider contract and the catalog policy. They are not specific to
Philomena, so they stay. The catalog test that covers the rule keeps its coverage; only its fixture client name changes
to avoid naming a removed site.

### Removals and side effects

- `internal/e621` and `internal/philomena` are deleted with their tests.
- `.env.example` loses the `REALBOORU_*`, `TBIB_*`, `E621_*`, `E926_*`, `DERPIBOORU_API_KEY`, `TWIBOORU_API_KEY`, and
  `FURBOORU_API_KEY` entries, and its client-list comment and `USER_AGENT` note are rewritten.
- `README.md` loses the e621 and Philomena rows, the removed names from the trademark list, the e621 half of the
  `related` bullet, and the "no rating metadata" warning bullet. The country-restriction note now names both `rule34`
  and `xbooru`.
- The architecture diagram in `AGENTS.md` loses the `e621` and `philomena` nodes and edges.

## Implementation units

### Unit 1: Remove Realbooru and TBIB

Remove the two Gelbooru-family specs, their URL constants, their `.env.example` entries, their README table entries, and
their license-list names.

Acceptance criteria:

- [x] `clientSpecs` contains neither `realbooru` nor `tbib`, and `realbooruURL` and `tbibURL` are gone.
- [x] `BOORU_CLIENTS=realbooru` fails validation with an unknown-client error.
- [x] `.env.example` has no `REALBOORU_*` or `TBIB_*` entries and its client-list comment omits both names.
- [x] `go build ./...` and `go test ./...` pass.

### Unit 2: Remove the e621 family and the User-Agent requirement

Delete `internal/e621`; remove the `e621` and `e926` specs, `e621URL` and `e926URL`, and `FamilyE621`; drop the
`FamilyE621` case, the e621 import, and the `RequiresUserAgent` field plus its `ClientActive` check and the
`DefaultUserAgent` constant. Consolidate `InDefault` (now uniformly true) out of `ClientSpec` and
`EnabledClients()`. Update `README.md`, `.env.example`, `AGENTS.md`, and the affected tests.

Acceptance criteria:

- [x] `internal/e621` no longer exists, and no non-historical file references `e621`, `e926`, or `FamilyE621`.
- [x] `ClientSpec` has no `InDefault` or `RequiresUserAgent` field; `EnabledClients()` returns every spec when
      `BOORU_CLIENTS` is empty and when it is `all`.
- [x] `go build ./...` and `go test ./...` pass, including a config test that the default and `all` client lists are
      the full roster.
- [x] `README.md` and `.env.example` no longer claim a non-default `USER_AGENT` is required by any client.

### Unit 3: Remove the Philomena family

Delete `internal/philomena`; remove the `derpibooru`, `twibooru`, and `furbooru` specs, their URL constants, and
`FamilyPhilomena`; remove the philomena import and the `default` branch. Change `buildProvider` to return an error for
an unsupported family and propagate it from `buildClients`. Update `README.md`, `.env.example`, `AGENTS.md`, and the
affected tests.

Acceptance criteria:

- [x] `internal/philomena` no longer exists, and no non-historical file references `derpibooru`, `twibooru`,
      `furbooru`, `philomena`, or `FamilyPhilomena`.
- [x] `buildProvider` returns an error naming an unsupported family instead of constructing a Philomena provider.
- [x] `go build ./...` and `go test ./...` pass.
- [x] `README.md`, `.env.example`, and the `AGENTS.md` diagram list only the remaining families.

### Unit 4: Country-restriction note for Xbooru

Extend the README note to cover `xbooru` alongside `rule34`, since both refuse requests from some countries and
regions.

Acceptance criteria:

- [x] `README.md` states that `rule34` and `xbooru` are subject to country and region restrictions.
- [x] No other country-specific claim is added.

## Verification

- Automated: `gofmt -l .`, `go build ./...`, and `go test ./...` (including `-race`) pass on the final tree.
- Automated: `grep` finds no reference to any removed client or family outside `docs/plans/done`.
- Human: with `BOORU_CLIENTS=all`, startup registers exactly the eight remaining clients and the log has no entry for a
  removed one.
- Human: a call that names a removed client (`search` with `clients: ["e621"]`) returns the unknown-client error rather
  than a silent empty result.
