package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alexfirilov/itportal-mcp/internal/cache"
)

// defaultSectionPageSize caps how many full rows a section resource returns by
// default so the payload stays inside the consumer's tool-output limit. Callers
// page further with ?offset=N (and optional ?limit=N).
const defaultSectionPageSize = 100

// IndexResource serves the COMPACT documentation index: one short line per object
// (type, id, name, summary, portal url) across every entity. This is the default
// entry point — small enough to fit the output limit — from which the model drills
// down via get_entity_details / search_docs / the per-section resources.
func (h *Handler) IndexResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	store := h.cache.Store()
	if store == nil {
		return nil, fmt.Errorf("snapshot store not ready")
	}

	typ, limit, offset := parseQuery(req.Params.URI)
	rows, total, err := store.Index(typ, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}
	counts, err := store.Counts()
	if err != nil {
		return nil, fmt.Errorf("counts: %w", err)
	}

	payload := struct {
		GeneratedAt string            `json:"generated_at"`
		Counts      map[string]int    `json:"counts"`
		Total       int               `json:"total"`
		Returned    int               `json:"returned"`
		Offset      int               `json:"offset"`
		Sections    map[string]string `json:"sections"`
		Guidance    string            `json:"guidance"`
		Index       []cache.IndexRow  `json:"index"`
	}{
		GeneratedAt: h.cache.Get().GeneratedAt.Format("2006-01-02 15:04:05 UTC"),
		Counts:      counts,
		Total:       total,
		Returned:    len(rows),
		Offset:      offset,
		Sections:    sectionURIs(),
		Guidance: "Compact index of every documented object. Use search_docs(query[,entity_type]) " +
			"to find objects by keyword/IP/serial/name, get_entity_details(entity_type,id) for a full " +
			"record, and the itportal://snapshot/<section> resources for a paginated full section. " +
			"Do NOT expect a single full-environment blob — drill down instead.",
		Index: rows,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal index: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// SectionResource serves one entity section as paginated JSON rows (full columns,
// no secrets). The section is taken from the URI path, e.g.
// itportal://snapshot/devices, with optional ?offset=&limit= query params.
func (h *Handler) SectionResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	store := h.cache.Store()
	if store == nil {
		return nil, fmt.Errorf("snapshot store not ready")
	}

	section := sectionFromURI(req.Params.URI)
	_, limit, offset := parseQuery(req.Params.URI)
	if limit <= 0 {
		limit = defaultSectionPageSize
	}

	rows, total, err := store.SectionJSON(section, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("section %q: %w", section, err)
	}

	payload := struct {
		Section  string           `json:"section"`
		Total    int              `json:"total"`
		Returned int              `json:"returned"`
		Offset   int              `json:"offset"`
		Limit    int              `json:"limit"`
		NextPage string           `json:"next_page,omitempty"`
		Items    []map[string]any `json:"items"`
	}{
		Section:  section,
		Total:    total,
		Returned: len(rows),
		Offset:   offset,
		Limit:    limit,
		Items:    rows,
	}
	if offset+len(rows) < total {
		payload.NextPage = fmt.Sprintf("itportal://snapshot/%s?offset=%d&limit=%d", section, offset+limit, limit)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal section: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// sectionURIs returns the section name → resource URI map advertised in the index.
func sectionURIs() map[string]string {
	out := make(map[string]string, len(sectionNames))
	for _, s := range sectionNames {
		out[s] = "itportal://snapshot/" + s
	}
	return out
}

// sectionNames is the canonical ordered list of section resource names.
var sectionNames = []string{
	"companies", "sites", "devices", "kbs", "contacts", "agreements",
	"ipnetworks", "documents", "accounts", "facilities", "cabinets", "configurations",
}

// sectionFromURI extracts the trailing path segment of a snapshot section URI.
func sectionFromURI(uri string) string {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	uri = strings.TrimSuffix(uri, "/")
	if i := strings.LastIndexByte(uri, '/'); i >= 0 {
		return uri[i+1:]
	}
	return uri
}

// parseQuery pulls optional entity_type/type, limit and offset query parameters
// from a resource URI. Unknown or malformed values fall back to zero.
func parseQuery(uri string) (typ string, limit, offset int) {
	i := strings.IndexByte(uri, '?')
	if i < 0 {
		return "", 0, 0
	}
	q, err := url.ParseQuery(uri[i+1:])
	if err != nil {
		return "", 0, 0
	}
	typ = strings.ToLower(strings.ReplaceAll(firstQuery(q, "type", "entity_type"), "_", ""))
	limit = atoiQuery(q.Get("limit"))
	offset = atoiQuery(q.Get("offset"))
	return typ, limit, offset
}

func firstQuery(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func atoiQuery(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
