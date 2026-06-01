# Auto-hyperlink ITPortal objects in answers

**Date:** 2026-06-01
**Status:** Approved (design)

## Goal

When the assistant's answer names a specific object documented in ITPortal
(company, site, device, KB article, contact, account, agreement, document, IP
network, facility, cabinet, configuration), that object name renders as a
clickable Markdown hyperlink to the object's page in the ITPortal web app. The
answer surface is Open WebUI, which renders Markdown, so `[name](url)` links
work directly.

## Background

The MCP already carries portal links most of the way:

- The ITPortal REST API returns a read-only `url` field on most entities, in the
  v4 web-app format `{base}/v4/app/{type}/{id}` (confirmed for `devices` and
  `companies` in `docs/test-portal-api.ps1`, the v4 URL format test).
- `internal/cache/snapshot.go` already emits a `- **Portal Link**: <url>` bullet
  whenever an entity's `url` is non-empty.
- `internal/mcp/server.go` Instructions already note that `url` is a read-only
  portal deep-link.

So this is a wiring job, not a rewrite. Three real gaps remain:

1. The model is never instructed to hyperlink the objects it mentions.
2. Portal URLs appear as a raw bullet, not as a clickable object name.
3. Contacts and IP Networks emit no portal link at all — the snapshot's Contacts
   section never prints one, and the `IPNetwork` model has no `url` field.

## Design decisions (confirmed)

- **Belt-and-suspenders**, not instruction-only: backfill a URL for every object,
  render names as Markdown links in the snapshot, and instruct the model.
- **All entity types** get links, including contacts and IP networks.
- **Construct a fallback URL** when the API omits `url`:
  `{ITPORTAL_BASE_URL}/v4/app/{segment}/{id}`. The API-provided `url` always wins
  when present.

## Changes

### 1. `internal/itportal` — URL primitives

- Add `URL string \`json:"url,omitempty"\`` to the `IPNetwork` struct. It is the
  only documented entity missing the field; the API returns it like every other
  type.
- Add a `BaseURL() string` accessor on `Client` so other packages can build
  fallback links against the configured instance root.
- Add two pure helpers (no client state):
  - `PortalPathSegment(itemType string) string` — maps a canonical singular,
    lowercased item type to its v4 web-app path segment:

    | itemType        | segment          |
    |-----------------|------------------|
    | company         | companies        |
    | site            | sites            |
    | device          | devices          |
    | kb              | kbs              |
    | contact         | contacts         |
    | account         | accounts         |
    | agreement       | agreements       |
    | document        | documents        |
    | facility        | facilities       |
    | cabinet         | cabinets         |
    | configuration   | configurations   |
    | ipnetwork       | ipnetworks       |

    Returns `""` for unknown types (caller then produces no link).
  - `BuildPortalURL(base, itemType string, id int) string` — returns
    `{base}/v4/app/{segment}/{id}`, or `""` when `id == 0` or the segment is
    unknown.

### 2. `internal/cache` — backfill + clickable headings

- `Cache` stores `portalBaseURL string`, set from `client.BaseURL()` in `New`.
- In `build()`, after all entity slices are fetched, run a single backfill pass:
  for every struct whose `URL` is empty and whose `ID` is non-zero, set
  `URL = itportal.BuildPortalURL(portalBaseURL, <type>, ID)`. This fixes the JSON
  resources (`itportal://devices`, etc.) and the rendered Markdown from one
  place — no per-type special-casing inside `buildMarkdown`.
- `buildMarkdown`: render each entity heading as a Markdown link when a URL
  exists:

  ```
  ### [Name](url) (ID: N) — Company
  ```

  Fall back to the plain `### Name (ID: N)` heading when no URL is available.
  Keep the existing `- **Portal Link**: <url>` bullet as well — the redundancy is
  cheap (the snapshot is prompt-cached) and guarantees the URL survives any
  `search_docs` slice window, which returns a fixed line range around a match
  rather than always including the heading.
- Add the heading link and the `Portal Link` bullet to the **Contacts** and **IP
  Networks** sections, which currently emit neither.

### 3. `internal/mcp` — live tools + model instruction

- `Handler` gains a `baseURL` field (from `client.BaseURL()`), wired in
  `NewServer`.
- `get_entity_details`: before marshalling, backfill an empty `url` on the
  returned struct via `BuildPortalURL`. This is the most likely single-object
  citation path, so it should always carry a link. For the device path, backfill
  `device.URL` inside the `deviceDetail` wrapper.
- `list_entities`: relies on the API-provided `url` plus the model instruction.
  Result sets are large and the per-item backfill loop adds little over the
  snapshot path, so it is intentionally out of scope.
- Add a **Hyperlinking objects** rule to the server `Instructions`:

  > When your answer mentions a specific documented object by name, render the
  > name as a Markdown link to its portal URL — for example
  > `[CORE-SW-01](https://portal.example/v4/app/devices/42)`. Each object's link
  > is in its snapshot heading and its `url` field; reuse that link. Never invent
  > a URL, and never link an object that is not present in the snapshot or a tool
  > result.

## Data flow

```
snapshot build
  -> fetch all entity slices from ITPortal
  -> backfill url on every struct where empty (BuildPortalURL)
  -> buildMarkdown renders headings as [name](url) + Portal Link bullet
  -> served via itportal://snapshot (prompt-cached)
  -> model reads once, copies links into answers
     + Instructions reinforce hyperlinking
     + search_docs inherits links from the markdown slices
     + get_entity_details returns a guaranteed url
```

## Testing

- `internal/itportal`: unit-test `BuildPortalURL` and `PortalPathSegment` —
  API-url passthrough is handled by callers; the helper covers construct, unknown
  type (empty), and zero id (empty).
- `internal/cache`: extend `snapshot_test.go` to assert that a rendered heading
  contains a Markdown link to `…/v4/app/…`, that a struct with an empty `URL`
  gets one backfilled, and that the Contacts and IP Networks sections now emit a
  link.
- `internal/mcp`: extend the `get_entity_details` test to assert the backfilled
  `url` on a struct the API returned without one.

## Known risk

The v4 path segment is confirmed only for `devices` and `companies`. For the
other types the constructed fallback assumes the plural API collection name. The
API-provided `url` is always used verbatim when present, so the assumption only
matters when the API omits `url` — rare per the test suite — and is a one-line
fix in `PortalPathSegment` once verified against the live portal.

## Out of scope

- Backfilling links into `list_entities` output.
- Rewriting create-tool confirmation messages (they already print `Portal: <url>`
  from the API GET-back; IP-network/address creates that return no `url` could be
  backfilled later but are not part of this change).
