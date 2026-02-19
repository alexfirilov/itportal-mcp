package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// ---- Input types ----
// jsonschema struct tags are used by the go-sdk to auto-generate JSON Schema
// for each tool's input, which the LLM uses to correctly populate fields.

type SearchDocsInput struct {
	Query      string `json:"query" jsonschema:"Search query to find in the documentation snapshot. Supports partial matches."`
	EntityType string `json:"entity_type,omitempty" jsonschema:"Optional: restrict search to a section. Values: company, site, device, kb, contact, agreement, ipnetwork, document, account, facility, cabinet, configuration"`
}

type ListEntitiesInput struct {
	EntityType     string `json:"entity_type" jsonschema:"Required. One of: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork"`
	Name           string `json:"name,omitempty" jsonschema:"Filter by exact name"`
	NameStartsWith string `json:"name_starts_with,omitempty" jsonschema:"Filter by name prefix"`
	CompanyID      string `json:"company_id,omitempty" jsonschema:"Filter by company ID (for sites, devices, contacts, accounts, KBs, agreements)"`
	SiteID         string `json:"site_id,omitempty" jsonschema:"Filter by site ID (for devices, contacts)"`
	TypeName       string `json:"type_name,omitempty" jsonschema:"Filter by entity type name (e.g. 'Server', 'Managed Services')"`
	IPAddress      string `json:"ip_address,omitempty" jsonschema:"Filter devices by IP address"`
	SerialNumber   string `json:"serial_number,omitempty" jsonschema:"Filter devices by serial number"`
	Manufacturer   string `json:"manufacturer,omitempty" jsonschema:"Filter devices by manufacturer"`
	ModifiedSince  string `json:"modified_since,omitempty" jsonschema:"Return items modified since this date (ISO 8601 format: YYYY-MM-DD)"`
	Limit          int    `json:"limit,omitempty" jsonschema:"Max results to return. Default 50, max 500."`
	Offset         int    `json:"offset,omitempty" jsonschema:"Results to skip (for pagination)"`
}

type GetEntityInput struct {
	EntityType string `json:"entity_type" jsonschema:"One of: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork"`
	ID         string `json:"id" jsonschema:"The numeric ID of the entity"`
}

type CreateKBArticleInput struct {
	CompanyID   int    `json:"company_id" jsonschema:"ID of the company this article belongs to"`
	Name        string `json:"name" jsonschema:"Title of the knowledge base article"`
	Description string `json:"description,omitempty" jsonschema:"Full content of the article. HTML is supported."`
	CategoryID  int    `json:"category_id,omitempty" jsonschema:"KB category ID. Use list_entities with entity_type=kb_category to discover available categories."`
	Public      bool   `json:"public,omitempty" jsonschema:"Set true to make the article publicly visible (default: false)"`
	Expires     string `json:"expires,omitempty" jsonschema:"Expiration date in YYYY-MM-DD format"`
}

type CreateDeviceInput struct {
	CompanyID       int     `json:"company_id" jsonschema:"ID of the company this device belongs to (required)"`
	SiteID          int     `json:"site_id,omitempty" jsonschema:"ID of the site where this device is located"`
	Name            string  `json:"name" jsonschema:"Device hostname or display name (required)"`
	TypeName        string  `json:"type_name,omitempty" jsonschema:"Device type (e.g. Server, Router, Switch, Firewall, Workstation, Printer, Access Point)"`
	Description     string  `json:"description,omitempty" jsonschema:"Purpose or description of the device"`
	Manufacturer    string  `json:"manufacturer,omitempty" jsonschema:"Hardware manufacturer (e.g. Cisco, Fortinet, Dell, HP, Ubiquiti)"`
	Model           string  `json:"model,omitempty" jsonschema:"Model name or number"`
	Serial          string  `json:"serial,omitempty" jsonschema:"Serial number"`
	Tag             string  `json:"tag,omitempty" jsonschema:"Asset tag or internal tracking ID"`
	Location        string  `json:"location,omitempty" jsonschema:"Physical location (e.g. Server Room Rack 2, Reception Desk)"`
	Domain          string  `json:"domain,omitempty" jsonschema:"Domain or realm the device is joined to"`
	IMEI            string  `json:"imei,omitempty" jsonschema:"IMEI (for mobile devices)"`
	InstallDate     string  `json:"install_date,omitempty" jsonschema:"Installation date in YYYY-MM-DD format"`
	WarrantyExpires string  `json:"warranty_expires,omitempty" jsonschema:"Warranty expiry date in YYYY-MM-DD format"`
	PurchaseDate    string  `json:"purchase_date,omitempty" jsonschema:"Purchase date in YYYY-MM-DD format"`
	PurchasePrice   float64 `json:"purchase_price,omitempty" jsonschema:"Purchase price"`
	IPAddress       string  `json:"ip_address,omitempty" jsonschema:"Primary IP address to add (e.g. 192.168.1.100)"`
	MACAddress      string  `json:"mac_address,omitempty" jsonschema:"MAC address for the primary IP (e.g. 00:11:22:33:44:55)"`
	ManagementURL   string  `json:"management_url,omitempty" jsonschema:"Management interface URL (e.g. https://192.168.1.1)"`
	ManagementTitle string  `json:"management_url_title,omitempty" jsonschema:"Label for the management URL (e.g. Web Interface, SSH)"`
	InitialNote     string  `json:"initial_note,omitempty" jsonschema:"Initial note to attach to the device (plain text or HTML)"`
}

type CreateEntityInput struct {
	EntityType string                 `json:"entity_type" jsonschema:"Entity type: company, site, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork"`
	Fields     map[string]interface{} `json:"fields" jsonschema:"JSON object with entity fields. Reference the documentation snapshot for field names and structure. Reference fields use {\"id\": N} format."`
}

type UpdateEntityInput struct {
	EntityType string                 `json:"entity_type" jsonschema:"One of: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork, additional_credential"`
	ID         string                 `json:"id" jsonschema:"Numeric ID of the entity to update"`
	Fields     map[string]interface{} `json:"fields" jsonschema:"JSON object with only the fields to change. Unchanged fields can be omitted. Reference fields use {\"id\": N} format."`
}

type AddDeviceIPInput struct {
	DeviceID    string `json:"device_id" jsonschema:"ID of the device"`
	IP          string `json:"ip" jsonschema:"IP address to add (e.g. 10.0.0.50)"`
	MAC         string `json:"mac,omitempty" jsonschema:"MAC address (e.g. aa:bb:cc:dd:ee:ff)"`
	Description string `json:"description,omitempty" jsonschema:"Description (e.g. LAN Interface, iDRAC, Mgmt Port)"`
	IPNetworkID int    `json:"ip_network_id,omitempty" jsonschema:"ID of the IP Network this address belongs to"`
}

type AddDeviceNoteInput struct {
	DeviceID  string `json:"device_id" jsonschema:"ID of the device"`
	Notes     string `json:"notes" jsonschema:"Note content. Plain text or HTML (set notes_html true for HTML)."`
	NotesHTML bool   `json:"notes_html,omitempty" jsonschema:"Set true if notes is HTML content"`
}

type UploadFileInput struct {
	EntityType  string `json:"entity_type" jsonschema:"Target entity: device_config (device configuration file), kb (KB attachment), contact_photo (contact image), document_file (document), agreement_file (agreement)"`
	EntityID    string `json:"entity_id" jsonschema:"Numeric ID of the entity to attach the file to"`
	FileName    string `json:"file_name" jsonschema:"Filename with extension (e.g. network-diagram.png, config.txt)"`
	ContentType string `json:"content_type" jsonschema:"MIME type (e.g. image/png, image/jpeg, application/pdf, text/plain)"`
	Base64Data  string `json:"base64_data" jsonschema:"Base64-encoded file content"`
}

type RefreshSnapshotInput struct{}

// ---- Handler methods ----

// SearchDocs performs a case-insensitive full-text search across the cached documentation snapshot.
func (h *Handler) SearchDocs(ctx context.Context, _ *sdkmcp.CallToolRequest, input SearchDocsInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Query == "" {
		return toolError("query must not be empty"), nil, nil
	}

	snap := h.cache.Get()
	markdown := snap.Markdown

	// If an entity type filter was requested, narrow to that section.
	if input.EntityType != "" {
		section := sectionHeader(input.EntityType)
		if section != "" {
			start := strings.Index(markdown, "\n## "+section)
			if start != -1 {
				end := findNextSection(markdown, start+1)
				if end > start {
					markdown = markdown[start:end]
				}
			}
		}
	}

	// Collect matching lines with context.
	queryLower := strings.ToLower(input.Query)
	lines := strings.Split(markdown, "\n")
	var matches []string
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), queryLower) {
			// Include the heading above the matching line for context.
			start := max(0, i-1)
			end := min(len(lines), i+4)
			block := strings.Join(lines[start:end], "\n")
			matches = append(matches, block)
		}
	}

	if len(matches) == 0 {
		return toolText(fmt.Sprintf("No results found for %q in the documentation snapshot.\n\nSnapshot coverage: %d companies, %d sites, %d devices, %d KB articles, %d contacts, %d agreements, %d IP networks, %d documents, %d accounts, %d facilities, %d cabinets, %d configurations.",
			input.Query, len(snap.Companies), len(snap.Sites), len(snap.Devices), len(snap.KBs), len(snap.Contacts),
			len(snap.Agreements), len(snap.IPNetworks), len(snap.Documents), len(snap.Accounts),
			len(snap.Facilities), len(snap.Cabinets), len(snap.Configurations))), nil, nil
	}

	// Deduplicate blocks.
	seen := map[string]bool{}
	var unique []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}

	result := fmt.Sprintf("Found %d match(es) for %q:\n\n%s", len(unique), input.Query, strings.Join(unique, "\n---\n"))
	return toolText(result), nil, nil
}

// ListEntities lists entities of the given type from ITPortal with optional filters.
func (h *Handler) ListEntities(ctx context.Context, _ *sdkmcp.CallToolRequest, input ListEntitiesInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Limit <= 0 {
		input.Limit = 50
	}
	if input.Limit > 500 {
		input.Limit = 500
	}

	opts := &itportal.ListOptions{
		Name:           input.Name,
		NameStartsWith: input.NameStartsWith,
		CompanyID:      input.CompanyID,
		SiteID:         input.SiteID,
		TypeName:       input.TypeName,
		IPAddress:      input.IPAddress,
		SerialNumber:   input.SerialNumber,
		Manufacturer:   input.Manufacturer,
		ModifiedSince:  input.ModifiedSince,
		Limit:          input.Limit,
		Offset:         input.Offset,
	}

	type result struct {
		Total  int         `json:"total"`
		Offset int         `json:"offset"`
		Limit  int         `json:"limit"`
		Items  interface{} `json:"items"`
	}

	var items interface{}
	var total int

	switch strings.ToLower(strings.ReplaceAll(input.EntityType, "_", "")) {
	case "company":
		v, t, err := h.client.ListCompanies(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list companies: %w", err)
		}
		items, total = v, t
	case "site":
		v, t, err := h.client.ListSites(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list sites: %w", err)
		}
		items, total = v, t
	case "device":
		v, t, err := h.client.ListDevices(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list devices: %w", err)
		}
		items, total = v, t
	case "kb", "knowledgebase":
		v, t, err := h.client.ListKBs(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list KBs: %w", err)
		}
		items, total = v, t
	case "contact":
		v, t, err := h.client.ListContacts(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list contacts: %w", err)
		}
		items, total = v, t
	case "account":
		v, t, err := h.client.ListAccounts(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list accounts: %w", err)
		}
		items, total = v, t
	case "agreement":
		v, t, err := h.client.ListAgreements(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list agreements: %w", err)
		}
		items, total = v, t
	case "document":
		v, t, err := h.client.ListDocuments(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list documents: %w", err)
		}
		items, total = v, t
	case "facility":
		v, t, err := h.client.ListFacilities(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list facilities: %w", err)
		}
		items, total = v, t
	case "cabinet":
		v, t, err := h.client.ListCabinets(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list cabinets: %w", err)
		}
		items, total = v, t
	case "configuration":
		v, t, err := h.client.ListConfigurations(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list configurations: %w", err)
		}
		items, total = v, t
	case "ipnetwork":
		v, t, err := h.client.ListIPNetworks(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list IP networks: %w", err)
		}
		items, total = v, t
	case "kbcategory":
		v, err := h.client.ListKBCategories(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list KB categories: %w", err)
		}
		items, total = v, len(v)
	case "devicetype":
		v, err := h.client.ListDeviceTypes(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list device types: %w", err)
		}
		items, total = v, len(v)
	case "template":
		v, t, err := h.client.ListTemplates(ctx, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("list templates: %w", err)
		}
		items, total = v, t
	default:
		return toolError(fmt.Sprintf("unknown entity_type %q. Valid values: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork, kb_category, device_type, template", input.EntityType)), nil, nil
	}

	out, err := json.MarshalIndent(result{Total: total, Offset: input.Offset, Limit: input.Limit, Items: items}, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return toolText(string(out)), nil, nil
}

// GetEntityDetails fetches a single entity and, for devices, also fetches sub-resources.
func (h *Handler) GetEntityDetails(ctx context.Context, _ *sdkmcp.CallToolRequest, input GetEntityInput) (*sdkmcp.CallToolResult, any, error) {
	if input.ID == "" {
		return toolError("id must not be empty"), nil, nil
	}

	switch strings.ToLower(strings.ReplaceAll(input.EntityType, "_", "")) {
	case "company":
		v, err := h.client.GetCompany(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get company: %w", err)
		}
		return marshalResult(v)
	case "site":
		v, err := h.client.GetSite(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get site: %w", err)
		}
		return marshalResult(v)
	case "device":
		return h.getDeviceDetails(ctx, input.ID)
	case "kb", "knowledgebase":
		v, err := h.client.GetKB(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get KB: %w", err)
		}
		return marshalResult(v)
	case "contact":
		v, err := h.client.GetContact(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get contact: %w", err)
		}
		return marshalResult(v)
	case "account":
		v, err := h.client.GetAccount(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get account: %w", err)
		}
		return marshalResult(v)
	case "agreement":
		v, err := h.client.GetAgreement(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get agreement: %w", err)
		}
		return marshalResult(v)
	case "document":
		v, err := h.client.GetDocument(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get document: %w", err)
		}
		return marshalResult(v)
	case "facility":
		v, err := h.client.GetFacility(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get facility: %w", err)
		}
		return marshalResult(v)
	case "cabinet":
		v, err := h.client.GetCabinet(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get cabinet: %w", err)
		}
		return marshalResult(v)
	case "configuration":
		v, err := h.client.GetConfiguration(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get configuration: %w", err)
		}
		return marshalResult(v)
	case "ipnetwork":
		v, err := h.client.GetIPNetwork(ctx, input.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get IP network: %w", err)
		}
		return marshalResult(v)
	default:
		return toolError(fmt.Sprintf("unknown entity_type %q", input.EntityType)), nil, nil
	}
}

// getDeviceDetails fetches a device plus all its sub-resources (IPs, management URLs, notes).
func (h *Handler) getDeviceDetails(ctx context.Context, id string) (*sdkmcp.CallToolResult, any, error) {
	device, err := h.client.GetDevice(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get device: %w", err)
	}
	ips, err := h.client.GetDeviceIPs(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get device IPs: %w", err)
	}
	notes, err := h.client.GetDeviceNotes(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get device notes: %w", err)
	}
	mgmtURLs, err := h.client.GetDeviceManagementURLs(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get device management URLs: %w", err)
	}

	type deviceDetail struct {
		Device         *itportal.Device      `json:"device"`
		IPAddresses    []itportal.DeviceIP   `json:"ip_addresses"`
		Notes          []itportal.DeviceNote `json:"notes"`
		ManagementURLs []itportal.DeviceMUrl `json:"management_urls"`
	}
	detail := deviceDetail{
		Device:         device,
		IPAddresses:    ips,
		Notes:          notes,
		ManagementURLs: mgmtURLs,
	}
	return marshalResult(detail)
}

// CreateKBArticle creates a new knowledge base article.
func (h *Handler) CreateKBArticle(ctx context.Context, _ *sdkmcp.CallToolRequest, input CreateKBArticleInput) (*sdkmcp.CallToolResult, any, error) {
	if input.CompanyID == 0 {
		return toolError("company_id is required"), nil, nil
	}
	if input.Name == "" {
		return toolError("name is required"), nil, nil
	}

	kb := &itportal.KB{
		Name:        input.Name,
		Description: input.Description,
		Company:     &itportal.CompanyReference{ID: input.CompanyID},
		Public:      input.Public,
		Expires:     input.Expires,
	}
	if input.CategoryID != 0 {
		kb.Category = &itportal.KBCategory{ID: input.CategoryID}
	}

	created, err := h.client.CreateKB(ctx, kb)
	if err != nil {
		return nil, nil, fmt.Errorf("create KB article: %w", err)
	}
	return toolText(fmt.Sprintf("KB article created successfully.\nID: %d\nTitle: %s\nPortal: %s",
		created.ID, created.Name, created.URL)), nil, nil
}

// CreateDevice creates a device and optionally adds an IP, management URL, and initial note.
func (h *Handler) CreateDevice(ctx context.Context, _ *sdkmcp.CallToolRequest, input CreateDeviceInput) (*sdkmcp.CallToolResult, any, error) {
	if input.CompanyID == 0 {
		return toolError("company_id is required"), nil, nil
	}
	if input.Name == "" {
		return toolError("name is required"), nil, nil
	}

	device := &itportal.Device{
		Name:            input.Name,
		Company:         &itportal.CompanyReference{ID: input.CompanyID},
		Description:     input.Description,
		Manufacturer:    input.Manufacturer,
		Model:           input.Model,
		Serial:          input.Serial,
		Tag:             input.Tag,
		Location:        input.Location,
		Domain:          input.Domain,
		IMEI:            input.IMEI,
		InstallDate:     input.InstallDate,
		WarrantyExpires: input.WarrantyExpires,
		PurchaseDate:    input.PurchaseDate,
		PurchasePrice:   input.PurchasePrice,
	}
	if input.SiteID != 0 {
		device.Site = &itportal.SiteReference{ID: input.SiteID}
	}
	if input.TypeName != "" {
		device.Type = &itportal.TypeItem{Name: input.TypeName}
	}

	created, err := h.client.CreateDevice(ctx, device)
	if err != nil {
		return nil, nil, fmt.Errorf("create device: %w", err)
	}

	var sideEffects []string
	devIDStr := strconv.Itoa(created.ID)

	if input.IPAddress != "" {
		ip := &itportal.DeviceIP{
			IP:  input.IPAddress,
			MAC: input.MACAddress,
		}
		if _, err := h.client.AddDeviceIP(ctx, devIDStr, ip); err != nil {
			sideEffects = append(sideEffects, fmt.Sprintf("⚠ Could not add IP %s: %v", input.IPAddress, err))
		} else {
			sideEffects = append(sideEffects, fmt.Sprintf("✓ IP added: %s", input.IPAddress))
		}
	}

	if input.ManagementURL != "" {
		title := input.ManagementTitle
		if title == "" {
			title = "Management Interface"
		}
		murl := &itportal.DeviceMUrl{Title: title, URL: input.ManagementURL}
		if _, err := h.client.AddDeviceManagementURL(ctx, devIDStr, murl); err != nil {
			sideEffects = append(sideEffects, fmt.Sprintf("⚠ Could not add management URL: %v", err))
		} else {
			sideEffects = append(sideEffects, fmt.Sprintf("✓ Management URL added: %s", input.ManagementURL))
		}
	}

	if input.InitialNote != "" {
		note := &itportal.DeviceNote{Notes: input.InitialNote}
		if _, err := h.client.AddDeviceNote(ctx, devIDStr, note); err != nil {
			sideEffects = append(sideEffects, fmt.Sprintf("⚠ Could not add note: %v", err))
		} else {
			sideEffects = append(sideEffects, "✓ Initial note added")
		}
	}

	msg := fmt.Sprintf("Device created successfully.\nID: %d\nName: %s\nPortal: %s",
		created.ID, created.Name, created.URL)
	if len(sideEffects) > 0 {
		msg += "\n\n" + strings.Join(sideEffects, "\n")
	}
	return toolText(msg), nil, nil
}

// CreateEntity creates any supported entity type from a generic fields map.
func (h *Handler) CreateEntity(ctx context.Context, _ *sdkmcp.CallToolRequest, input CreateEntityInput) (*sdkmcp.CallToolResult, any, error) {
	if input.EntityType == "" {
		return toolError("entity_type is required"), nil, nil
	}
	if len(input.Fields) == 0 {
		return toolError("fields must not be empty"), nil, nil
	}

	// Re-marshal fields to the appropriate concrete type.
	fieldsJSON, err := json.Marshal(input.Fields)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal fields: %w", err)
	}

	type createResult struct {
		ID  int    `json:"id"`
		URL string `json:"url,omitempty"`
	}

	unmarshalAndCreate := func(target interface{}, createFn func() (int, string, error)) (*sdkmcp.CallToolResult, any, error) {
		if err := json.Unmarshal(fieldsJSON, target); err != nil {
			return toolError(fmt.Sprintf("invalid fields for %s: %v", input.EntityType, err)), nil, nil
		}
		id, url, err := createFn()
		if err != nil {
			return nil, nil, err
		}
		return toolText(fmt.Sprintf("%s created. ID: %d  Portal: %s", input.EntityType, id, url)), nil, nil
	}

	switch strings.ToLower(strings.ReplaceAll(input.EntityType, "_", "")) {
	case "company":
		var v itportal.Company
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateCompany(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create company: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "site":
		var v itportal.Site
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateSite(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create site: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "contact":
		var v itportal.Contact
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateContact(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create contact: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "account":
		var v itportal.Account
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateAccount(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create account: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "agreement":
		var v itportal.Agreement
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateAgreement(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create agreement: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "document":
		var v itportal.Document
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateDocument(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create document: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "ipnetwork":
		var v itportal.IPNetwork
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateIPNetwork(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create IP network: %w", err)
			}
			return created.ID, "", nil
		})
	case "facility":
		var v itportal.Facility
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateFacility(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create facility: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "cabinet":
		var v itportal.Cabinet
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateCabinet(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create cabinet: %w", err)
			}
			return created.ID, created.URL, nil
		})
	case "configuration":
		var v itportal.Configuration
		return unmarshalAndCreate(&v, func() (int, string, error) {
			created, err := h.client.CreateConfiguration(ctx, &v)
			if err != nil {
				return 0, "", fmt.Errorf("create configuration: %w", err)
			}
			return created.ID, created.URL, nil
		})
	default:
		return toolError(fmt.Sprintf("entity_type %q is not supported for create_entity. Use create_device or create_kb_article for those types.", input.EntityType)), nil, nil
	}
}

// UpdateEntity patches an existing entity with the given fields.
func (h *Handler) UpdateEntity(ctx context.Context, _ *sdkmcp.CallToolRequest, input UpdateEntityInput) (*sdkmcp.CallToolResult, any, error) {
	if input.ID == "" {
		return toolError("id is required"), nil, nil
	}
	if len(input.Fields) == 0 {
		return toolError("fields must not be empty"), nil, nil
	}

	var err error
	switch strings.ToLower(strings.ReplaceAll(input.EntityType, "_", "")) {
	case "company":
		err = h.client.UpdateCompany(ctx, input.ID, input.Fields)
	case "site":
		err = h.client.UpdateSite(ctx, input.ID, input.Fields)
	case "device":
		err = h.client.UpdateDevice(ctx, input.ID, input.Fields)
	case "kb", "knowledgebase":
		err = h.client.UpdateKB(ctx, input.ID, input.Fields)
	case "contact":
		err = h.client.UpdateContact(ctx, input.ID, input.Fields)
	case "account":
		err = h.client.UpdateAccount(ctx, input.ID, input.Fields)
	case "agreement":
		err = h.client.UpdateAgreement(ctx, input.ID, input.Fields)
	case "document":
		err = h.client.UpdateDocument(ctx, input.ID, input.Fields)
	case "facility":
		err = h.client.UpdateFacility(ctx, input.ID, input.Fields)
	case "cabinet":
		err = h.client.UpdateCabinet(ctx, input.ID, input.Fields)
	case "configuration":
		err = h.client.UpdateConfiguration(ctx, input.ID, input.Fields)
	case "ipnetwork":
		err = h.client.UpdateIPNetwork(ctx, input.ID, input.Fields)
	case "additionalcredential":
		err = h.client.UpdateAdditionalCredential(ctx, input.ID, input.Fields)
	default:
		return toolError(fmt.Sprintf("unknown entity_type %q for update", input.EntityType)), nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("update %s %s: %w", input.EntityType, input.ID, err)
	}
	return toolText(fmt.Sprintf("%s ID %s updated successfully.", input.EntityType, input.ID)), nil, nil
}

// AddDeviceIP adds an IP address record to a device.
func (h *Handler) AddDeviceIP(ctx context.Context, _ *sdkmcp.CallToolRequest, input AddDeviceIPInput) (*sdkmcp.CallToolResult, any, error) {
	if input.DeviceID == "" {
		return toolError("device_id is required"), nil, nil
	}
	if input.IP == "" {
		return toolError("ip is required"), nil, nil
	}

	ip := &itportal.DeviceIP{
		IP:          input.IP,
		MAC:         input.MAC,
		Description: input.Description,
	}
	if input.IPNetworkID != 0 {
		ip.IPNetwork = &itportal.IPNetworkReference{ID: input.IPNetworkID}
	}

	created, err := h.client.AddDeviceIP(ctx, input.DeviceID, ip)
	if err != nil {
		return nil, nil, fmt.Errorf("add device IP: %w", err)
	}
	return toolText(fmt.Sprintf("IP %s added to device %s (IP record ID: %d).", created.IP, input.DeviceID, created.ID)), nil, nil
}

// AddDeviceNote adds a timestamped note to a device.
func (h *Handler) AddDeviceNote(ctx context.Context, _ *sdkmcp.CallToolRequest, input AddDeviceNoteInput) (*sdkmcp.CallToolResult, any, error) {
	if input.DeviceID == "" {
		return toolError("device_id is required"), nil, nil
	}
	if input.Notes == "" {
		return toolError("notes must not be empty"), nil, nil
	}

	note := &itportal.DeviceNote{
		Notes:     input.Notes,
		NotesHtml: input.NotesHTML,
	}
	created, err := h.client.AddDeviceNote(ctx, input.DeviceID, note)
	if err != nil {
		return nil, nil, fmt.Errorf("add device note: %w", err)
	}
	return toolText(fmt.Sprintf("Note added to device %s (note ID: %d).", input.DeviceID, created.ID)), nil, nil
}

// UploadFile decodes a base64 payload and uploads it to an ITPortal entity.
func (h *Handler) UploadFile(ctx context.Context, _ *sdkmcp.CallToolRequest, input UploadFileInput) (*sdkmcp.CallToolResult, any, error) {
	if input.EntityID == "" {
		return toolError("entity_id is required"), nil, nil
	}
	if input.FileName == "" {
		return toolError("file_name is required"), nil, nil
	}
	if input.Base64Data == "" {
		return toolError("base64_data is required"), nil, nil
	}

	fileData, err := base64.StdEncoding.DecodeString(input.Base64Data)
	if err != nil {
		// Try URL-safe base64 as fallback.
		fileData, err = base64.URLEncoding.DecodeString(input.Base64Data)
		if err != nil {
			return toolError(fmt.Sprintf("base64_data is not valid base64: %v", err)), nil, nil
		}
	}

	var uploadPath string
	switch strings.ToLower(strings.ReplaceAll(input.EntityType, "_", "")) {
	case "deviceconfig":
		uploadPath = fmt.Sprintf("/api/2.0/devices/%s/configurationFiles/", input.EntityID)
	case "kb":
		uploadPath = fmt.Sprintf("/api/2.0/kbs/%s/file/", input.EntityID)
	case "contactphoto":
		uploadPath = fmt.Sprintf("/api/2.0/contacts/%s/file/", input.EntityID)
	case "documentfile":
		uploadPath = fmt.Sprintf("/api/2.0/documents/%s/file/", input.EntityID)
	case "agreementfile":
		uploadPath = fmt.Sprintf("/api/2.0/agreements/%s/file/", input.EntityID)
	default:
		return toolError(fmt.Sprintf("unknown entity_type %q for upload. Valid values: device_config, kb, contact_photo, document_file, agreement_file", input.EntityType)), nil, nil
	}

	if err := h.client.UploadFile(ctx, uploadPath, input.FileName, input.ContentType, fileData); err != nil {
		return nil, nil, fmt.Errorf("upload file to %s: %w", uploadPath, err)
	}
	return toolText(fmt.Sprintf("File %q (%d bytes) uploaded to %s ID %s.", input.FileName, len(fileData), input.EntityType, input.EntityID)), nil, nil
}

// RefreshSnapshot forces an immediate documentation snapshot rebuild.
func (h *Handler) RefreshSnapshot(ctx context.Context, _ *sdkmcp.CallToolRequest, _ RefreshSnapshotInput) (*sdkmcp.CallToolResult, any, error) {
	snap, err := h.cache.Refresh(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("refresh snapshot: %w", err)
	}
	return toolText(fmt.Sprintf(
		"Snapshot refreshed at %s UTC.\nCompanies: %d · Sites: %d · Devices: %d · KB articles: %d · Contacts: %d · Agreements: %d · IP networks: %d · Documents: %d · Accounts: %d · Facilities: %d · Cabinets: %d · Configurations: %d",
		snap.GeneratedAt.Format("2006-01-02 15:04:05"),
		len(snap.Companies), len(snap.Sites), len(snap.Devices),
		len(snap.KBs), len(snap.Contacts), len(snap.Agreements), len(snap.IPNetworks),
		len(snap.Documents), len(snap.Accounts), len(snap.Facilities), len(snap.Cabinets), len(snap.Configurations),
	)), nil, nil
}

// ---- Helpers ----

func toolText(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}
}

func toolError(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}
}

func marshalResult(v interface{}) (*sdkmcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return toolText(string(data)), nil, nil
}

// sectionHeader returns the markdown section heading for an entity type filter.
func sectionHeader(entityType string) string {
	switch strings.ToLower(strings.ReplaceAll(entityType, "_", "")) {
	case "company":
		return "Companies"
	case "site":
		return "Sites"
	case "device":
		return "Devices"
	case "kb", "knowledgebase":
		return "Knowledge Base Articles"
	case "contact":
		return "Contacts"
	case "agreement":
		return "Agreements"
	case "ipnetwork":
		return "IP Networks"
	case "document":
		return "Documents"
	case "account":
		return "Accounts"
	case "facility":
		return "Facilities"
	case "cabinet":
		return "Cabinets"
	case "configuration":
		return "Configurations"
	}
	return ""
}

// findNextSection returns the index of the next top-level section (##) after start.
func findNextSection(s string, start int) int {
	idx := strings.Index(s[start:], "\n## ")
	if idx == -1 {
		return len(s)
	}
	return start + idx
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
