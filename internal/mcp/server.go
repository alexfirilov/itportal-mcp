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
the ITPortal REST API v2.1 and an embedded SQLite index of the documentation.

You have access to:
1. A COMPACT documentation index (itportal://snapshot) — one short line per object (type, id, name,
   one-line summary, portal url) across every documented company, site, device, KB article, contact,
   agreement, IP network, document, account, facility, cabinet and configuration. It is small by
   design and fits the tool-output limit. It is NOT the full environment — drill down for detail.
2. Per-section resources (itportal://snapshot/devices, /configurations, /accounts, …) that return
   the full rows of one section as paginated JSON (use ?offset= & ?limit= to page).
3. Tools to search, query, create, update and delete documentation in real time, backed by the
   SQLite index for fast, precise lookups.

Workflow for answering questions:
1. Read itportal://snapshot once to get the compact index of what exists (ids, names, summaries).
   Do NOT try to load one giant full-environment blob — there isn't one; that was the old anti-pattern.
2. To find specific objects, use search_docs(query[,entity_type]). It does exact lookups by IP
   address, serial number and name, and full-text keyword search — far more precise than scanning text.
3. For a full record (and, for devices, IPs/notes/management URLs), use get_entity_details(entity_type,id).
4. To enumerate a whole section, read the matching itportal://snapshot/<section> resource and page it.
5. The index auto-refreshes periodically. Call refresh_snapshot for guaranteed-fresh data, then
   re-read itportal://snapshot.

Tool guide:
- Read:    search_docs, list_entities, get_entity_details, get_logs, get_credentials.
- Create:  create_device, create_kb_article, create_entity (generic), add_device_ip, add_device_note,
           add_interaction, upload_file.
- Modify:  update_entity, delete_entity.
- Linking & files: manage_relationship (link two objects), manage_folder + manage_folder_file
           (per-object document trees), manage_credential (additional credentials).
- Switch ports: manage_switch_ports (a switch's Switch Ports tab — list/get/create/update/delete
           port ranges; per-port descriptions are read-only via the API, so record port notes in
           the range description).
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
	// itportal://snapshot — COMPACT index (default entry point). Small JSON: one
	// line per object. Drill down with search_docs / get_entity_details / sections.
	server.AddResource(&sdkmcp.Resource{
		Name: "ITPortal Documentation Index",
		Description: "COMPACT index of every documented object: type, id, name, one-line summary and portal " +
			"url. Small enough to fit the output limit — read this first to see what exists, then drill " +
			"down with search_docs and get_entity_details. NOT a full-environment dump. Supports " +
			"?type=device&limit=&offset= query params.",
		URI:      "itportal://snapshot",
		MIMEType: "application/json",
	}, h.IndexResource)

	// itportal://snapshot/<section> — full rows of one section, paginated JSON.
	sectionDescriptions := map[string]string{
		"companies":      "Full company records",
		"sites":          "Full site records",
		"devices":        "Full device records (hardware, serial, location)",
		"kbs":            "Knowledge base articles with content",
		"contacts":       "Full contact records",
		"agreements":     "Full agreement records",
		"ipnetworks":     "Full IP network records",
		"documents":      "Full document records",
		"accounts":       "Full account records (no passwords/2FA)",
		"facilities":     "Full facility records",
		"cabinets":       "Full cabinet records",
		"configurations": "Full configuration records",
	}
	for _, section := range sectionNames {
		server.AddResource(&sdkmcp.Resource{
			Name: "Snapshot section: " + section,
			Description: sectionDescriptions[section] + " as paginated JSON (default " +
				"100 rows; page with ?offset= & ?limit=).",
			URI:      "itportal://snapshot/" + section,
			MIMEType: "application/json",
		}, h.SectionResource)
	}

	// ---- Read tools ----

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "search_docs",
		Description: "Search the documentation via the embedded SQLite index. Resolves exact lookups by IP address, serial number and name, plus full-text keyword search over names, summaries, notes and identifiers. Returns compact hits (type, id, name, summary, portal url, match snippet) — drill into any with get_entity_details. Fast and token-efficient; does not hit the live API.",
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
		Description: "Create a new knowledge base article for a company. Use this to document procedures, configurations, troubleshooting guides or any other reference information. The 'description' field is a short synopsis; put the full note/document body in 'article' (HTML) or 'article_markdown' (Markdown, auto-converted).",
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
		Description: "Update (PATCH) an existing entity. Only include fields that should change. Reference fields use {\"id\": N} format. Entity types: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork, additional_credential. For kb, the note/document body is the 'article' field (HTML); pass 'article_markdown' instead to author in Markdown (auto-converted to article). 'description' is only the short synopsis.",
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
		Name:        "manage_switch_ports",
		Description: "Read and manage a switch's Switch Ports tab. Actions: list (all switch-port ranges for a device, each with its full nested port list — port numbers, per-port descriptions and device/IP assignments), get (one range by range_id), create (a new port range — needs name, starting_port, ending_port; ITPortal auto-provisions the ports), update (range fields incl. description), delete (a range). IMPORTANT: the ITPortal API only supports writing the RANGE container (name, port span, description, multiple_devices). Individual per-port descriptions and port-to-device assignments are READ-ONLY over the API and can only be edited in the ITPortal web UI — to record an uplink/port note when the per-port field isn't writable, put it in the range description.",
	}, h.ManageSwitchPorts)

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
