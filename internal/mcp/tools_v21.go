package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// normType normalises a friendly entity-type string to a comparable key.
func normType(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "_", ""))
}

// objectPathFor maps a friendly entity type to its REST collection segment,
// used to build per-object sub-resource paths (relationships, folders, …).
func objectPathFor(entityType string) (string, bool) {
	switch normType(entityType) {
	case "company", "companies":
		return "companies", true
	case "site", "sites":
		return "sites", true
	case "device", "devices":
		return "devices", true
	case "kb", "kbs", "knowledgebase":
		return "kbs", true
	case "contact", "contacts":
		return "contacts", true
	case "account", "accounts":
		return "accounts", true
	case "agreement", "agreements":
		return "agreements", true
	case "document", "documents":
		return "documents", true
	case "facility", "facilities":
		return "facilities", true
	case "cabinet", "cabinets":
		return "cabinets", true
	case "configuration", "configurations":
		return "configurations", true
	case "ipnetwork", "ipnetworks", "subnet":
		return "ipnetworks", true
	}
	return "", false
}

// ---- delete_entity ----

type DeleteEntityInput struct {
	EntityType string `json:"entity_type" jsonschema:"One of: company, site, device, kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork, address, additional_credential, interaction"`
	ID         string `json:"id" jsonschema:"Numeric ID of the entity to delete"`
}

func (h *Handler) DeleteEntity(ctx context.Context, _ *sdkmcp.CallToolRequest, input DeleteEntityInput) (*sdkmcp.CallToolResult, any, error) {
	if input.ID == "" {
		return toolError("id is required"), nil, nil
	}
	var err error
	switch normType(input.EntityType) {
	case "company":
		err = h.client.DeleteCompany(ctx, input.ID)
	case "site":
		err = h.client.DeleteSite(ctx, input.ID)
	case "device":
		err = h.client.DeleteDevice(ctx, input.ID)
	case "kb", "knowledgebase":
		err = h.client.DeleteKB(ctx, input.ID)
	case "contact":
		err = h.client.DeleteContact(ctx, input.ID)
	case "account":
		err = h.client.DeleteAccount(ctx, input.ID)
	case "agreement":
		err = h.client.DeleteAgreement(ctx, input.ID)
	case "document":
		err = h.client.DeleteDocument(ctx, input.ID)
	case "facility":
		err = h.client.DeleteFacility(ctx, input.ID)
	case "cabinet":
		err = h.client.DeleteCabinet(ctx, input.ID)
	case "configuration":
		err = h.client.DeleteConfiguration(ctx, input.ID)
	case "ipnetwork":
		err = h.client.DeleteIPNetwork(ctx, input.ID)
	case "address":
		err = h.client.DeleteAddress(ctx, input.ID)
	case "additionalcredential":
		err = h.client.DeleteAdditionalCredential(ctx, input.ID)
	case "interaction":
		err = h.client.DeleteInteraction(ctx, input.ID)
	default:
		return toolError(fmt.Sprintf("unknown or non-deletable entity_type %q", input.EntityType)), nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("delete %s %s: %w", input.EntityType, input.ID, err)
	}
	return toolText(fmt.Sprintf("%s ID %s deleted.", input.EntityType, input.ID)), nil, nil
}

// ---- manage_relationship ----

type ManageRelationshipInput struct {
	Action     string `json:"action" jsonschema:"One of: list, get, create, update, delete"`
	ObjectType string `json:"object_type" jsonschema:"Source object type: device, document, configuration, account, agreement, contact, facility, cabinet, site, kb, ipnetwork, company"`
	ObjectID   string `json:"object_id" jsonschema:"Numeric ID of the source object"`
	LinkID     string `json:"link_id,omitempty" jsonschema:"Relationship (link) ID — required for get/update/delete"`
	TargetType string `json:"target_type,omitempty" jsonschema:"Target itemType for create. Capitalised singular, e.g. Device, Document, Configuration, Account, Agreement, Contact, Facility, Cabinet, Site, KB, Subnet"`
	TargetID   int    `json:"target_id,omitempty" jsonschema:"Numeric ID of the target object — required for create"`
	Notes      string `json:"notes,omitempty" jsonschema:"Optional note describing the relationship"`
}

func (h *Handler) ManageRelationship(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageRelationshipInput) (*sdkmcp.CallToolResult, any, error) {
	objPath, ok := objectPathFor(input.ObjectType)
	if !ok {
		return toolError(fmt.Sprintf("unknown object_type %q", input.ObjectType)), nil, nil
	}
	if input.ObjectID == "" {
		return toolError("object_id is required"), nil, nil
	}

	switch strings.ToLower(input.Action) {
	case "list", "":
		rels, err := h.client.ListRelationships(ctx, objPath, input.ObjectID)
		if err != nil {
			return nil, nil, fmt.Errorf("list relationships: %w", err)
		}
		return marshalResult(rels)
	case "get":
		if input.LinkID == "" {
			return toolError("link_id is required for get"), nil, nil
		}
		rel, err := h.client.GetRelationship(ctx, objPath, input.ObjectID, input.LinkID)
		if err != nil {
			return nil, nil, fmt.Errorf("get relationship: %w", err)
		}
		return marshalResult(rel)
	case "create":
		if input.TargetType == "" || input.TargetID == 0 {
			return toolError("target_type and target_id are required for create"), nil, nil
		}
		rel := &itportal.Relationship{
			Target: &itportal.RelationshipTarget{ItemType: input.TargetType, ID: input.TargetID},
			Notes:  input.Notes,
		}
		created, err := h.client.CreateRelationship(ctx, objPath, input.ObjectID, rel)
		if err != nil {
			return nil, nil, fmt.Errorf("create relationship: %w", err)
		}
		return toolText(fmt.Sprintf("Relationship created (link ID: %d): %s %s ↔ %s %d.",
			created.ID, input.ObjectType, input.ObjectID, input.TargetType, input.TargetID)), nil, nil
	case "update":
		if input.LinkID == "" {
			return toolError("link_id is required for update"), nil, nil
		}
		if err := h.client.UpdateRelationship(ctx, objPath, input.ObjectID, input.LinkID, map[string]interface{}{"notes": input.Notes}); err != nil {
			return nil, nil, fmt.Errorf("update relationship: %w", err)
		}
		return toolText(fmt.Sprintf("Relationship %s updated.", input.LinkID)), nil, nil
	case "delete":
		if input.LinkID == "" {
			return toolError("link_id is required for delete"), nil, nil
		}
		if err := h.client.DeleteRelationship(ctx, objPath, input.ObjectID, input.LinkID); err != nil {
			return nil, nil, fmt.Errorf("delete relationship: %w", err)
		}
		return toolText(fmt.Sprintf("Relationship %s deleted.", input.LinkID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q (use list, get, create, update, delete)", input.Action)), nil, nil
	}
}

// ---- manage_folder ----

type ManageFolderInput struct {
	Action         string `json:"action" jsonschema:"One of: list, get, create, update, delete"`
	ObjectType     string `json:"object_type,omitempty" jsonschema:"Object whose folder tree to manage (default: document)"`
	ObjectID       string `json:"object_id" jsonschema:"Numeric ID of the object. The first list call auto-creates Root_Folder."`
	FolderID       string `json:"folder_id,omitempty" jsonschema:"Folder ID — required for get/update/delete"`
	Name           string `json:"name,omitempty" jsonschema:"Folder name — required for create"`
	Description    string `json:"description,omitempty" jsonschema:"Folder description"`
	ParentFolderID int    `json:"parent_folder_id,omitempty" jsonschema:"Parent folder ID (use the Root_Folder ID for top-level folders)"`
}

func (h *Handler) ManageFolder(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageFolderInput) (*sdkmcp.CallToolResult, any, error) {
	objType := input.ObjectType
	if objType == "" {
		objType = "document"
	}
	objPath, ok := objectPathFor(objType)
	if !ok {
		return toolError(fmt.Sprintf("unknown object_type %q", objType)), nil, nil
	}
	if input.ObjectID == "" {
		return toolError("object_id is required"), nil, nil
	}

	switch strings.ToLower(input.Action) {
	case "list", "":
		folders, err := h.client.ListFolders(ctx, objPath, input.ObjectID)
		if err != nil {
			return nil, nil, fmt.Errorf("list folders: %w", err)
		}
		return marshalResult(folders)
	case "get":
		if input.FolderID == "" {
			return toolError("folder_id is required for get"), nil, nil
		}
		folder, err := h.client.GetFolder(ctx, objPath, input.ObjectID, input.FolderID)
		if err != nil {
			return nil, nil, fmt.Errorf("get folder: %w", err)
		}
		return marshalResult(folder)
	case "create":
		if input.Name == "" {
			return toolError("name is required for create"), nil, nil
		}
		folder := &itportal.Folder{Name: input.Name, Description: input.Description, ParentFolderID: input.ParentFolderID}
		created, err := h.client.CreateFolder(ctx, objPath, input.ObjectID, folder)
		if err != nil {
			return nil, nil, fmt.Errorf("create folder: %w", err)
		}
		return toolText(fmt.Sprintf("Folder %q created (ID: %d).", input.Name, created.ID)), nil, nil
	case "update":
		if input.FolderID == "" {
			return toolError("folder_id is required for update"), nil, nil
		}
		fields := map[string]interface{}{}
		if input.Name != "" {
			fields["name"] = input.Name
		}
		if input.Description != "" {
			fields["description"] = input.Description
		}
		if err := h.client.UpdateFolder(ctx, objPath, input.ObjectID, input.FolderID, fields); err != nil {
			return nil, nil, fmt.Errorf("update folder: %w", err)
		}
		return toolText(fmt.Sprintf("Folder %s updated.", input.FolderID)), nil, nil
	case "delete":
		if input.FolderID == "" {
			return toolError("folder_id is required for delete"), nil, nil
		}
		if err := h.client.DeleteFolder(ctx, objPath, input.ObjectID, input.FolderID); err != nil {
			return nil, nil, fmt.Errorf("delete folder: %w", err)
		}
		return toolText(fmt.Sprintf("Folder %s deleted.", input.FolderID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q", input.Action)), nil, nil
	}
}

// ---- manage_folder_file ----

type ManageFolderFileInput struct {
	Action      string `json:"action" jsonschema:"One of: list, upload, download, update, delete"`
	ObjectType  string `json:"object_type,omitempty" jsonschema:"Object that owns the folder (default: document)"`
	ObjectID    string `json:"object_id" jsonschema:"Numeric ID of the object"`
	FolderID    string `json:"folder_id" jsonschema:"Folder ID the file lives in"`
	FileID      string `json:"file_id,omitempty" jsonschema:"File ID — required for download/update/delete"`
	FileName    string `json:"file_name,omitempty" jsonschema:"Filename with extension — required for upload"`
	ContentType string `json:"content_type,omitempty" jsonschema:"MIME type for upload (e.g. application/pdf)"`
	Base64Data  string `json:"base64_data,omitempty" jsonschema:"Base64-encoded file content — required for upload"`
	Description string `json:"description,omitempty" jsonschema:"File description (upload/update)"`
}

func (h *Handler) ManageFolderFile(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageFolderFileInput) (*sdkmcp.CallToolResult, any, error) {
	objType := input.ObjectType
	if objType == "" {
		objType = "document"
	}
	objPath, ok := objectPathFor(objType)
	if !ok {
		return toolError(fmt.Sprintf("unknown object_type %q", objType)), nil, nil
	}
	if input.ObjectID == "" || input.FolderID == "" {
		return toolError("object_id and folder_id are required"), nil, nil
	}

	switch strings.ToLower(input.Action) {
	case "list", "":
		files, err := h.client.ListFolderFiles(ctx, objPath, input.ObjectID, input.FolderID)
		if err != nil {
			return nil, nil, fmt.Errorf("list folder files: %w", err)
		}
		return marshalResult(files)
	case "upload":
		if input.FileName == "" || input.Base64Data == "" {
			return toolError("file_name and base64_data are required for upload"), nil, nil
		}
		data, err := decodeBase64(input.Base64Data)
		if err != nil {
			return toolError(err.Error()), nil, nil
		}
		id, err := h.client.UploadFolderFile(ctx, objPath, input.ObjectID, input.FolderID, input.FileName, input.ContentType, data, input.Description)
		if err != nil {
			return nil, nil, fmt.Errorf("upload folder file: %w", err)
		}
		return toolText(fmt.Sprintf("Uploaded %q (%d bytes) to folder %s (file ID: %d).", input.FileName, len(data), input.FolderID, id)), nil, nil
	case "download":
		if input.FileID == "" {
			return toolError("file_id is required for download"), nil, nil
		}
		raw, err := h.client.DownloadFolderFile(ctx, objPath, input.ObjectID, input.FolderID, input.FileID)
		if err != nil {
			return nil, nil, fmt.Errorf("download folder file: %w", err)
		}
		return toolText(fmt.Sprintf("File %s (%d bytes), base64:\n%s", input.FileID, len(raw), base64.StdEncoding.EncodeToString(raw))), nil, nil
	case "update":
		if input.FileID == "" {
			return toolError("file_id is required for update"), nil, nil
		}
		fields := map[string]interface{}{}
		if input.FileName != "" {
			fields["fileName"] = input.FileName
		}
		if input.Description != "" {
			fields["description"] = input.Description
		}
		if err := h.client.UpdateFolderFile(ctx, objPath, input.ObjectID, input.FolderID, input.FileID, fields); err != nil {
			return nil, nil, fmt.Errorf("update folder file: %w", err)
		}
		return toolText(fmt.Sprintf("File %s updated.", input.FileID)), nil, nil
	case "delete":
		if input.FileID == "" {
			return toolError("file_id is required for delete"), nil, nil
		}
		if err := h.client.DeleteFolderFile(ctx, objPath, input.ObjectID, input.FolderID, input.FileID); err != nil {
			return nil, nil, fmt.Errorf("delete folder file: %w", err)
		}
		return toolText(fmt.Sprintf("File %s deleted.", input.FileID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q", input.Action)), nil, nil
	}
}

// ---- manage_switch_ports ----

type ManageSwitchPortsInput struct {
	Action          string `json:"action" jsonschema:"One of: list, get, create, update, delete"`
	DeviceID        string `json:"device_id" jsonschema:"Numeric ID of the switch device"`
	RangeID         string `json:"range_id,omitempty" jsonschema:"Switch-port-range ID — required for get/update/delete"`
	Name            string `json:"name,omitempty" jsonschema:"Range name, e.g. \"RJ\" — required for create"`
	Description     string `json:"description,omitempty" jsonschema:"Range description (create/update). Per-port descriptions are NOT API-writable, so record uplink/port notes here."`
	StartingPort    int    `json:"starting_port,omitempty" jsonschema:"First physical port number — required for create"`
	EndingPort      int    `json:"ending_port,omitempty" jsonschema:"Last physical port number — required for create"`
	MultipleDevices bool   `json:"multiple_devices,omitempty" jsonschema:"Whether ports may map to multiple devices (usually false; only sent on create)"`
}

func (h *Handler) ManageSwitchPorts(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageSwitchPortsInput) (*sdkmcp.CallToolResult, any, error) {
	if input.DeviceID == "" {
		return toolError("device_id is required"), nil, nil
	}

	switch strings.ToLower(input.Action) {
	case "list", "":
		ranges, err := h.client.ListSwitchPortRanges(ctx, input.DeviceID)
		if err != nil {
			return nil, nil, fmt.Errorf("list switch port ranges: %w", err)
		}
		return marshalResult(dedupeSwitchPortRanges(ranges))
	case "get":
		if input.RangeID == "" {
			return toolError("range_id is required for get"), nil, nil
		}
		ranges, err := h.client.ListSwitchPortRanges(ctx, input.DeviceID)
		if err != nil {
			return nil, nil, fmt.Errorf("get switch port range: %w", err)
		}
		for i := range ranges {
			if strconv.Itoa(ranges[i].ID) == input.RangeID {
				return marshalResult(ranges[i])
			}
		}
		return toolError(fmt.Sprintf("no switch port range %s on device %s", input.RangeID, input.DeviceID)), nil, nil
	case "create":
		if input.Name == "" || input.StartingPort == 0 || input.EndingPort == 0 {
			return toolError("name, starting_port and ending_port are required for create"), nil, nil
		}
		r := &itportal.SwitchPortRange{
			Name:            input.Name,
			Description:     input.Description,
			StartingPort:    input.StartingPort,
			EndingPort:      input.EndingPort,
			MultipleDevices: input.MultipleDevices,
		}
		id, err := h.client.CreateSwitchPortRange(ctx, input.DeviceID, r)
		if err != nil {
			return nil, nil, fmt.Errorf("create switch port range: %w", err)
		}
		if id == 0 {
			return toolText(fmt.Sprintf("Switch port range %q (ports %d-%d) created on device %s.",
				input.Name, input.StartingPort, input.EndingPort, input.DeviceID)), nil, nil
		}
		return toolText(fmt.Sprintf("Switch port range %q (ports %d-%d) created on device %s (range ID: %d).",
			input.Name, input.StartingPort, input.EndingPort, input.DeviceID, id)), nil, nil
	case "update":
		if input.RangeID == "" {
			return toolError("range_id is required for update"), nil, nil
		}
		fields := map[string]interface{}{}
		if input.Name != "" {
			fields["name"] = input.Name
		}
		if input.Description != "" {
			fields["description"] = input.Description
		}
		if input.StartingPort != 0 {
			fields["startingPort"] = input.StartingPort
		}
		if input.EndingPort != 0 {
			fields["endingPort"] = input.EndingPort
		}
		if len(fields) == 0 {
			return toolError("no fields to update (set name, description, starting_port or ending_port)"), nil, nil
		}
		if err := h.client.UpdateSwitchPortRange(ctx, input.DeviceID, input.RangeID, fields); err != nil {
			return nil, nil, fmt.Errorf("update switch port range: %w", err)
		}
		return toolText(fmt.Sprintf("Switch port range %s updated.", input.RangeID)), nil, nil
	case "delete":
		if input.RangeID == "" {
			return toolError("range_id is required for delete"), nil, nil
		}
		if err := h.client.DeleteSwitchPortRange(ctx, input.DeviceID, input.RangeID); err != nil {
			return nil, nil, fmt.Errorf("delete switch port range: %w", err)
		}
		return toolText(fmt.Sprintf("Switch port range %s deleted.", input.RangeID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q (use list, get, create, update, delete)", input.Action)), nil, nil
	}
}

// ---- manage_type ----

var validTypeKinds = map[string]bool{
	"account": true, "agreement": true, "company": true, "contact": true,
	"device": true, "document": true, "facility": true, "configuration": true,
}

type ManageTypeInput struct {
	Action string `json:"action" jsonschema:"One of: list, create, update, delete"`
	Kind   string `json:"kind" jsonschema:"Type category: account, agreement, company, contact, device, document, facility, configuration"`
	TypeID string `json:"type_id,omitempty" jsonschema:"Type ID — required for update/delete"`
	Name   string `json:"name,omitempty" jsonschema:"Type name — required for create, optional for update (rename)"`
}

func (h *Handler) ManageType(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageTypeInput) (*sdkmcp.CallToolResult, any, error) {
	kind := normType(input.Kind)
	if !validTypeKinds[kind] {
		return toolError(fmt.Sprintf("unknown type kind %q. Valid: account, agreement, company, contact, device, document, facility, configuration", input.Kind)), nil, nil
	}
	switch strings.ToLower(input.Action) {
	case "list", "":
		types, err := h.client.ListTypes(ctx, kind)
		if err != nil {
			return nil, nil, fmt.Errorf("list %s types: %w", kind, err)
		}
		return marshalResult(types)
	case "create":
		if input.Name == "" {
			return toolError("name is required for create"), nil, nil
		}
		id, err := h.client.CreateType(ctx, kind, input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("create %s type: %w", kind, err)
		}
		return toolText(fmt.Sprintf("%s type %q created (ID: %d).", kind, input.Name, id)), nil, nil
	case "update":
		if input.TypeID == "" || input.Name == "" {
			return toolError("type_id and name are required for update"), nil, nil
		}
		if err := h.client.UpdateType(ctx, kind, input.TypeID, map[string]interface{}{"name": input.Name}); err != nil {
			return nil, nil, fmt.Errorf("update %s type: %w", kind, err)
		}
		return toolText(fmt.Sprintf("%s type %s renamed to %q.", kind, input.TypeID, input.Name)), nil, nil
	case "delete":
		if input.TypeID == "" {
			return toolError("type_id is required for delete"), nil, nil
		}
		if err := h.client.DeleteType(ctx, kind, input.TypeID); err != nil {
			return nil, nil, fmt.Errorf("delete %s type: %w", kind, err)
		}
		return toolText(fmt.Sprintf("%s type %s deleted.", kind, input.TypeID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q", input.Action)), nil, nil
	}
}

// ---- manage_kb_category ----

type ManageKBCategoryInput struct {
	Action        string `json:"action" jsonschema:"One of: list, create, update, delete, create_subcategory, update_subcategory, delete_subcategory"`
	CategoryID    string `json:"category_id,omitempty" jsonschema:"KB category ID (required except for list/create)"`
	SubCategoryID string `json:"sub_category_id,omitempty" jsonschema:"Subcategory ID (required for update_subcategory/delete_subcategory)"`
	Name          string `json:"name,omitempty" jsonschema:"Category or subcategory name (required for create/update actions)"`
}

func (h *Handler) ManageKBCategory(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageKBCategoryInput) (*sdkmcp.CallToolResult, any, error) {
	switch strings.ToLower(input.Action) {
	case "list", "":
		cats, err := h.client.ListKBCategories(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list KB categories: %w", err)
		}
		return marshalResult(cats)
	case "create":
		if input.Name == "" {
			return toolError("name is required"), nil, nil
		}
		id, err := h.client.CreateKBCategory(ctx, input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("create KB category: %w", err)
		}
		return toolText(fmt.Sprintf("KB category %q created (ID: %d).", input.Name, id)), nil, nil
	case "update":
		if input.CategoryID == "" || input.Name == "" {
			return toolError("category_id and name are required"), nil, nil
		}
		if err := h.client.UpdateKBCategory(ctx, input.CategoryID, map[string]interface{}{"name": input.Name}); err != nil {
			return nil, nil, fmt.Errorf("update KB category: %w", err)
		}
		return toolText(fmt.Sprintf("KB category %s renamed to %q.", input.CategoryID, input.Name)), nil, nil
	case "delete":
		if input.CategoryID == "" {
			return toolError("category_id is required"), nil, nil
		}
		if err := h.client.DeleteKBCategory(ctx, input.CategoryID); err != nil {
			return nil, nil, fmt.Errorf("delete KB category: %w", err)
		}
		return toolText(fmt.Sprintf("KB category %s deleted.", input.CategoryID)), nil, nil
	case "createsubcategory", "create_subcategory":
		if input.CategoryID == "" || input.Name == "" {
			return toolError("category_id and name are required"), nil, nil
		}
		id, err := h.client.CreateKBSubCategory(ctx, input.CategoryID, input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("create KB subcategory: %w", err)
		}
		return toolText(fmt.Sprintf("KB subcategory %q created under category %s (ID: %d).", input.Name, input.CategoryID, id)), nil, nil
	case "updatesubcategory", "update_subcategory":
		if input.CategoryID == "" || input.SubCategoryID == "" || input.Name == "" {
			return toolError("category_id, sub_category_id and name are required"), nil, nil
		}
		if err := h.client.UpdateKBSubCategory(ctx, input.CategoryID, input.SubCategoryID, map[string]interface{}{"name": input.Name}); err != nil {
			return nil, nil, fmt.Errorf("update KB subcategory: %w", err)
		}
		return toolText(fmt.Sprintf("KB subcategory %s renamed to %q.", input.SubCategoryID, input.Name)), nil, nil
	case "deletesubcategory", "delete_subcategory":
		if input.CategoryID == "" || input.SubCategoryID == "" {
			return toolError("category_id and sub_category_id are required"), nil, nil
		}
		if err := h.client.DeleteKBSubCategory(ctx, input.CategoryID, input.SubCategoryID); err != nil {
			return nil, nil, fmt.Errorf("delete KB subcategory: %w", err)
		}
		return toolText(fmt.Sprintf("KB subcategory %s deleted.", input.SubCategoryID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q", input.Action)), nil, nil
	}
}

// ---- add_interaction ----

type AddInteractionInput struct {
	Action     string `json:"action,omitempty" jsonschema:"One of: create (default), list"`
	ObjectType string `json:"object_type" jsonschema:"Object type: account, agreement, cabinet, configuration, contact, device, document, facility, ipnetwork, kb, site (company/client not supported)"`
	ObjectID   string `json:"object_id" jsonschema:"Numeric ID of the object"`
	Note       string `json:"note,omitempty" jsonschema:"Interaction note text — required for create"`
}

func (h *Handler) AddInteraction(ctx context.Context, _ *sdkmcp.CallToolRequest, input AddInteractionInput) (*sdkmcp.CallToolResult, any, error) {
	objType := normType(input.ObjectType)
	if objType == "" {
		return toolError("object_type is required"), nil, nil
	}
	if input.ObjectID == "" {
		return toolError("object_id is required"), nil, nil
	}
	switch strings.ToLower(input.Action) {
	case "list":
		items, _, err := h.client.ListInteractions(ctx, objType, input.ObjectID)
		if err != nil {
			return nil, nil, fmt.Errorf("list interactions: %w", err)
		}
		return marshalResult(items)
	case "create", "":
		if input.Note == "" {
			return toolError("note is required for create"), nil, nil
		}
		created, err := h.client.CreateInteraction(ctx, objType, input.ObjectID, &itportal.Interaction{Note: input.Note})
		if err != nil {
			return nil, nil, fmt.Errorf("create interaction: %w", err)
		}
		return toolText(fmt.Sprintf("Interaction added to %s %s (ID: %d).", objType, input.ObjectID, created.ID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q", input.Action)), nil, nil
	}
}

// ---- manage_credential (additional credentials) ----

type ManageCredentialInput struct {
	Action           string `json:"action" jsonschema:"One of: get, create, update, delete"`
	CredentialID     string `json:"credential_id,omitempty" jsonschema:"Credential ID — required for get/update/delete"`
	Type             string `json:"type,omitempty" jsonschema:"Credential type/label (create)"`
	Username         string `json:"username,omitempty" jsonschema:"Username (create/update)"`
	Password         string `json:"password,omitempty" jsonschema:"Password (create/update)"`
	Description      string `json:"description,omitempty" jsonschema:"Description (create/update)"`
	PortalObjectType string `json:"portal_object_type,omitempty" jsonschema:"Attach to object itemType, e.g. Device, Account, Configuration (create)"`
	PortalObjectID   int    `json:"portal_object_id,omitempty" jsonschema:"Attach to object ID (create)"`
}

func (h *Handler) ManageCredential(ctx context.Context, _ *sdkmcp.CallToolRequest, input ManageCredentialInput) (*sdkmcp.CallToolResult, any, error) {
	switch strings.ToLower(input.Action) {
	case "get":
		if input.CredentialID == "" {
			return toolError("credential_id is required for get"), nil, nil
		}
		cred, err := h.client.GetAdditionalCredential(ctx, input.CredentialID)
		if err != nil {
			return nil, nil, fmt.Errorf("get credential: %w", err)
		}
		return marshalResult(cred)
	case "create":
		cred := &itportal.AdditionalCredential{
			Type:        input.Type,
			Username:    input.Username,
			Password:    input.Password,
			Description: input.Description,
		}
		if input.PortalObjectType != "" && input.PortalObjectID != 0 {
			cred.PortalObject = &itportal.PortalObjectRef{ItemType: input.PortalObjectType, ID: input.PortalObjectID}
		}
		created, err := h.client.CreateAdditionalCredential(ctx, cred)
		if err != nil {
			return nil, nil, fmt.Errorf("create credential: %w", err)
		}
		return toolText(fmt.Sprintf("Credential created (ID: %d).", created.ID)), nil, nil
	case "update":
		if input.CredentialID == "" {
			return toolError("credential_id is required for update"), nil, nil
		}
		fields := map[string]interface{}{}
		if input.Type != "" {
			fields["type"] = input.Type
		}
		if input.Username != "" {
			fields["username"] = input.Username
		}
		if input.Password != "" {
			fields["password"] = input.Password
		}
		if input.Description != "" {
			fields["description"] = input.Description
		}
		if len(fields) == 0 {
			return toolError("no fields to update"), nil, nil
		}
		if err := h.client.UpdateAdditionalCredential(ctx, input.CredentialID, fields); err != nil {
			return nil, nil, fmt.Errorf("update credential: %w", err)
		}
		return toolText(fmt.Sprintf("Credential %s updated.", input.CredentialID)), nil, nil
	case "delete":
		if input.CredentialID == "" {
			return toolError("credential_id is required for delete"), nil, nil
		}
		if err := h.client.DeleteAdditionalCredential(ctx, input.CredentialID); err != nil {
			return nil, nil, fmt.Errorf("delete credential: %w", err)
		}
		return toolText(fmt.Sprintf("Credential %s deleted.", input.CredentialID)), nil, nil
	default:
		return toolError(fmt.Sprintf("unknown action %q (use get, create, update, delete)", input.Action)), nil, nil
	}
}

// ---- get_credentials (read secrets for an object) ----

type GetCredentialsInput struct {
	ObjectType string `json:"object_type" jsonschema:"One of: account, device, configuration"`
	ObjectID   string `json:"object_id" jsonschema:"Numeric ID of the object"`
}

func (h *Handler) GetCredentials(ctx context.Context, _ *sdkmcp.CallToolRequest, input GetCredentialsInput) (*sdkmcp.CallToolResult, any, error) {
	if input.ObjectID == "" {
		return toolError("object_id is required"), nil, nil
	}
	var (
		creds []itportal.Credential
		err   error
	)
	switch normType(input.ObjectType) {
	case "account":
		creds, err = h.client.GetAccountCredentials(ctx, input.ObjectID)
	case "device":
		creds, err = h.client.GetDeviceCredentials(ctx, input.ObjectID)
	case "configuration":
		creds, err = h.client.GetConfigurationCredentials(ctx, input.ObjectID)
	default:
		return toolError(fmt.Sprintf("unknown object_type %q (use account, device, configuration)", input.ObjectType)), nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get %s credentials: %w", input.ObjectType, err)
	}
	return marshalResult(creds)
}

// ---- get_logs ----

type GetLogsInput struct {
	LogType   string `json:"log_type" jsonschema:"One of: userAccess, adminAccess, loginLogout, passwordAccess, passwordChanges"`
	StartDate string `json:"start_date,omitempty" jsonschema:"Start date YYYY-MM-DD (required by most log endpoints)"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"End date YYYY-MM-DD"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max rows to return (default 50)"`
}

func (h *Handler) GetLogs(ctx context.Context, _ *sdkmcp.CallToolRequest, input GetLogsInput) (*sdkmcp.CallToolResult, any, error) {
	if input.LogType == "" {
		return toolError("log_type is required"), nil, nil
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := h.client.GetLogs(ctx, input.LogType, input.StartDate, input.EndDate, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("get logs %s: %w", input.LogType, err)
	}
	return marshalResult(rows)
}

// decodeBase64 decodes standard or URL-safe base64.
func decodeBase64(s string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("base64_data is not valid base64: %w", err)
	}
	return data, nil
}
