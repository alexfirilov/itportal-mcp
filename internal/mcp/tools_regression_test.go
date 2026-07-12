package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// newHandler builds a Handler whose client points at the given test server.
func newHandler(url string) *Handler {
	c := itportal.NewClient(url, "secret")
	return &Handler{client: c, baseURL: url}
}

// writeList writes a v2.1-style list envelope (mirrors the itportal test helper).
func writeList[T any](w http.ResponseWriter, results []T, nextCursor string) {
	type data struct {
		Results    []T    `json:"results"`
		Count      int    `json:"count"`
		NextCursor string `json:"nextCursor,omitempty"`
	}
	body := struct {
		Code int  `json:"code"`
		Data data `json:"data"`
	}{Code: 200, Data: data{Results: results, Count: len(results), NextCursor: nextCursor}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// TestCreateDeviceDefaultsHostName reproduces BUG 1: the devices endpoint rejects
// a POST without hostName. create_device must default hostName to name.
func TestCreateDeviceDefaultsHostName(t *testing.T) {
	var posted itportal.Device
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&posted)
			w.Header().Set("Location", "/api/2.1/devices/42/")
			w.WriteHeader(http.StatusCreated)
			return
		}
		writeList(w, []itportal.Device{{ID: 42, Name: "graylog", HostName: "graylog"}}, "")
	}))
	defer srv.Close()

	h := newHandler(srv.URL)
	res, _, err := h.CreateDevice(context.Background(), nil, CreateDeviceInput{
		CompanyID: 3, Name: "graylog", TypeName: "Server",
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if posted.HostName != "graylog" {
		t.Errorf("hostName not defaulted to name: got %q, want %q", posted.HostName, "graylog")
	}
	if posted.Name != "graylog" {
		t.Errorf("name not sent: got %q", posted.Name)
	}
	_ = res
}

// TestCreateDeviceExplicitHostName verifies an explicit host_name overrides the
// name default.
func TestCreateDeviceExplicitHostName(t *testing.T) {
	var posted itportal.Device
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&posted)
			w.Header().Set("Location", "/api/2.1/devices/42/")
			w.WriteHeader(http.StatusCreated)
			return
		}
		writeList(w, []itportal.Device{{ID: 42, Name: "Graylog Server", HostName: "gl01"}}, "")
	}))
	defer srv.Close()

	h := newHandler(srv.URL)
	if _, _, err := h.CreateDevice(context.Background(), nil, CreateDeviceInput{
		CompanyID: 3, Name: "Graylog Server", HostName: "gl01",
	}); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if posted.HostName != "gl01" {
		t.Errorf("explicit host_name dropped: got %q, want %q", posted.HostName, "gl01")
	}
}

// TestCreateEntityAccountKeepsName reproduces BUG 2: create_entity for an account
// must pass the name field through to POST /accounts/.
func TestCreateEntityAccountKeepsName(t *testing.T) {
	var posted map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&posted)
			w.Header().Set("Location", "/api/2.1/accounts/55/")
			w.WriteHeader(http.StatusCreated)
			return
		}
		writeList(w, []itportal.Account{{ID: 55, Name: "Cloudflare"}}, "")
	}))
	defer srv.Close()

	h := newHandler(srv.URL)
	if _, _, err := h.CreateEntity(context.Background(), nil, CreateEntityInput{
		EntityType: "account",
		Fields:     map[string]interface{}{"name": "Cloudflare", "username": "ops@x"},
	}); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if posted["name"] != "Cloudflare" {
		t.Errorf("name dropped from account POST body: got %#v", posted["name"])
	}
}

// TestGetDeviceDetailsDedupesManagementURLs reproduces BUG 3: a device whose
// management-URL endpoint returns the same record many times must surface each
// distinct URL only once.
func TestGetDeviceDetailsDedupesManagementURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/managementUrls/"):
			dup := make([]itportal.DeviceMUrl, 0, 50)
			for i := 0; i < 50; i++ {
				dup = append(dup, itportal.DeviceMUrl{ID: 9, Title: "Web UI", URL: "https://10.0.0.1"})
			}
			writeList(w, dup, "")
		case strings.HasSuffix(r.URL.Path, "/ips/"):
			writeList(w, []itportal.DeviceIP{}, "")
		case strings.HasSuffix(r.URL.Path, "/notes/"):
			writeList(w, []itportal.DeviceNote{}, "")
		default:
			writeList(w, []itportal.Device{{ID: 139, Name: "fw01", URL: "https://p/v4/app/devices/139"}}, "")
		}
	}))
	defer srv.Close()

	h := newHandler(srv.URL)
	res, _, err := h.GetEntityDetails(context.Background(), nil, GetEntityInput{EntityType: "device", ID: "139"})
	if err != nil {
		t.Fatalf("GetEntityDetails: %v", err)
	}
	out := resultText(t, res)
	if n := strings.Count(out, `"https://10.0.0.1"`); n != 1 {
		t.Errorf("management URL appears %d times in output, want 1:\n%s", n, out)
	}
}

// TestDedupeManagementURLsByTitleURL covers records that lack an id.
func TestDedupeManagementURLsByTitleURL(t *testing.T) {
	in := []itportal.DeviceMUrl{
		{Title: "Web", URL: "https://a"},
		{Title: "Web", URL: "https://a"},
		{Title: "SSH", URL: "https://b"},
	}
	got := dedupeManagementURLs(in)
	if len(got) != 2 {
		t.Fatalf("got %d distinct URLs, want 2", len(got))
	}
}

// TestDedupeDeviceIPsByID covers the stale-cursor echo that returned every device
// IP twice (e.g. device 186's 4 IPs came back as 8 with duplicated ids).
func TestDedupeDeviceIPsByID(t *testing.T) {
	in := []itportal.DeviceIP{
		{ID: 601, IP: "10.0.0.1:9200"},
		{ID: 602, IP: "10.0.0.1:1515"},
		{ID: 601, IP: "10.0.0.1:9200"},
		{ID: 602, IP: "10.0.0.1:1515"},
	}
	got := dedupeDeviceIPs(in)
	if len(got) != 2 {
		t.Fatalf("got %d distinct IPs, want 2", len(got))
	}
}

// TestDedupeSwitchPortRangesByID covers the same stale-cursor echo on the
// switchPortRanges sub-resource, which returned every range twice (observed live
// on device 124: the single RJ range id=5 came back duplicated).
func TestDedupeSwitchPortRangesByID(t *testing.T) {
	in := []itportal.SwitchPortRange{
		{ID: 5, Name: "RJ", StartingPort: 1, EndingPort: 8},
		{ID: 5, Name: "RJ", StartingPort: 1, EndingPort: 8},
	}
	got := dedupeSwitchPortRanges(in)
	if len(got) != 1 {
		t.Fatalf("got %d distinct ranges, want 1", len(got))
	}
}

// TestGetDeviceDetailsDedupesIPs verifies the dedupe is wired into the device
// detail path so a doubled IP list surfaces each IP once in the output.
func TestGetDeviceDetailsDedupesIPs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ips/"):
			writeList(w, []itportal.DeviceIP{
				{ID: 601, IP: "10.0.0.1:9200", Description: "Indexer"},
				{ID: 601, IP: "10.0.0.1:9200", Description: "Indexer"},
			}, "")
		case strings.HasSuffix(r.URL.Path, "/managementUrls/"):
			writeList(w, []itportal.DeviceMUrl{}, "")
		case strings.HasSuffix(r.URL.Path, "/notes/"):
			writeList(w, []itportal.DeviceNote{}, "")
		default:
			writeList(w, []itportal.Device{{ID: 139, Name: "fw01", URL: "https://p/v4/app/devices/139"}}, "")
		}
	}))
	defer srv.Close()

	h := newHandler(srv.URL)
	res, _, err := h.GetEntityDetails(context.Background(), nil, GetEntityInput{EntityType: "device", ID: "139"})
	if err != nil {
		t.Fatalf("GetEntityDetails: %v", err)
	}
	out := resultText(t, res)
	if n := strings.Count(out, `"10.0.0.1:9200"`); n != 1 {
		t.Errorf("device IP appears %d times in output, want 1:\n%s", n, out)
	}
}
