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
	"strings"
	"time"
)

// Client is an authenticated HTTP client for the ITPortal REST API v2.0.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new ITPortal API client.
// baseURL is the root of the ITPortal instance (no trailing slash).
// apiKey is the Authorization token (found in ITPortal Settings → API).
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListOptions holds common query parameters supported by list endpoints.
type ListOptions struct {
	Name           string
	NameStartsWith string
	CompanyID      string
	SiteID         string
	FacilityID     string
	CabinetID      string
	TypeName       string
	IPAddress      string
	MacAddress     string
	SerialNumber   string
	Tag            string
	Manufacturer   string
	ModifiedSince  string
	InOut          *bool // nil = all, true = active, false = inactive
	Deleted        *bool
	ForeignID      string
	Limit          int
	Offset         int
	OrderBy        string
	Extra          map[string]string
}

func (o *ListOptions) toQuery() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.Name != "" {
		q.Set("name", o.Name)
	}
	if o.NameStartsWith != "" {
		q.Set("nameStartsWith", o.NameStartsWith)
	}
	if o.CompanyID != "" {
		q.Set("companyId", o.CompanyID)
	}
	if o.SiteID != "" {
		q.Set("siteId", o.SiteID)
	}
	if o.FacilityID != "" {
		q.Set("facilityId", o.FacilityID)
	}
	if o.CabinetID != "" {
		q.Set("cabinetId", o.CabinetID)
	}
	if o.TypeName != "" {
		q.Set("typeName", o.TypeName)
	}
	if o.IPAddress != "" {
		q.Set("ipAddress", o.IPAddress)
	}
	if o.MacAddress != "" {
		q.Set("macAddress", o.MacAddress)
	}
	if o.SerialNumber != "" {
		q.Set("serialNumber", o.SerialNumber)
	}
	if o.Tag != "" {
		q.Set("tag", o.Tag)
	}
	if o.Manufacturer != "" {
		q.Set("manufacturer", o.Manufacturer)
	}
	if o.ModifiedSince != "" {
		q.Set("modifiedSince", o.ModifiedSince)
	}
	if o.InOut != nil {
		q.Set("inOut", strconv.FormatBool(*o.InOut))
	}
	if o.Deleted != nil {
		q.Set("deleted", strconv.FormatBool(*o.Deleted))
	}
	if o.ForeignID != "" {
		q.Set("foreignId", o.ForeignID)
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Offset > 0 {
		q.Set("offset", strconv.Itoa(o.Offset))
	}
	if o.OrderBy != "" {
		q.Set("orderBy", o.OrderBy)
	}
	for k, v := range o.Extra {
		q.Set(k, v)
	}
	return q
}

// do executes an authenticated HTTP request against the ITPortal API.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, query url.Values) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request %s %s: %w", method, path, err)
	}

	req.Header.Set("Authorization", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response from %s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ITPortal API %s %s → %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// listPage fetches a single page of entities.
func listPage[T any](ctx context.Context, c *Client, path string, opts *ListOptions) ([]T, int, error) {
	data, err := c.do(ctx, http.MethodGet, path, nil, opts.toQuery())
	if err != nil {
		return nil, 0, err
	}
	var wrapper struct {
		Code int `json:"code"`
		Data struct {
			Results []T `json:"results"`
			Total   int `json:"total"`
			Count   int `json:"count"`
			Offset  int `json:"offset"`
			Limit   int `json:"limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, 0, fmt.Errorf("unmarshal list response from %s: %w", path, err)
	}
	return wrapper.Data.Results, wrapper.Data.Total, nil
}

// listAll fetches all pages up to maxItems using the configured page size.
func listAll[T any](ctx context.Context, c *Client, path string, opts *ListOptions, maxItems int) ([]T, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	const pageSize = 100
	var all []T
	offset := 0
	for {
		pagOpts := *opts
		pagOpts.Limit = pageSize
		pagOpts.Offset = offset

		items, total, err := listPage[T](ctx, c, path, &pagOpts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		offset += len(items)
		if offset >= total || offset >= maxItems || len(items) == 0 {
			break
		}
	}
	return all, nil
}

// getOne fetches a single entity by fully-qualified path (including ID).
func getOne[T any](ctx context.Context, c *Client, path string) (*T, error) {
	items, _, err := listPage[T](ctx, c, path, nil)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no entity found at %s", path)
	}
	return &items[0], nil
}

// createOne POSTs a new entity and returns the created record.
func createOne[T any](ctx context.Context, c *Client, path string, body interface{}) (*T, error) {
	data, err := c.do(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Code int `json:"code"`
		Data T   `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal create response from %s: %w", path, err)
	}
	return &wrapper.Data, nil
}

// ---- Companies ----

func (c *Client) ListCompanies(ctx context.Context, opts *ListOptions) ([]Company, int, error) {
	return listPage[Company](ctx, c, "/api/2.0/companies/", opts)
}

func (c *Client) ListAllCompanies(ctx context.Context, opts *ListOptions, max int) ([]Company, error) {
	return listAll[Company](ctx, c, "/api/2.0/companies/", opts, max)
}

func (c *Client) GetCompany(ctx context.Context, id string) (*Company, error) {
	return getOne[Company](ctx, c, "/api/2.0/companies/"+id+"/")
}

func (c *Client) CreateCompany(ctx context.Context, company *Company) (*Company, error) {
	return createOne[Company](ctx, c, "/api/2.0/companies/", company)
}

func (c *Client) UpdateCompany(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/companies/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteCompany(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/companies/"+id+"/", nil, nil)
	return err
}

// ---- Sites ----

func (c *Client) ListSites(ctx context.Context, opts *ListOptions) ([]Site, int, error) {
	return listPage[Site](ctx, c, "/api/2.0/sites/", opts)
}

func (c *Client) ListAllSites(ctx context.Context, opts *ListOptions, max int) ([]Site, error) {
	return listAll[Site](ctx, c, "/api/2.0/sites/", opts, max)
}

func (c *Client) GetSite(ctx context.Context, id string) (*Site, error) {
	return getOne[Site](ctx, c, "/api/2.0/sites/"+id+"/")
}

func (c *Client) CreateSite(ctx context.Context, site *Site) (*Site, error) {
	return createOne[Site](ctx, c, "/api/2.0/sites/", site)
}

func (c *Client) UpdateSite(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/sites/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteSite(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/sites/"+id+"/", nil, nil)
	return err
}

// ---- Devices ----

func (c *Client) ListDevices(ctx context.Context, opts *ListOptions) ([]Device, int, error) {
	return listPage[Device](ctx, c, "/api/2.0/devices/", opts)
}

func (c *Client) ListAllDevices(ctx context.Context, opts *ListOptions, max int) ([]Device, error) {
	return listAll[Device](ctx, c, "/api/2.0/devices/", opts, max)
}

func (c *Client) GetDevice(ctx context.Context, id string) (*Device, error) {
	return getOne[Device](ctx, c, "/api/2.0/devices/"+id+"/")
}

func (c *Client) CreateDevice(ctx context.Context, device *Device) (*Device, error) {
	return createOne[Device](ctx, c, "/api/2.0/devices/", device)
}

func (c *Client) UpdateDevice(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/devices/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteDevice(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/devices/"+id+"/", nil, nil)
	return err
}

func (c *Client) GetDeviceIPs(ctx context.Context, deviceID string) ([]DeviceIP, error) {
	return listAll[DeviceIP](ctx, c, "/api/2.0/devices/"+deviceID+"/ips/", nil, 500)
}

func (c *Client) AddDeviceIP(ctx context.Context, deviceID string, ip *DeviceIP) (*DeviceIP, error) {
	return createOne[DeviceIP](ctx, c, "/api/2.0/devices/"+deviceID+"/ips/", ip)
}

func (c *Client) GetDeviceNotes(ctx context.Context, deviceID string) ([]DeviceNote, error) {
	return listAll[DeviceNote](ctx, c, "/api/2.0/devices/"+deviceID+"/notes/", nil, 500)
}

func (c *Client) AddDeviceNote(ctx context.Context, deviceID string, note *DeviceNote) (*DeviceNote, error) {
	return createOne[DeviceNote](ctx, c, "/api/2.0/devices/"+deviceID+"/notes/", note)
}

func (c *Client) GetDeviceManagementURLs(ctx context.Context, deviceID string) ([]DeviceMUrl, error) {
	return listAll[DeviceMUrl](ctx, c, "/api/2.0/devices/"+deviceID+"/managementUrls/", nil, 100)
}

func (c *Client) AddDeviceManagementURL(ctx context.Context, deviceID string, murl *DeviceMUrl) (*DeviceMUrl, error) {
	return createOne[DeviceMUrl](ctx, c, "/api/2.0/devices/"+deviceID+"/managementUrls/", murl)
}

func (c *Client) GetDeviceCredentials(ctx context.Context, deviceID string) ([]Credential, error) {
	return listAll[Credential](ctx, c, "/api/2.0/devices/"+deviceID+"/credentials/", nil, 100)
}

// ---- Knowledge Base ----

func (c *Client) ListKBs(ctx context.Context, opts *ListOptions) ([]KB, int, error) {
	return listPage[KB](ctx, c, "/api/2.0/kbs/", opts)
}

func (c *Client) ListAllKBs(ctx context.Context, opts *ListOptions, max int) ([]KB, error) {
	return listAll[KB](ctx, c, "/api/2.0/kbs/", opts, max)
}

func (c *Client) GetKB(ctx context.Context, id string) (*KB, error) {
	return getOne[KB](ctx, c, "/api/2.0/kbs/"+id+"/")
}

func (c *Client) CreateKB(ctx context.Context, kb *KB) (*KB, error) {
	return createOne[KB](ctx, c, "/api/2.0/kbs/", kb)
}

func (c *Client) UpdateKB(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/kbs/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteKB(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/kbs/"+id+"/", nil, nil)
	return err
}

func (c *Client) ListKBCategories(ctx context.Context) ([]KBCategory, error) {
	return listAll[KBCategory](ctx, c, "/api/2.0/categories/kb/", nil, 500)
}

// ---- Contacts ----

func (c *Client) ListContacts(ctx context.Context, opts *ListOptions) ([]Contact, int, error) {
	return listPage[Contact](ctx, c, "/api/2.0/contacts/", opts)
}

func (c *Client) ListAllContacts(ctx context.Context, opts *ListOptions, max int) ([]Contact, error) {
	return listAll[Contact](ctx, c, "/api/2.0/contacts/", opts, max)
}

func (c *Client) GetContact(ctx context.Context, id string) (*Contact, error) {
	return getOne[Contact](ctx, c, "/api/2.0/contacts/"+id+"/")
}

func (c *Client) CreateContact(ctx context.Context, contact *Contact) (*Contact, error) {
	return createOne[Contact](ctx, c, "/api/2.0/contacts/", contact)
}

func (c *Client) UpdateContact(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/contacts/"+id+"/", fields, nil)
	return err
}

// ---- Accounts ----

func (c *Client) ListAccounts(ctx context.Context, opts *ListOptions) ([]Account, int, error) {
	return listPage[Account](ctx, c, "/api/2.0/accounts/", opts)
}

func (c *Client) GetAccount(ctx context.Context, id string) (*Account, error) {
	return getOne[Account](ctx, c, "/api/2.0/accounts/"+id+"/")
}

func (c *Client) CreateAccount(ctx context.Context, account *Account) (*Account, error) {
	return createOne[Account](ctx, c, "/api/2.0/accounts/", account)
}

func (c *Client) UpdateAccount(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/accounts/"+id+"/", fields, nil)
	return err
}

// ---- Agreements ----

func (c *Client) ListAgreements(ctx context.Context, opts *ListOptions) ([]Agreement, int, error) {
	return listPage[Agreement](ctx, c, "/api/2.0/agreements/", opts)
}

func (c *Client) ListAllAgreements(ctx context.Context, opts *ListOptions, max int) ([]Agreement, error) {
	return listAll[Agreement](ctx, c, "/api/2.0/agreements/", opts, max)
}

func (c *Client) GetAgreement(ctx context.Context, id string) (*Agreement, error) {
	return getOne[Agreement](ctx, c, "/api/2.0/agreements/"+id+"/")
}

func (c *Client) CreateAgreement(ctx context.Context, agreement *Agreement) (*Agreement, error) {
	return createOne[Agreement](ctx, c, "/api/2.0/agreements/", agreement)
}

func (c *Client) UpdateAgreement(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/agreements/"+id+"/", fields, nil)
	return err
}

// ---- Documents ----

func (c *Client) ListDocuments(ctx context.Context, opts *ListOptions) ([]Document, int, error) {
	return listPage[Document](ctx, c, "/api/2.0/documents/", opts)
}

func (c *Client) GetDocument(ctx context.Context, id string) (*Document, error) {
	return getOne[Document](ctx, c, "/api/2.0/documents/"+id+"/")
}

func (c *Client) CreateDocument(ctx context.Context, doc *Document) (*Document, error) {
	return createOne[Document](ctx, c, "/api/2.0/documents/", doc)
}

func (c *Client) UpdateDocument(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/documents/"+id+"/", fields, nil)
	return err
}

// ---- IP Networks ----

func (c *Client) ListIPNetworks(ctx context.Context, opts *ListOptions) ([]IPNetwork, int, error) {
	return listPage[IPNetwork](ctx, c, "/api/2.0/ipnetworks/", opts)
}

func (c *Client) ListAllIPNetworks(ctx context.Context, opts *ListOptions, max int) ([]IPNetwork, error) {
	return listAll[IPNetwork](ctx, c, "/api/2.0/ipnetworks/", opts, max)
}

func (c *Client) GetIPNetwork(ctx context.Context, id string) (*IPNetwork, error) {
	return getOne[IPNetwork](ctx, c, "/api/2.0/ipnetworks/"+id+"/")
}

func (c *Client) CreateIPNetwork(ctx context.Context, network *IPNetwork) (*IPNetwork, error) {
	return createOne[IPNetwork](ctx, c, "/api/2.0/ipnetworks/", network)
}

func (c *Client) UpdateIPNetwork(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/ipnetworks/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteIPNetwork(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/ipnetworks/"+id+"/", nil, nil)
	return err
}

// ---- Facilities ----

func (c *Client) ListFacilities(ctx context.Context, opts *ListOptions) ([]Facility, int, error) {
	return listPage[Facility](ctx, c, "/api/2.0/facilities/", opts)
}

func (c *Client) GetFacility(ctx context.Context, id string) (*Facility, error) {
	return getOne[Facility](ctx, c, "/api/2.0/facilities/"+id+"/")
}

func (c *Client) CreateFacility(ctx context.Context, facility *Facility) (*Facility, error) {
	return createOne[Facility](ctx, c, "/api/2.0/facilities/", facility)
}

func (c *Client) UpdateFacility(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/facilities/"+id+"/", fields, nil)
	return err
}

// ---- Cabinets ----

func (c *Client) ListCabinets(ctx context.Context, opts *ListOptions) ([]Cabinet, int, error) {
	return listPage[Cabinet](ctx, c, "/api/2.0/cabinets/", opts)
}

func (c *Client) GetCabinet(ctx context.Context, id string) (*Cabinet, error) {
	return getOne[Cabinet](ctx, c, "/api/2.0/cabinets/"+id+"/")
}

func (c *Client) CreateCabinet(ctx context.Context, cabinet *Cabinet) (*Cabinet, error) {
	return createOne[Cabinet](ctx, c, "/api/2.0/cabinets/", cabinet)
}

func (c *Client) UpdateCabinet(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/cabinets/"+id+"/", fields, nil)
	return err
}

// ---- Configurations ----

func (c *Client) ListConfigurations(ctx context.Context, opts *ListOptions) ([]Configuration, int, error) {
	return listPage[Configuration](ctx, c, "/api/2.0/configurations/", opts)
}

func (c *Client) GetConfiguration(ctx context.Context, id string) (*Configuration, error) {
	return getOne[Configuration](ctx, c, "/api/2.0/configurations/"+id+"/")
}

func (c *Client) CreateConfiguration(ctx context.Context, config *Configuration) (*Configuration, error) {
	return createOne[Configuration](ctx, c, "/api/2.0/configurations/", config)
}

func (c *Client) UpdateConfiguration(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/configurations/"+id+"/", fields, nil)
	return err
}

// ---- Form Instances ----

func (c *Client) ListFormInstances(ctx context.Context, opts *ListOptions) ([]FormInstance, int, error) {
	return listPage[FormInstance](ctx, c, "/api/2.0/forminstances/", opts)
}

func (c *Client) GetFormInstance(ctx context.Context, id string) (*FormInstance, error) {
	return getOne[FormInstance](ctx, c, "/api/2.0/forminstances/"+id+"/")
}

// ---- Templates ----

func (c *Client) ListTemplates(ctx context.Context, opts *ListOptions) ([]Template, int, error) {
	return listPage[Template](ctx, c, "/api/2.0/templates/", opts)
}

func (c *Client) GetObjectTemplates(ctx context.Context, objectType, objectID string) ([]Template, int, error) {
	path := fmt.Sprintf("/api/2.0/templates/%s/%s/", objectType, objectID)
	return listPage[Template](ctx, c, path, nil)
}

func (c *Client) UpdateTemplateField(ctx context.Context, objectType, objectID, templateID, fieldID string, value interface{}) error {
	path := fmt.Sprintf("/api/2.0/templates/%s/%s/%s/fields/%s/", objectType, objectID, templateID, fieldID)
	_, err := c.do(ctx, http.MethodPatch, path, map[string]interface{}{"value": value}, nil)
	return err
}

// ---- Additional Credentials ----

func (c *Client) ListAdditionalCredentials(ctx context.Context, opts *ListOptions) ([]AdditionalCredential, int, error) {
	return listPage[AdditionalCredential](ctx, c, "/api/2.0/additionalCredentials/", opts)
}

func (c *Client) CreateAdditionalCredential(ctx context.Context, cred *AdditionalCredential) (*AdditionalCredential, error) {
	return createOne[AdditionalCredential](ctx, c, "/api/2.0/additionalCredentials/", cred)
}

func (c *Client) UpdateAdditionalCredential(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/additionalCredentials/"+id+"/", fields, nil)
	return err
}

// ---- Interactions ----

func (c *Client) ListInteractions(ctx context.Context, objectType, objectID string) ([]Interaction, int, error) {
	path := fmt.Sprintf("/api/2.0/interactions/%s/%s/", objectType, objectID)
	return listPage[Interaction](ctx, c, path, nil)
}

func (c *Client) CreateInteraction(ctx context.Context, objectType, objectID string, interaction *Interaction) (*Interaction, error) {
	path := fmt.Sprintf("/api/2.0/interactions/%s/%s/", objectType, objectID)
	return createOne[Interaction](ctx, c, path, interaction)
}

// ---- System ----

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	return listAll[User](ctx, c, "/api/2.0/system/users/", nil, 1000)
}

func (c *Client) ListSecurityGroups(ctx context.Context) ([]SecurityGroup, error) {
	return listAll[SecurityGroup](ctx, c, "/api/2.0/system/groups/securityGroups/", nil, 500)
}

func (c *Client) ListCountries(ctx context.Context) ([]Country, error) {
	return listAll[Country](ctx, c, "/api/2.0/system/countries/", nil, 300)
}

// ---- Reference types ----

func (c *Client) ListDeviceTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/device/", nil, 500)
}

func (c *Client) ListCompanyTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/company/", nil, 500)
}

func (c *Client) ListAccountTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/account/", nil, 500)
}

func (c *Client) ListContactTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/contact/", nil, 500)
}

func (c *Client) ListDocumentTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/document/", nil, 500)
}

func (c *Client) ListAgreementTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/agreement/", nil, 500)
}

func (c *Client) ListFacilityTypes(ctx context.Context) ([]TypeItem, error) {
	return listAll[TypeItem](ctx, c, "/api/2.0/types/facility/", nil, 500)
}

// ---- File Upload ----

// UploadFile uploads raw file bytes to the given ITPortal endpoint via multipart/form-data.
// uploadPath must be a path like /api/2.0/devices/{id}/configurationFiles/
func (c *Client) UploadFile(ctx context.Context, uploadPath, fileName, contentType string, fileData []byte) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("create multipart file field: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return fmt.Errorf("write file data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+uploadPath, &buf)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute upload to %s: %w", uploadPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload to %s failed with status %d: %s", uploadPath, resp.StatusCode, string(body))
	}
	return nil
}
