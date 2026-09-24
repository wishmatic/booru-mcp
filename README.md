# Booru MCP

An MCP server, written in Go, that gives image generation models real booru tags. It exposes one tool, `tags`, which
searches tag names on Danbooru and returns the matches with their category and work count.

## The `tags` tool

| Argument    | Required | Default | Notes                                                                                   |
| ----------- | -------- | ------- | --------------------------------------------------------------------------------------- |
| `search`    | yes      |         | a literal substring of the tag name, so `blue hair` and `blue_hair` are the same search |
| `offset`    | no       | `0`     | how many matching tags to skip                                                          |
| `limit`     | no       | `25`    | how many matching tags to return                                                        |
| `min_count` | no       | `1`     | lowest work count a substring match may have; `0` includes the zero-work canonical tail |
| `exact`     | no       | `false` | when true, `search` is the whole tag name and the result is that one tag or nothing     |
| `category`  | no       | all     | keep only these categories: `general`, `artist`, `copyright`, `character`, `meta`       |

Results come back ordered by work count (name ascending on ties) as one page of the match list:

```json
{
    "search": "blue_hair",
    "exact": false,
    "status": "ok",
    "snapshot_date": "2026-09-24",
    "offset": 0,
    "limit": 25,
    "min_count": 1,
    "more": true,
    "alias_of": "",
    "withheld": 0,
    "withheld_best_count": 0,
    "synonyms": [],
    "tags": [
        {
            "name": "blue_hair",
            "category": "general",
            "count": 1207766,
            "count_is_target": false,
            "alias_of": "",
            "implications": []
        }
    ]
}
```

Walk the list with `offset` and stop when `more` is false. Substring results exclude tags below `min_count` works
(default 1), because Danbooru's user-editable tag table also holds concatenated and punctuation-mangled zero-work
names. Those rows are still real canonical tags, so `withheld` and `withheld_best_count` report what the floor removed,
and `min_count: 0` returns them; an empty page is never phrased as proof that the tag does not exist.

`synonyms` lists canonical tags the search is also known by, taken from Danbooru's alias graph, which substring
matching can never find: searching `piss` reports `pee` and `urine`. Synonyms are names only and are not subject to
`category`; call the tool with `exact: true` on one to get its count.

An empty `tags` array is accompanied by a `status` that says whether the search genuinely matched nothing
(`no_substring_match`), whether an exact name was absent (`exact_not_found`), or whether an unanswerable case was
reached (`unknown`).

Use `exact: true` to ask whether one tag exists. It is the only reliable existence check: a substring miss proves
nothing. Aliases resolve to their target, reported as `alias_of` and the target's count (`count_is_target` marks a
count that belongs to the target rather than to the named tag), and each result carries the `implications` Danbooru
knows for it. Counts are read live, so `snapshot_date` records the date they were read.

Every call is live; the only cached data is the alias and implication graph, which is crawled in the background and
refreshed on `RELATION_INDEX_REFRESH_HOURS`. An upstream failure is reported as an error rather than as an empty page.

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
