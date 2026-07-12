package itportal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

// This file groups the endpoints introduced or reshaped in ITPortal API v2.1:
// relationships, per-object folders/folderFiles, type & KB-category management,
// company-scoped lists, top-level addresses, credentials and audit logs.

// ---- Relationships (invLinks) ----
//
// objectPath is the plural resource segment of the source object, e.g. "devices"
// or "documents". target.itemType is the capitalised singular, e.g. "Device".

func (c *Client) ListRelationships(ctx context.Context, objectPath, objectID string) ([]Relationship, error) {
	return listAll[Relationship](ctx, c, fmt.Sprintf("/api/2.0/%s/%s/relationships/", objectPath, objectID), nil, 500)
}

func (c *Client) GetRelationship(ctx context.Context, objectPath, objectID, linkID string) (*Relationship, error) {
	return getOne[Relationship](ctx, c, fmt.Sprintf("/api/2.0/%s/%s/relationships/%s/", objectPath, objectID, linkID))
}

func (c *Client) CreateRelationship(ctx context.Context, objectPath, objectID string, rel *Relationship) (*Relationship, error) {
	id, err := c.createID(ctx, fmt.Sprintf("/api/2.0/%s/%s/relationships/", objectPath, objectID), rel)
	if err != nil {
		return nil, err
	}
	rel.ID = id
	return rel, nil
}

func (c *Client) UpdateRelationship(ctx context.Context, objectPath, objectID, linkID string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/2.0/%s/%s/relationships/%s/", objectPath, objectID, linkID), fields, nil)
	return err
}

func (c *Client) DeleteRelationship(ctx context.Context, objectPath, objectID, linkID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/2.0/%s/%s/relationships/%s/", objectPath, objectID, linkID), nil, nil)
	return err
}

// ---- Folders (per-object document tree) ----

func (c *Client) ListFolders(ctx context.Context, objectPath, objectID string) ([]Folder, error) {
	return listAll[Folder](ctx, c, fmt.Sprintf("/api/2.0/%s/%s/folders/", objectPath, objectID), nil, 1000)
}

func (c *Client) GetFolder(ctx context.Context, objectPath, objectID, folderID string) (*Folder, error) {
	return getOne[Folder](ctx, c, fmt.Sprintf("/api/2.0/%s/%s/folders/%s/", objectPath, objectID, folderID))
}

func (c *Client) CreateFolder(ctx context.Context, objectPath, objectID string, folder *Folder) (*Folder, error) {
	id, err := c.createID(ctx, fmt.Sprintf("/api/2.0/%s/%s/folders/", objectPath, objectID), folder)
	if err != nil {
		return nil, err
	}
	folder.ID = id
	return folder, nil
}

func (c *Client) UpdateFolder(ctx context.Context, objectPath, objectID, folderID string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/2.0/%s/%s/folders/%s/", objectPath, objectID, folderID), fields, nil)
	return err
}

func (c *Client) DeleteFolder(ctx context.Context, objectPath, objectID, folderID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/2.0/%s/%s/folders/%s/", objectPath, objectID, folderID), nil, nil)
	return err
}

// ---- Folder files (multipart upload within a folder) ----

func (c *Client) ListFolderFiles(ctx context.Context, objectPath, objectID, folderID string) ([]FolderFile, error) {
	return listAll[FolderFile](ctx, c, fmt.Sprintf("/api/2.0/%s/%s/folderFiles/%s/", objectPath, objectID, folderID), nil, 1000)
}

// UploadFolderFile uploads a file into a folder and returns the new file id.
func (c *Client) UploadFolderFile(ctx context.Context, objectPath, objectID, folderID, fileName, contentType string, data []byte, description string) (int, error) {
	path := fmt.Sprintf("/api/2.0/%s/%s/folderFiles/%s/", objectPath, objectID, folderID)
	extra := map[string]string{}
	if description != "" {
		extra["description"] = description
	}
	resp, err := c.uploadMultipart(ctx, path, fileName, contentType, data, extra)
	if err != nil {
		return 0, err
	}
	return parseLocationID(resp.Header.Get("Location")), nil
}

// DownloadFolderFile fetches the raw bytes of a stored file.
func (c *Client) DownloadFolderFile(ctx context.Context, objectPath, objectID, folderID, fileID string) ([]byte, error) {
	path := fmt.Sprintf("/api/2.0/%s/%s/folderFiles/%s/%s/", objectPath, objectID, folderID, fileID)
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) UpdateFolderFile(ctx context.Context, objectPath, objectID, folderID, fileID string, fields map[string]interface{}) error {
	path := fmt.Sprintf("/api/2.0/%s/%s/folderFiles/%s/%s/", objectPath, objectID, folderID, fileID)
	_, err := c.do(ctx, http.MethodPatch, path, fields, nil)
	return err
}

func (c *Client) DeleteFolderFile(ctx context.Context, objectPath, objectID, folderID, fileID string) error {
	path := fmt.Sprintf("/api/2.0/%s/%s/folderFiles/%s/%s/", objectPath, objectID, folderID, fileID)
	_, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// ---- Switch ports (per-device port ranges) ----
//
// Only the range container is writable. Individual per-port descriptions and
// port-to-device assignments are read-only over the REST API (UI-only).

func (c *Client) ListSwitchPortRanges(ctx context.Context, deviceID string) ([]SwitchPortRange, error) {
	return listAll[SwitchPortRange](ctx, c, fmt.Sprintf("/api/2.0/devices/%s/switchPortRanges/", deviceID), nil, 500)
}

// CreateSwitchPortRange creates a range (which auto-provisions its ports) and
// returns the new range id (0 if the API omits a Location header).
func (c *Client) CreateSwitchPortRange(ctx context.Context, deviceID string, r *SwitchPortRange) (int, error) {
	return c.createID(ctx, fmt.Sprintf("/api/2.0/devices/%s/switchPortRanges/", deviceID), r)
}

func (c *Client) UpdateSwitchPortRange(ctx context.Context, deviceID, rangeID string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/2.0/devices/%s/switchPortRanges/%s/", deviceID, rangeID), fields, nil)
	return err
}

func (c *Client) DeleteSwitchPortRange(ctx context.Context, deviceID, rangeID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/2.0/devices/%s/switchPortRanges/%s/", deviceID, rangeID), nil, nil)
	return err
}

// ---- Type management (generic over kind) ----
//
// kind is one of: account, agreement, company, contact, device, document,
// facility, configuration.

func (c *Client) ListTypes(ctx context.Context, kind string) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/"+kind+"/", nil, 1000)
}

func (c *Client) CreateType(ctx context.Context, kind, name string) (int, error) {
	return c.createID(ctx, "/api/2.0/types/"+kind+"/", map[string]string{"name": name})
}

func (c *Client) UpdateType(ctx context.Context, kind, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/types/"+kind+"/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteType(ctx context.Context, kind, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/types/"+kind+"/"+id+"/", nil, nil)
	return err
}

// ---- KB category & subcategory management ----

func (c *Client) CreateKBCategory(ctx context.Context, name string) (int, error) {
	return c.createID(ctx, "/api/2.0/categories/kb/", map[string]string{"name": name})
}

func (c *Client) UpdateKBCategory(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/categories/kb/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteKBCategory(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/categories/kb/"+id+"/", nil, nil)
	return err
}

func (c *Client) CreateKBSubCategory(ctx context.Context, categoryID, name string) (int, error) {
	return c.createID(ctx, "/api/2.0/categories/kb/"+categoryID+"/subcategories/", map[string]string{"name": name})
}

func (c *Client) UpdateKBSubCategory(ctx context.Context, categoryID, subID string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/categories/kb/"+categoryID+"/subcategories/"+subID+"/", fields, nil)
	return err
}

func (c *Client) DeleteKBSubCategory(ctx context.Context, categoryID, subID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/categories/kb/"+categoryID+"/subcategories/"+subID+"/", nil, nil)
	return err
}

// ---- Company-scoped lists ----

func (c *Client) ListCompanyDevices(ctx context.Context, companyID string, opts *ListOptions) ([]Device, int, error) {
	return listOne[Device](ctx, c, "/api/2.0/companies/"+companyID+"/devices/", opts)
}

func (c *Client) ListCompanyDocuments(ctx context.Context, companyID string, opts *ListOptions) ([]Document, int, error) {
	return listOne[Document](ctx, c, "/api/2.0/companies/"+companyID+"/documents/", opts)
}

func (c *Client) ListCompanyAccounts(ctx context.Context, companyID string, opts *ListOptions) ([]Account, int, error) {
	return listOne[Account](ctx, c, "/api/2.0/companies/"+companyID+"/accounts/", opts)
}

func (c *Client) ListCompanyAddresses(ctx context.Context, companyID string, opts *ListOptions) ([]Address, int, error) {
	return listOne[Address](ctx, c, "/api/2.0/companies/"+companyID+"/addresses/", opts)
}

// ---- Credentials ----

func (c *Client) GetAccountCredentials(ctx context.Context, accountID string) ([]Credential, error) {
	return listAll[Credential](ctx, c, "/api/2.0/accounts/"+accountID+"/credentials/", nil, 100)
}

// GetConfigurationCredentials returns credentials for a configuration. Requires the
// encryption key when the org uses custom encryption.
func (c *Client) GetConfigurationCredentials(ctx context.Context, configID string) ([]Credential, error) {
	return listAll[Credential](ctx, c, "/api/2.0/configurations/"+configID+"/credentials/", nil, 100)
}

// ---- Top-level addresses ----

func (c *Client) ListAddresses(ctx context.Context, opts *ListOptions) ([]Address, int, error) {
	return listOne[Address](ctx, c, "/api/2.0/addresses/", opts)
}

func (c *Client) GetAddress(ctx context.Context, id string) (*Address, error) {
	return getOne[Address](ctx, c, "/api/2.0/addresses/"+id+"/")
}

func (c *Client) CreateAddress(ctx context.Context, addr *Address) (*Address, error) {
	return createOne[Address](ctx, c, "/api/2.0/addresses/", addr)
}

func (c *Client) UpdateAddress(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/addresses/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteAddress(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/addresses/"+id+"/", nil, nil)
	return err
}

// ---- Forms ----

func (c *Client) ListForms(ctx context.Context, opts *ListOptions) ([]FormTemplate, int, error) {
	return listOne[FormTemplate](ctx, c, "/api/2.0/forms/", opts)
}

// ---- System ----

func (c *Client) ListMainContacts(ctx context.Context) ([]Contact, error) {
	return listAll[Contact](ctx, c, "/api/2.0/system/companies/mainContacts/", nil, 1000)
}

// ---- Audit logs ----
//
// logType is one of: userAccess, adminAccess, loginLogout, passwordAccess,
// passwordChanges. Most log endpoints require startDate and endDate (YYYY-MM-DD).

func (c *Client) GetLogs(ctx context.Context, logType, startDate, endDate string, limit int) ([]map[string]any, error) {
	q := url.Values{}
	if startDate != "" {
		q.Set("startDate", startDate)
	}
	if endDate != "" {
		q.Set("endDate", endDate)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	data, err := c.do(ctx, http.MethodGet, "/api/2.0/logs/"+logType+"/", nil, q)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Data struct {
			Results []map[string]any `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal logs %s: %w", logType, err)
	}
	return wrapper.Data.Results, nil
}

// ---- Missing entity deletes (parity across all top-level entities) ----

func (c *Client) DeleteContact(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/contacts/"+id+"/", nil, nil)
	return err
}

func (c *Client) DeleteAccount(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/accounts/"+id+"/", nil, nil)
	return err
}

func (c *Client) DeleteAgreement(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/agreements/"+id+"/", nil, nil)
	return err
}

func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/documents/"+id+"/", nil, nil)
	return err
}

func (c *Client) DeleteFacility(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/facilities/"+id+"/", nil, nil)
	return err
}

func (c *Client) DeleteCabinet(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/cabinets/"+id+"/", nil, nil)
	return err
}

func (c *Client) DeleteConfiguration(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/configurations/"+id+"/", nil, nil)
	return err
}

// ---- Multipart helper ----

// uploadMultipart POSTs a file plus optional extra form fields as multipart/form-data.
func (c *Client) uploadMultipart(ctx context.Context, path, fileName, contentType string, data []byte, extra map[string]string) (*apiResponse, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range extra {
		if err := w.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("write form field %s: %w", k, err)
		}
	}
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("create multipart file field: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write file data: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.resolvePath(path), &buf)
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.encryptionKey != "" {
		req.Header.Set("X-Encryption-Key", c.encryptionKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute upload to %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload to %s failed with status %d: %s", path, resp.StatusCode, string(body))
	}
	return &apiResponse{Status: resp.StatusCode, Header: resp.Header, Body: body}, nil
}
