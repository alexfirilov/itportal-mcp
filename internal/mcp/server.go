// Package mcp wires together the ITPortal client, the documentation cache, and the
// MCP server. It registers all tools and resources and returns a ready-to-serve
// *mcp.Server.
package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alexfirilov/itportal-mcp/internal/cache"
	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// Handler bundles the shared dependencies injected into every tool/resource handler.
type Handler struct {
	client *itportal.Client
	cache  *cache.Cache
}

// NewServer builds and configures the MCP server with all tools and resources.
func NewServer(client *itportal.Client, c *cache.Cache) *sdkmcp.Server {
	h := &Handler{client: client, cache: c}

	instructions := `You are an ITPortal documentation assistant for a Managed Service Provider.

You have access to:
1. A live documentation snapshot (itportal://snapshot) containing all documented companies, sites,
   devices, knowledge base articles, contacts, agreements and IP networks. Reading this resource
   gives you the full picture of the client environment without extra API calls.
2. Tools to search, query, create and update documentation in real time.

Workflow for answering questions:
- For broad questions ("what devices does Acme have?"), use search_docs or list_entities with filters.
- For specific entity details (IPs, notes, management URLs on a device), use get_entity_details.
- The snapshot is refreshed every 30 minutes; call refresh_snapshot if you need guaranteed fresh data.

Workflow for documenting new information:
- Use create_device for hardware, create_kb_article for procedures/notes.
- After creation, optionally add IPs with add_device_ip and notes with add_device_note.
- To attach an image or config file, use upload_file with base64-encoded content.
- Use update_entity to correct or extend existing records.

Field conventions:
- Reference fields (company, site, type) use {"id": N} objects.
- Dates are YYYY-MM-DD strings.
- The "url" field on entities is a read-only portal deep-link, not editable.`

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "itportal-mcp",
		Version: "1.0.0",
	}, &sdkmcp.ServerOptions{
		Instructions: instructions,
	})

	// ---- Resources ----
	// itportal://snapshot  — full Markdown documentation (primary context caching target)
	server.AddResource(&sdkmcp.Resource{
		Name:        "ITPortal Documentation Snapshot",
		Description: "Full documentation snapshot: all companies, sites, devices, KB articles, contacts, agreements and IP networks in Markdown. Read this once to load the entire environment into context.",
		URI:         "itportal://snapshot",
		MIMEType:    "text/markdown",
	}, h.SnapshotResource)

	server.AddResource(&sdkmcp.Resource{
		Name:        "Companies",
		Description: "All documented companies as JSON",
		URI:         "itportal://companies",
		MIMEType:    "application/json",
	}, h.CompaniesResource)

	server.AddResource(&sdkmcp.Resource{
		Name:        "Sites",
		Description: "All documented sites as JSON",
		URI:         "itportal://sites",
		MIMEType:    "application/json",
	}, h.SitesResource)

	server.AddResource(&sdkmcp.Resource{
		Name:        "Devices",
		Description: "All documented devices as JSON",
		URI:         "itportal://devices",
		MIMEType:    "application/json",
	}, h.DevicesResource)

	server.AddResource(&sdkmcp.Resource{
		Name:        "Knowledge Base Articles",
		Description: "All KB articles as JSON",
		URI:         "itportal://kbs",
		MIMEType:    "application/json",
	}, h.KBsResource)

	server.AddResource(&sdkmcp.Resource{
		Name:        "Contacts",
		Description: "All documented contacts as JSON",
		URI:         "itportal://contacts",
		MIMEType:    "application/json",
	}, h.ContactsResource)

	// ---- Read tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "search_docs",
		Description: "Search the cached documentation snapshot for any keyword, company name, device name, IP address, serial number, or topic. Returns matching sections with context. Fast and token-efficient — searches local cache, not the live API.",
	}, h.SearchDocs)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_entities",
		Description: "List entities of a given type from ITPortal with optional filters. Returns paginated live results directly from the API. Use for targeted queries where snapshot search isn't precise enough.",
	}, h.ListEntities)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_entity_details",
		Description: "Fetch full details for a single entity by type and ID. For devices, also returns IP addresses, management URLs and notes. Use when you need complete structured data for a specific record.",
	}, h.GetEntityDetails)

	// ---- Write tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_kb_article",
		Description: "Create a new knowledge base article for a company. Use this to document procedures, configurations, troubleshooting guides or any other reference information.",
	}, h.CreateKBArticle)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_device",
		Description: "Create a new device record in ITPortal. Optionally adds a primary IP, management URL and an initial note in a single call. Use for onboarding new hardware.",
	}, h.CreateDevice)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_entity",
		Description: "Create any other entity type (company, site, contact, account, agreement, document, facility, cabinet, configuration, ip_network). Provide fields as a JSON object. Refer to the snapshot for field names and reference object structure.",
	}, h.CreateEntity)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_entity",
		Description: "Update (PATCH) an existing entity. Only include fields that should change. Reference fields use {\"id\": N} format. Entity types: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork, additional_credential.",
	}, h.UpdateEntity)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add_device_ip",
		Description: "Add an IP address record to an existing device. Optionally associates it with a MAC address, description and IP network.",
	}, h.AddDeviceIP)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add_device_note",
		Description: "Add a timestamped note to an existing device. Supports plain text or HTML.",
	}, h.AddDeviceNote)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "upload_file",
		Description: "Upload a file or image to an ITPortal entity. Accepts base64-encoded content. Useful for attaching network diagrams, screenshots, configuration files or contact photos.",
	}, h.UploadFile)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "refresh_snapshot",
		Description: "Force an immediate rebuild of the documentation snapshot from ITPortal. Use after making bulk changes or when you need guaranteed up-to-date data. The snapshot normally auto-refreshes every 30 minutes.",
	}, h.RefreshSnapshot)

	return server
}
