package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SnapshotResource serves the full documentation snapshot as Markdown.
// Reading this resource gives the LLM the entire documented environment in one
// shot; Anthropic prompt caching will cache it between calls when the content
// is unchanged.
func (h *Handler) SnapshotResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	snap := h.cache.Get()
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     snap.Markdown,
			},
		},
	}, nil
}

// CompaniesResource returns all cached companies as JSON.
func (h *Handler) CompaniesResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	snap := h.cache.Get()
	data, err := json.MarshalIndent(snap.Companies, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal companies: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// SitesResource returns all cached sites as JSON.
func (h *Handler) SitesResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	snap := h.cache.Get()
	data, err := json.MarshalIndent(snap.Sites, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sites: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// DevicesResource returns all cached devices as JSON.
func (h *Handler) DevicesResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	snap := h.cache.Get()
	data, err := json.MarshalIndent(snap.Devices, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal devices: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// KBsResource returns all cached KB articles as JSON.
func (h *Handler) KBsResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	snap := h.cache.Get()
	data, err := json.MarshalIndent(snap.KBs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal KBs: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

// ContactsResource returns all cached contacts as JSON.
func (h *Handler) ContactsResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	snap := h.cache.Get()
	data, err := json.MarshalIndent(snap.Contacts, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal contacts: %w", err)
	}
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}
