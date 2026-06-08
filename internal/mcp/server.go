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
	client  *itportal.Client
	cache   *cache.Cache
	baseURL string
}

// NewServer builds and configures the MCP server with all tools and resources.
func NewServer(client *itportal.Client, c *cache.Cache) *sdkmcp.Server {
	h := &Handler{client: client, cache: c, baseURL: client.BaseURL()}

	instructions := `You are an ITPortal documentation assistant for a Managed Service Provider, backed by
the ITPortal REST API v2.1.

You have access to:
1. A live documentation snapshot (itportal://snapshot) containing all documented companies, sites,
   devices, knowledge base articles, contacts, agreements, IP networks, documents, accounts,
   facilities, cabinets and configurations. Reading this resource once per conversation loads the
   entire environment into context and is cached — subsequent turns do not re-charge for those tokens.
2. Tools to search, query, create, update and delete documentation in real time.

Workflow for answering questions:
1. At the start of every conversation, read itportal://snapshot to load the full environment into
   context. Do this once — not on every query. The content is prompt-cached so the cost is minimal.
2. For follow-up targeted lookups within the same conversation, use search_docs to quickly find
   relevant sections without re-reading the full snapshot.
3. For specific entity sub-resources (IPs, notes, management URLs on a device), use get_entity_details.
4. The snapshot auto-refreshes periodically. Call refresh_snapshot if you need guaranteed fresh data
   mid-conversation, then re-read itportal://snapshot to update your context.

Tool guide:
- Read:    search_docs, list_entities, get_entity_details, get_logs, get_credentials.
- Create:  create_device, create_kb_article, create_entity (generic), add_device_ip, add_device_note,
           add_interaction, upload_file.
- Modify:  update_entity, delete_entity.
- Linking & files: manage_relationship (link two objects), manage_folder + manage_folder_file
           (per-object document trees), manage_credential (additional credentials).
- Admin:   manage_type (custom type lists), manage_kb_category (KB categories/subcategories).

Field conventions:
- Reference fields (company, site, type) use {"id": N} objects.
- Dates are YYYY-MM-DD strings.
- The "url" field on entities is a read-only portal deep-link, not editable.
- Relationship/credential targets use an itemType + id pair (e.g. {"itemType":"Device","id":42}).
- Credentials (passwords, 2FA) are never in the snapshot; read them explicitly with get_credentials.
- Hyperlinking: when your answer mentions a specific documented object by name (company, site,
  device, KB article, contact, account, agreement, document, IP network, facility, cabinet or
  configuration), render the name as a Markdown link to its portal url — e.g.
  [CORE-SW-01](https://portal.example/v4/app/devices/42). Each object's link is in its snapshot
  heading and its "url" field; reuse it. Never invent a url, and never link an object that is not
  present in the snapshot or a tool result.`

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "itportal-mcp",
		Version: "2.1.0",
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
		Description: "Force an immediate rebuild of the documentation snapshot from ITPortal. Use after making bulk changes or when you need guaranteed up-to-date data. The snapshot normally auto-refreshes on a schedule.",
	}, h.RefreshSnapshot)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_entity",
		Description: "Delete an entity by type and ID. Supports company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork, address, additional_credential and interaction. Deletes are permanent — confirm the target first.",
	}, h.DeleteEntity)

	// ---- v2.1: relationships, folders, files ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "manage_relationship",
		Description: "List, create, update or delete relationships (links) between two portal objects. Links are symmetric — a device↔document link appears from both sides. Use action=create with object_type/object_id as the source and target_type/target_id as the destination.",
	}, h.ManageRelationship)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "manage_folder",
		Description: "Manage the folder tree attached to an object (defaults to documents). Actions: list, get, create, update, delete. The first list call auto-creates Root_Folder; create child folders by passing parent_folder_id.",
	}, h.ManageFolder)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "manage_folder_file",
		Description: "Upload, list, download, rename or delete files inside an object's folder. Upload takes base64-encoded content; download returns base64. A folder cannot be deleted while it still contains files.",
	}, h.ManageFolderFile)

	// ---- v2.1: admin / metadata ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "manage_type",
		Description: "List, create, rename or delete the custom type lists used by entities (kinds: account, agreement, company, contact, device, document, facility, configuration). A type in use cannot be deleted.",
	}, h.ManageType)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "manage_kb_category",
		Description: "Manage knowledge-base categories and subcategories: list, create, update, delete, and create_subcategory/update_subcategory/delete_subcategory. A category containing articles cannot be deleted.",
	}, h.ManageKBCategory)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add_interaction",
		Description: "Add (or list) timeline interaction notes on an object. Valid object types: account, agreement, cabinet, configuration, contact, device, document, facility, ipnetwork, kb, site. Company/client is not supported.",
	}, h.AddInteraction)

	// ---- v2.1: credentials & logs ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "manage_credential",
		Description: "Create, read, update or delete additional credentials and attach them to any object via portal_object_type/portal_object_id. Handles secrets — only call when explicitly asked to store or change a credential.",
	}, h.ManageCredential)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_credentials",
		Description: "Retrieve the stored credentials (username/password/2FA) for an account, device or configuration. Returns secrets, so only call when the user explicitly needs them. Requires the server's encryption key for custom-encryption orgs.",
	}, h.GetCredentials)

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_logs",
		Description: "Query ITPortal audit logs: userAccess, adminAccess, loginLogout, passwordAccess, passwordChanges. Most require a start_date/end_date range (YYYY-MM-DD).",
	}, h.GetLogs)

	return server
}
