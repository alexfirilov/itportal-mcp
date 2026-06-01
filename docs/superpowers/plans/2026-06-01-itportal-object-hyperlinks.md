# ITPortal Object Hyperlinks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the assistant render every documented ITPortal object it mentions as a clickable Markdown link to that object's page in the portal.

**Architecture:** Backfill a portal `url` on every entity (API value wins, else construct `{base}/v4/app/{segment}/{id}`), render snapshot headings as Markdown links so `search_docs` and the cached snapshot carry links automatically, backfill the same url on single-object `get_entity_details` lookups, and add a server instruction telling the model to hyperlink object mentions.

**Tech Stack:** Go, the `modelcontextprotocol/go-sdk` MCP server, standard `testing` package.

---

## File Structure

- `internal/itportal/client.go` — add `BaseURL()` accessor on `Client`.
- `internal/itportal/models.go` — add `URL` field to `IPNetwork`.
- `internal/itportal/portal_url.go` (new) — `PortalPathSegment` + `BuildPortalURL` pure helpers.
- `internal/itportal/portal_url_test.go` (new) — helper unit tests.
- `internal/cache/snapshot.go` — store `portalBaseURL`, add `backfillPortalURLs`, render headings as links, add links to Contacts + IP Networks.
- `internal/cache/snapshot_test.go` — backfill + link-rendering tests.
- `internal/mcp/server.go` — set `Handler.baseURL`, add hyperlinking instruction.
- `internal/mcp/tools.go` — add `baseURL` field, `marshalWithURL` helper, backfill in `get_entity_details`.
- `internal/mcp/tools_url_test.go` (new) — `marshalWithURL` backfill test.

---

## Task 1: URL primitives in the itportal package

**Files:**
- Modify: `internal/itportal/client.go` (add `BaseURL()` near the other `Client` methods)
- Modify: `internal/itportal/models.go:365-382` (the `IPNetwork` struct)
- Create: `internal/itportal/portal_url.go`
- Test: `internal/itportal/portal_url_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/itportal/portal_url_test.go`:

```go
package itportal

import "testing"

func TestBuildPortalURL(t *testing.T) {
	const base = "https://portal.example"
	cases := []struct {
		name     string
		itemType string
		id       int
		want     string
	}{
		{"device", "device", 42, "https://portal.example/v4/app/devices/42"},
		{"company", "company", 7, "https://portal.example/v4/app/companies/7"},
		{"ipnetwork", "ipnetwork", 3, "https://portal.example/v4/app/ipnetworks/3"},
		{"kb alias", "knowledgebase", 5, "https://portal.example/v4/app/kbs/5"},
		{"zero id", "device", 0, ""},
		{"unknown type", "widget", 9, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildPortalURL(base, tc.itemType, tc.id); got != tc.want {
				t.Errorf("BuildPortalURL(%q, %d) = %q, want %q", tc.itemType, tc.id, got, tc.want)
			}
		})
	}
}

func TestBuildPortalURLTrimsTrailingSlash(t *testing.T) {
	if got := BuildPortalURL("https://portal.example/", "device", 1); got != "https://portal.example/v4/app/devices/1" {
		t.Errorf("trailing slash not trimmed: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/itportal/ -run TestBuildPortalURL -v`
Expected: FAIL — `undefined: BuildPortalURL`.

- [ ] **Step 3: Create the helper file**

Create `internal/itportal/portal_url.go`:

```go
package itportal

import (
	"strconv"
	"strings"
)

// PortalPathSegment maps a normalised (lowercase, no underscores) entity type to
// its segment in the ITPortal v4 web-app URL (/v4/app/<segment>/<id>). Returns ""
// for unknown types. Confirmed against the live portal for devices and companies;
// the rest follow the plural API collection names.
func PortalPathSegment(itemType string) string {
	switch strings.ToLower(strings.ReplaceAll(itemType, "_", "")) {
	case "company":
		return "companies"
	case "site":
		return "sites"
	case "device":
		return "devices"
	case "kb", "knowledgebase":
		return "kbs"
	case "contact":
		return "contacts"
	case "account":
		return "accounts"
	case "agreement":
		return "agreements"
	case "document":
		return "documents"
	case "facility":
		return "facilities"
	case "cabinet":
		return "cabinets"
	case "configuration":
		return "configurations"
	case "ipnetwork":
		return "ipnetworks"
	}
	return ""
}

// BuildPortalURL constructs the v4 web-app deep-link for an object, or "" when the
// id is zero or the type is unknown. Callers should prefer an API-provided url and
// only fall back to this when that url is empty.
func BuildPortalURL(base, itemType string, id int) string {
	seg := PortalPathSegment(itemType)
	if seg == "" || id == 0 {
		return ""
	}
	return strings.TrimRight(base, "/") + "/v4/app/" + seg + "/" + strconv.Itoa(id)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/itportal/ -run TestBuildPortalURL -v`
Expected: PASS (both test functions).

- [ ] **Step 5: Add the `BaseURL()` accessor**

In `internal/itportal/client.go`, immediately after the `NewClient` function (right before `buildAuthHeader`), add:

```go
// BaseURL returns the configured ITPortal instance root (no trailing slash).
func (c *Client) BaseURL() string {
	return c.baseURL
}
```

- [ ] **Step 6: Add the `URL` field to `IPNetwork`**

In `internal/itportal/models.go`, in the `IPNetwork` struct, add a `URL` field after `Modified` (matching the other entities that already carry it):

```go
	Modified       string            `json:"modified,omitempty"`
	URL            string            `json:"url,omitempty"`
}
```

- [ ] **Step 7: Verify the package builds and tests pass**

Run: `go build ./... && go test ./internal/itportal/`
Expected: build succeeds; tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/itportal/portal_url.go internal/itportal/portal_url_test.go internal/itportal/client.go internal/itportal/models.go
git commit -m "feat(itportal): add portal URL helpers and IPNetwork url field"
```

---

## Task 2: Backfill portal URLs in the snapshot

**Files:**
- Modify: `internal/cache/snapshot.go` (Cache struct, `New`, `build`, new `backfillPortalURLs`)
- Test: `internal/cache/snapshot_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/cache/snapshot_test.go`:

```go
func TestBackfillPortalURLs(t *testing.T) {
	s := &Snapshot{
		Devices:    []itportal.Device{{ID: 9, Name: "fw01"}, {ID: 10, Name: "sw01", URL: "https://api-given/x"}},
		Contacts:   []itportal.Contact{{ID: 4, FirstName: "Ada"}},
		IPNetworks: []itportal.IPNetwork{{ID: 3, Name: "LAN"}},
	}
	backfillPortalURLs(s, "https://portal.example")

	if s.Devices[0].URL != "https://portal.example/v4/app/devices/9" {
		t.Errorf("device 9 url = %q, want constructed link", s.Devices[0].URL)
	}
	if s.Devices[1].URL != "https://api-given/x" {
		t.Errorf("device 10 url overwritten = %q, want API value preserved", s.Devices[1].URL)
	}
	if s.Contacts[0].URL != "https://portal.example/v4/app/contacts/4" {
		t.Errorf("contact 4 url = %q, want constructed link", s.Contacts[0].URL)
	}
	if s.IPNetworks[0].URL != "https://portal.example/v4/app/ipnetworks/3" {
		t.Errorf("ipnetwork 3 url = %q, want constructed link", s.IPNetworks[0].URL)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache/ -run TestBackfillPortalURLs -v`
Expected: FAIL — `undefined: backfillPortalURLs`.

- [ ] **Step 3: Add the backfill function**

In `internal/cache/snapshot.go`, add this function (place it just above `buildMarkdown`):

```go
// backfillPortalURLs sets a constructed portal deep-link on every entity whose
// API-provided url is empty, so the snapshot and JSON resources always carry a
// link. Entities that already have a url keep it untouched.
func backfillPortalURLs(s *Snapshot, base string) {
	if base == "" {
		return
	}
	set := func(url *string, itemType string, id int) {
		if *url == "" {
			*url = itportal.BuildPortalURL(base, itemType, id)
		}
	}
	for i := range s.Companies {
		set(&s.Companies[i].URL, "company", s.Companies[i].ID)
	}
	for i := range s.Sites {
		set(&s.Sites[i].URL, "site", s.Sites[i].ID)
	}
	for i := range s.Devices {
		set(&s.Devices[i].URL, "device", s.Devices[i].ID)
	}
	for i := range s.KBs {
		set(&s.KBs[i].URL, "kb", s.KBs[i].ID)
	}
	for i := range s.Contacts {
		set(&s.Contacts[i].URL, "contact", s.Contacts[i].ID)
	}
	for i := range s.Agreements {
		set(&s.Agreements[i].URL, "agreement", s.Agreements[i].ID)
	}
	for i := range s.IPNetworks {
		set(&s.IPNetworks[i].URL, "ipnetwork", s.IPNetworks[i].ID)
	}
	for i := range s.Documents {
		set(&s.Documents[i].URL, "document", s.Documents[i].ID)
	}
	for i := range s.Accounts {
		set(&s.Accounts[i].URL, "account", s.Accounts[i].ID)
	}
	for i := range s.Facilities {
		set(&s.Facilities[i].URL, "facility", s.Facilities[i].ID)
	}
	for i := range s.Cabinets {
		set(&s.Cabinets[i].URL, "cabinet", s.Cabinets[i].ID)
	}
	for i := range s.Configurations {
		set(&s.Configurations[i].URL, "configuration", s.Configurations[i].ID)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cache/ -run TestBackfillPortalURLs -v`
Expected: PASS.

- [ ] **Step 5: Store the base URL on the Cache and call the backfill**

In `internal/cache/snapshot.go`, add a field to the `Cache` struct (after `deviceLimit`):

```go
	deviceLimit     int
	portalBaseURL   string
```

In `New`, set it right after the `Cache` literal is created. Change:

```go
	c := &Cache{
		client:          client,
		limitPerEntity:  limitPerEntity,
		deviceLimit:     deviceLimit,
		refreshInterval: refreshInterval,
		logger:          logger,
	}
```

to:

```go
	c := &Cache{
		client:          client,
		limitPerEntity:  limitPerEntity,
		deviceLimit:     deviceLimit,
		portalBaseURL:   client.BaseURL(),
		refreshInterval: refreshInterval,
		logger:          logger,
	}
```

In `build`, find the block that assembles `snap` and renders Markdown:

```go
	snap.Markdown = buildMarkdown(snap)
	return snap, nil
```

Change it to backfill first:

```go
	backfillPortalURLs(snap, c.portalBaseURL)
	snap.Markdown = buildMarkdown(snap)
	return snap, nil
```

- [ ] **Step 6: Verify the package builds and all cache tests pass**

Run: `go build ./... && go test ./internal/cache/`
Expected: build succeeds; all tests PASS (existing tests still green).

- [ ] **Step 7: Commit**

```bash
git add internal/cache/snapshot.go internal/cache/snapshot_test.go
git commit -m "feat(cache): backfill portal deep-links on every snapshot entity"
```

---

## Task 3: Render snapshot headings as Markdown links

**Files:**
- Modify: `internal/cache/snapshot.go` (`buildMarkdown` headings + a `headingLink` helper; Contacts and IP Networks gain a Portal Link)
- Test: `internal/cache/snapshot_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/cache/snapshot_test.go`:

```go
func TestBuildMarkdownRendersHeadingLinks(t *testing.T) {
	snap := &Snapshot{
		Devices:    []itportal.Device{{ID: 9, Name: "fw01", URL: "https://portal.example/v4/app/devices/9"}},
		Contacts:   []itportal.Contact{{ID: 4, FirstName: "Ada", LastName: "Byte", URL: "https://portal.example/v4/app/contacts/4"}},
		IPNetworks: []itportal.IPNetwork{{ID: 3, Name: "LAN", URL: "https://portal.example/v4/app/ipnetworks/3"}},
	}
	md := buildMarkdown(snap)

	for _, want := range []string{
		"### [fw01](https://portal.example/v4/app/devices/9) (ID: 9)",
		"### [Ada Byte](https://portal.example/v4/app/contacts/4) (ID: 4)",
		"### [LAN](https://portal.example/v4/app/ipnetworks/3) (ID: 3)",
		"- **Portal Link**: https://portal.example/v4/app/contacts/4",
		"- **Portal Link**: https://portal.example/v4/app/ipnetworks/3",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestBuildMarkdownHeadingWithoutURLStaysPlain(t *testing.T) {
	snap := &Snapshot{Devices: []itportal.Device{{ID: 9, Name: "fw01"}}}
	md := buildMarkdown(snap)
	if !strings.Contains(md, "### fw01 (ID: 9)") {
		t.Errorf("plain heading missing; got:\n%s", md)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache/ -run TestBuildMarkdownRendersHeadingLinks -v`
Expected: FAIL — headings are still plain `### fw01 (ID: 9)`, Contacts/IP Networks have no Portal Link.

- [ ] **Step 3: Add the `headingLink` helper**

In `internal/cache/snapshot.go`, add near `formatAddress`:

```go
// headingLink renders name as a Markdown link when url is set, else plain name.
func headingLink(name, url string) string {
	if url == "" {
		return name
	}
	return "[" + name + "](" + url + ")"
}
```

- [ ] **Step 4: Wrap every entity heading name with `headingLink`**

In `buildMarkdown`, change each entity's `### ...` heading so the name is wrapped. Make these exact replacements:

Companies:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)\n", headingLink(co.Name, co.URL), co.ID)
```
Sites:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", headingLink(si.Name, si.URL), si.ID, companyCtx)
```
Devices:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", headingLink(d.Name, d.URL), d.ID, typeName, locationCtx)
```
Knowledge Base:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", headingLink(kb.Name, kb.URL), kb.ID, companyCtx)
```
Contacts (the name var is `fullName`):
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", headingLink(fullName, co.URL), co.ID, companyCtx)
```
Agreements (heading uses `Agreement ID: %d`):
```go
		fmt.Fprintf(&b, "### %s%s%s\n", headingLink(fmt.Sprintf("Agreement ID: %d", ag.ID), ag.URL), typeName, companyCtx)
```
IP Networks:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", headingLink(net.Name, net.URL), net.ID, companyCtx)
```
Documents (name var is `doc.Description`):
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", headingLink(doc.Description, doc.URL), doc.ID, typeName, companyCtx)
```
Accounts (heading uses a `heading` string built just above — replace the two lines that build and print it):
```go
			heading := fmt.Sprintf("Account ID: %d%s%s", ac.ID, typeName, companyCtx)
			fmt.Fprintf(&b, "### %s\n", headingLink(heading, ac.URL))
```
Facilities:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", headingLink(f.Name, f.URL), f.ID, typeName, companyCtx)
```
Cabinets:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", headingLink(cab.Name, cab.URL), cab.ID, companyCtx)
```
Configurations:
```go
		fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", headingLink(cfg.Name, cfg.URL), cfg.ID, typeName, companyCtx)
```

- [ ] **Step 5: Add the Portal Link bullet to Contacts and IP Networks**

Contacts section — before the final `b.WriteString("\n")` of the contact loop, add:
```go
		if co.URL != "" {
			fmt.Fprintf(&b, "- **Portal Link**: %s\n", co.URL)
		}
```

IP Networks section — before the final `b.WriteString("\n")` of the network loop, add:
```go
		if net.URL != "" {
			fmt.Fprintf(&b, "- **Portal Link**: %s\n", net.URL)
		}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/cache/ -run 'TestBuildMarkdown' -v`
Expected: PASS — `TestBuildMarkdownRendersHeadingLinks`, `TestBuildMarkdownHeadingWithoutURLStaysPlain`, and the existing `TestBuildMarkdownIncludesEntities` all green (the existing test checks substrings like `"fw01"` and `"Acme"`, which remain present inside the link text).

- [ ] **Step 7: Run the full cache suite**

Run: `go test ./internal/cache/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cache/snapshot.go internal/cache/snapshot_test.go
git commit -m "feat(cache): render entity headings as portal Markdown links"
```

---

## Task 4: Backfill live lookups and instruct the model

**Files:**
- Modify: `internal/mcp/tools.go` (`Handler` gets `baseURL`; add `marshalWithURL`; backfill in `GetEntityDetails` + `getDeviceDetails`)
- Modify: `internal/mcp/server.go` (set `Handler.baseURL`; add hyperlinking instruction)
- Test: `internal/mcp/tools_url_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcp/tools_url_test.go`:

```go
package mcp

import "testing"

func TestMarshalWithURLBackfillsEmpty(t *testing.T) {
	h := &Handler{baseURL: "https://portal.example"}

	empty := ""
	if _, _, err := h.marshalWithURL("device", 42, &empty, struct{}{}); err != nil {
		t.Fatalf("marshalWithURL error: %v", err)
	}
	if empty != "https://portal.example/v4/app/devices/42" {
		t.Errorf("empty url not backfilled: %q", empty)
	}

	given := "https://api-given/x"
	if _, _, err := h.marshalWithURL("device", 42, &given, struct{}{}); err != nil {
		t.Fatalf("marshalWithURL error: %v", err)
	}
	if given != "https://api-given/x" {
		t.Errorf("existing url overwritten: %q", given)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestMarshalWithURLBackfills -v`
Expected: FAIL — `Handler` has no `baseURL` field and `marshalWithURL` is undefined.

- [ ] **Step 3: Add the `baseURL` field to `Handler`**

In `internal/mcp/server.go`, change the `Handler` struct:

```go
type Handler struct {
	client  *itportal.Client
	cache   *cache.Cache
	baseURL string
}
```

And in `NewServer`, change the handler construction:

```go
	h := &Handler{client: client, cache: c, baseURL: client.BaseURL()}
```

- [ ] **Step 4: Add the `marshalWithURL` helper**

In `internal/mcp/tools.go`, add near `marshalResult`:

```go
// marshalWithURL backfills a constructed portal deep-link onto an entity whose
// API-provided url is empty, then marshals it. url must point at the entity's URL
// field so the backfill is reflected in the marshalled output.
func (h *Handler) marshalWithURL(itemType string, id int, url *string, v interface{}) (*sdkmcp.CallToolResult, any, error) {
	if *url == "" {
		*url = itportal.BuildPortalURL(h.baseURL, itemType, id)
	}
	return marshalResult(v)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestMarshalWithURLBackfills -v`
Expected: PASS.

- [ ] **Step 6: Backfill in `GetEntityDetails`**

In `internal/mcp/tools.go`, in `GetEntityDetails`, compute the normalised type once at the top of the function (right after the `id` empty-check) and switch on it:

```go
	norm := strings.ToLower(strings.ReplaceAll(input.EntityType, "_", ""))
	switch norm {
```

Then for each single-object case currently ending in `return marshalResult(v)`, change it to call `marshalWithURL`. The cases and their replacements:

```go
	case "company":
		v, err := h.client.GetCompany(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get company: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "site":
		v, err := h.client.GetSite(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get site: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "device":
		return h.getDeviceDetails(ctx, input.ID)
	case "kb", "knowledgebase":
		v, err := h.client.GetKB(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get KB: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "contact":
		v, err := h.client.GetContact(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get contact: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "account":
		v, err := h.client.GetAccount(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get account: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "agreement":
		v, err := h.client.GetAgreement(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get agreement: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "document":
		v, err := h.client.GetDocument(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get document: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "facility":
		v, err := h.client.GetFacility(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get facility: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "cabinet":
		v, err := h.client.GetCabinet(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get cabinet: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "configuration":
		v, err := h.client.GetConfiguration(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get configuration: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	case "ipnetwork":
		v, err := h.client.GetIPNetwork(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get IP network: %w", err)
		}
		return h.marshalWithURL(norm, v.ID, &v.URL, v)
	default:
		return toolError(fmt.Sprintf("unknown entity_type %q", input.EntityType)), nil, nil
	}
```

- [ ] **Step 7: Backfill the device detail path**

In `internal/mcp/tools.go`, in `getDeviceDetails`, after the device is fetched (right after the `GetDevice` error check), add:

```go
	if device.URL == "" {
		device.URL = itportal.BuildPortalURL(h.baseURL, "device", device.ID)
	}
```

- [ ] **Step 8: Add the hyperlinking instruction**

In `internal/mcp/server.go`, in the `instructions` string, add a new line at the end of the `Field conventions:` block (after the credentials bullet):

```
- Hyperlinking: when your answer mentions a specific documented object by name (company, site,
  device, KB article, contact, account, agreement, document, IP network, facility, cabinet or
  configuration), render the name as a Markdown link to its portal url — e.g.
  [CORE-SW-01](https://portal.example/v4/app/devices/42). Each object's link is in its snapshot
  heading and its "url" field; reuse it. Never invent a url, and never link an object that is not
  present in the snapshot or a tool result.
```

- [ ] **Step 9: Verify the whole module builds, vets, and tests pass**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build + vet clean; all tests PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/server.go internal/mcp/tools_url_test.go
git commit -m "feat(mcp): backfill portal links on lookups and instruct model to hyperlink objects"
```

---

## Self-Review notes

- **Spec coverage:** IPNetwork `url` field (T1), `BuildPortalURL`/segment helper + fallback construction (T1), `BaseURL()` accessor (T1), snapshot backfill incl. contacts/IP networks (T2), clickable headings + Portal Link bullets for contacts/IP networks (T3), `get_entity_details` backfill incl. device path (T4), server hyperlinking instruction (T4). `list_entities` left to API url + instruction — explicitly out of scope per spec.
- **Type consistency:** `marshalWithURL(itemType string, id int, url *string, v interface{})` defined in T4 Step 4, used in T4 Step 6/7. `headingLink(name, url string)` defined T3 Step 3, used T3 Step 4. `backfillPortalURLs(s *Snapshot, base string)` defined T2 Step 3, used T2 Step 5. `BuildPortalURL`/`PortalPathSegment` defined T1, used in T2/T4.
- **Note for the implementer:** `GetEntityDetails` GET methods return `*T` pointers, so `&v.URL` is addressable — `marshalWithURL` mutates the struct that gets marshalled. `strings` is already imported in `tools.go`.
