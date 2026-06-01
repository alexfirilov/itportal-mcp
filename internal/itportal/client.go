package itportal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultAPIVersion is the ITPortal REST API version targeted when none is configured.
const DefaultAPIVersion = "2.1"

// internalVersionPrefix is the version embedded in path literals throughout this file.
// do() rewrites it to the configured apiVersion at request time, so call-sites can keep
// using stable, readable path strings.
const internalVersionPrefix = "/api/2.0/"

// locationIDPattern extracts the trailing numeric id from a Location header.
var locationIDPattern = regexp.MustCompile(`(\d+)/?$`)

// Client is an authenticated HTTP client for the ITPortal REST API (v2.x).
type Client struct {
	baseURL       string
	apiVersion    string
	authHeader    string
	encryptionKey string
	httpClient    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithAPIVersion overrides the API version (default DefaultAPIVersion).
func WithAPIVersion(v string) Option {
	return func(c *Client) {
		if v != "" {
			c.apiVersion = v
		}
	}
}

// WithEncryptionKey sets the custom encryption key sent as X-Encryption-Key on
// credential endpoints (only needed when the org uses custom encryption).
func WithEncryptionKey(k string) Option {
	return func(c *Client) { c.encryptionKey = k }
}

// NewClient creates a new ITPortal API client.
// baseURL is the root of the ITPortal instance (no trailing slash).
// apiKey is the ITPortal API token; it is sent as HTTP Basic auth (key as password)
// unless it already carries an explicit scheme ("Basic "/"Bearer ").
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiVersion: DefaultAPIVersion,
		authHeader: buildAuthHeader(apiKey),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// buildAuthHeader returns the Authorization header value for the given key.
func buildAuthHeader(apiKey string) string {
	k := strings.TrimSpace(apiKey)
	low := strings.ToLower(k)
	if strings.HasPrefix(low, "basic ") || strings.HasPrefix(low, "bearer ") {
		return k
	}
	// ITPortal expects the API key as the password in HTTP Basic auth.
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+k))
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
	Offset         int    // deprecated in v2.1; prefer Cursor
	Cursor         string // v2.1 cursor pagination token
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
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
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

// resolvePath rewrites the internal version prefix to the configured API version.
func (c *Client) resolvePath(path string) string {
	if strings.HasPrefix(path, internalVersionPrefix) {
		return "/api/" + c.apiVersion + "/" + strings.TrimPrefix(path, internalVersionPrefix)
	}
	return path
}

// apiResponse is the low-level result of an HTTP call.
type apiResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

// doMeta executes an authenticated request and returns the full response. It does
// not enforce a 2xx status; callers decide how to interpret the result.
func (c *Client) doMeta(ctx context.Context, method, path string, body interface{}, query url.Values) (*apiResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.baseURL + c.resolvePath(path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request %s %s: %w", method, path, err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		// RFC 7396 merge-patch content type is required for PATCH in v2.1.
		if method == http.MethodPatch {
			req.Header.Set("Content-Type", "application/merge-patch+json")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if c.encryptionKey != "" {
		req.Header.Set("X-Encryption-Key", c.encryptionKey)
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
	return &apiResponse{Status: resp.StatusCode, Header: resp.Header, Body: respBody}, nil
}

// do executes a request and returns the body, enforcing a 2xx status code.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, query url.Values) ([]byte, error) {
	resp, err := c.doMeta(ctx, method, path, body, query)
	if err != nil {
		return nil, err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return nil, fmt.Errorf("ITPortal API %s %s → %d: %s", method, path, resp.Status, string(resp.Body))
	}
	return resp.Body, nil
}

// createID POSTs a new entity and returns the id parsed from the Location header.
// v2.1 responds 201 with a Location header and no body.
func (c *Client) createID(ctx context.Context, path string, body interface{}) (int, error) {
	resp, err := c.doMeta(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return 0, err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return 0, fmt.Errorf("ITPortal API POST %s → %d: %s", path, resp.Status, string(resp.Body))
	}
	if id := parseLocationID(resp.Header.Get("Location")); id != 0 {
		return id, nil
	}
	// Fallback: some deployments return the entity in the body.
	var wrapper struct {
		Data struct {
			ID      int `json:"id"`
			Results []struct {
				ID int `json:"id"`
			} `json:"results"`
		} `json:"data"`
	}
	if json.Unmarshal(resp.Body, &wrapper) == nil {
		if wrapper.Data.ID != 0 {
			return wrapper.Data.ID, nil
		}
		if len(wrapper.Data.Results) > 0 {
			return wrapper.Data.Results[0].ID, nil
		}
	}
	return 0, nil
}

// parseLocationID extracts the trailing numeric id from a Location header value.
func parseLocationID(location string) int {
	m := locationIDPattern.FindStringSubmatch(location)
	if len(m) < 2 {
		return 0
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

// pageMeta carries pagination metadata returned alongside a list page.
type pageMeta struct {
	Total      int
	Count      int
	NextCursor string
}

// listPage fetches a single page of entities and its pagination metadata.
func listPage[T any](ctx context.Context, c *Client, path string, opts *ListOptions) ([]T, pageMeta, error) {
	data, err := c.do(ctx, http.MethodGet, path, nil, opts.toQuery())
	if err != nil {
		return nil, pageMeta{}, err
	}
	var wrapper struct {
		Code int `json:"code"`
		Data struct {
			Results    []T    `json:"results"`
			Total      int    `json:"total"`
			Count      int    `json:"count"`
			NextCursor string `json:"nextCursor"`
			Offset     int    `json:"offset"`
			Limit      int    `json:"limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, pageMeta{}, fmt.Errorf("unmarshal list response from %s: %w", path, err)
	}
	meta := pageMeta{Total: wrapper.Data.Total, Count: wrapper.Data.Count, NextCursor: wrapper.Data.NextCursor}
	return wrapper.Data.Results, meta, nil
}

// listOne is the (results, total) form used by exported List* methods. With cursor
// pagination Total may be unreported; callers should treat 0 as "unknown".
func listOne[T any](ctx context.Context, c *Client, path string, opts *ListOptions) ([]T, int, error) {
	items, meta, err := listPage[T](ctx, c, path, opts)
	if err != nil {
		return nil, 0, err
	}
	total := meta.Total
	if total == 0 {
		total = len(items)
	}
	return items, total, nil
}

// listAll fetches all pages up to maxItems, following the v2.1 nextCursor token.
func listAll[T any](ctx context.Context, c *Client, path string, opts *ListOptions, maxItems int) ([]T, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	const pageSize = 100
	var all []T
	cursor := ""
	for {
		pagOpts := *opts
		pagOpts.Limit = pageSize
		pagOpts.Cursor = cursor

		items, meta, err := listPage[T](ctx, c, path, &pagOpts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(all) >= maxItems {
			all = all[:maxItems]
			break
		}
		if meta.NextCursor == "" || len(items) == 0 {
			break
		}
		cursor = meta.NextCursor
	}
	return all, nil
}

// getOne fetches a single entity. v2.1 returns the record inside data.results[0].
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

// createOne POSTs a new entity, then GETs it back so the full record (including the
// read-only portal url) is returned. collectionPath is the list endpoint, e.g.
// "/api/2.0/companies/"; the single-resource GET is collectionPath + id + "/".
func createOne[T any](ctx context.Context, c *Client, collectionPath string, body interface{}) (*T, error) {
	id, err := c.createID(ctx, collectionPath, body)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, fmt.Errorf("create at %s succeeded but no id was returned", collectionPath)
	}
	return getOne[T](ctx, c, collectionPath+strconv.Itoa(id)+"/")
}

// ---- Companies ----

func (c *Client) ListCompanies(ctx context.Context, opts *ListOptions) ([]Company, int, error) {
	return listOne[Company](ctx, c, "/api/2.0/companies/", opts)
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
	return listOne[Site](ctx, c, "/api/2.0/sites/", opts)
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
	return listOne[Device](ctx, c, "/api/2.0/devices/", opts)
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
	id, err := c.createID(ctx, "/api/2.0/devices/"+deviceID+"/ips/", ip)
	if err != nil {
		return nil, err
	}
	ip.ID = id
	return ip, nil
}

func (c *Client) GetDeviceNotes(ctx context.Context, deviceID string) ([]DeviceNote, error) {
	return listAll[DeviceNote](ctx, c, "/api/2.0/devices/"+deviceID+"/notes/", nil, 500)
}

func (c *Client) AddDeviceNote(ctx context.Context, deviceID string, note *DeviceNote) (*DeviceNote, error) {
	id, err := c.createID(ctx, "/api/2.0/devices/"+deviceID+"/notes/", note)
	if err != nil {
		return nil, err
	}
	note.ID = id
	return note, nil
}

func (c *Client) GetDeviceManagementURLs(ctx context.Context, deviceID string) ([]DeviceMUrl, error) {
	return listAll[DeviceMUrl](ctx, c, "/api/2.0/devices/"+deviceID+"/managementUrls/", nil, 100)
}

func (c *Client) AddDeviceManagementURL(ctx context.Context, deviceID string, murl *DeviceMUrl) (*DeviceMUrl, error) {
	id, err := c.createID(ctx, "/api/2.0/devices/"+deviceID+"/managementUrls/", murl)
	if err != nil {
		return nil, err
	}
	murl.ID = id
	return murl, nil
}

func (c *Client) DeleteDeviceManagementURL(ctx context.Context, deviceID, murlID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/devices/"+deviceID+"/managementUrls/"+murlID+"/", nil, nil)
	return err
}

func (c *Client) GetDeviceCredentials(ctx context.Context, deviceID string) ([]Credential, error) {
	return listAll[Credential](ctx, c, "/api/2.0/devices/"+deviceID+"/credentials/", nil, 100)
}

// ---- Knowledge Base ----

func (c *Client) ListKBs(ctx context.Context, opts *ListOptions) ([]KB, int, error) {
	return listOne[KB](ctx, c, "/api/2.0/kbs/", opts)
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
	return listOne[Contact](ctx, c, "/api/2.0/contacts/", opts)
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
	return listOne[Account](ctx, c, "/api/2.0/accounts/", opts)
}

func (c *Client) ListAllAccounts(ctx context.Context, opts *ListOptions, max int) ([]Account, error) {
	return listAll[Account](ctx, c, "/api/2.0/accounts/", opts, max)
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
	return listOne[Agreement](ctx, c, "/api/2.0/agreements/", opts)
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
	return listOne[Document](ctx, c, "/api/2.0/documents/", opts)
}

func (c *Client) ListAllDocuments(ctx context.Context, opts *ListOptions, max int) ([]Document, error) {
	return listAll[Document](ctx, c, "/api/2.0/documents/", opts, max)
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
	return listOne[IPNetwork](ctx, c, "/api/2.0/ipnetworks/", opts)
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
	return listOne[Facility](ctx, c, "/api/2.0/facilities/", opts)
}

func (c *Client) ListAllFacilities(ctx context.Context, opts *ListOptions, max int) ([]Facility, error) {
	return listAll[Facility](ctx, c, "/api/2.0/facilities/", opts, max)
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
	return listOne[Cabinet](ctx, c, "/api/2.0/cabinets/", opts)
}

func (c *Client) ListAllCabinets(ctx context.Context, opts *ListOptions, max int) ([]Cabinet, error) {
	return listAll[Cabinet](ctx, c, "/api/2.0/cabinets/", opts, max)
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
	return listOne[Configuration](ctx, c, "/api/2.0/configurations/", opts)
}

func (c *Client) ListAllConfigurations(ctx context.Context, opts *ListOptions, max int) ([]Configuration, error) {
	return listAll[Configuration](ctx, c, "/api/2.0/configurations/", opts, max)
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
	return listOne[FormInstance](ctx, c, "/api/2.0/forminstances/", opts)
}

func (c *Client) GetFormInstance(ctx context.Context, id string) (*FormInstance, error) {
	return getOne[FormInstance](ctx, c, "/api/2.0/forminstances/"+id+"/")
}

// ---- Templates ----

func (c *Client) ListTemplates(ctx context.Context, opts *ListOptions) ([]Template, int, error) {
	return listOne[Template](ctx, c, "/api/2.0/templates/", opts)
}

func (c *Client) GetObjectTemplates(ctx context.Context, objectType, objectID string) ([]Template, int, error) {
	path := fmt.Sprintf("/api/2.0/templates/%s/%s/", objectType, objectID)
	return listOne[Template](ctx, c, path, nil)
}

func (c *Client) UpdateTemplateField(ctx context.Context, objectType, objectID, templateID, fieldID string, value interface{}) error {
	path := fmt.Sprintf("/api/2.0/templates/%s/%s/%s/fields/%s/", objectType, objectID, templateID, fieldID)
	_, err := c.do(ctx, http.MethodPatch, path, map[string]interface{}{"value": value}, nil)
	return err
}

// ---- Additional Credentials ----

func (c *Client) ListAdditionalCredentials(ctx context.Context, opts *ListOptions) ([]AdditionalCredential, int, error) {
	return listOne[AdditionalCredential](ctx, c, "/api/2.0/additionalCredentials/", opts)
}

func (c *Client) GetAdditionalCredential(ctx context.Context, id string) (*AdditionalCredential, error) {
	return getOne[AdditionalCredential](ctx, c, "/api/2.0/additionalCredentials/"+id+"/")
}

func (c *Client) CreateAdditionalCredential(ctx context.Context, cred *AdditionalCredential) (*AdditionalCredential, error) {
	// Don't GET back: the single-resource read requires the encryption key and would
	// return the secret. Return the input echoed with its new id instead.
	id, err := c.createID(ctx, "/api/2.0/additionalCredentials/", cred)
	if err != nil {
		return nil, err
	}
	cred.ID = id
	return cred, nil
}

func (c *Client) UpdateAdditionalCredential(ctx context.Context, id string, fields map[string]interface{}) error {
	_, err := c.do(ctx, http.MethodPatch, "/api/2.0/additionalCredentials/"+id+"/", fields, nil)
	return err
}

func (c *Client) DeleteAdditionalCredential(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/additionalCredentials/"+id+"/", nil, nil)
	return err
}

// ---- Interactions ----

func (c *Client) ListInteractions(ctx context.Context, objectType, objectID string) ([]Interaction, int, error) {
	path := fmt.Sprintf("/api/2.0/interactions/%s/%s/", objectType, objectID)
	return listOne[Interaction](ctx, c, path, nil)
}

func (c *Client) CreateInteraction(ctx context.Context, objectType, objectID string, interaction *Interaction) (*Interaction, error) {
	path := fmt.Sprintf("/api/2.0/interactions/%s/%s/", objectType, objectID)
	id, err := c.createID(ctx, path, interaction)
	if err != nil {
		return nil, err
	}
	interaction.ID = id
	return interaction, nil
}

func (c *Client) DeleteInteraction(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/2.0/interactions/"+id+"/", nil, nil)
	return err
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
	_, err := c.uploadMultipart(ctx, uploadPath, fileName, contentType, fileData, nil)
	return err
}
