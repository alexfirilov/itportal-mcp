package itportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListAllStopsOnStaleCursor reproduces BUG 3: a sub-resource list endpoint
// that echoes back the same nextCursor regardless of the cursor we send. Before
// the fix, listAll re-fetched the same page and appended duplicate records until
// maxItems (e.g. a device with one management URL returned ~100 copies of it).
func TestListAllStopsOnStaleCursor(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Always return the same single record AND the same non-empty cursor,
		// ignoring the incoming cursor query param.
		writeList(w, []DeviceMUrl{{ID: 5, Title: "Web UI", URL: "https://10.0.0.1"}}, "STALE")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	urls, err := c.GetDeviceManagementURLs(context.Background(), "139")
	if err != nil {
		t.Fatalf("GetDeviceManagementURLs: %v", err)
	}
	// The stale-cursor guard must break the loop almost immediately instead of
	// running until maxItems (which previously produced ~100 duplicate records).
	// It tolerates at most the page that first revealed the repeating cursor;
	// the MCP layer dedupes the residual duplicate (see TestDedupe... in mcp).
	if len(urls) > 2 {
		t.Fatalf("got %d management URLs; stale cursor must not loop to maxItems", len(urls))
	}
	if calls > 2 {
		t.Errorf("made %d requests; stale-cursor loop should stop once the cursor repeats", calls)
	}
}

// TestAccountURLDecodesIntact is the evidence for BUG 4: when the API returns a
// well-formed accountUrl ("https://..."), the client decodes it verbatim. No
// MCP-side transform strips the leading "h". If this test passes while the live
// portal still shows "ttps://", the corruption is upstream in ITPortal's data.
func TestAccountURLDecodesIntact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeList(w, []Account{{ID: 1, AccountURL: "https://graylog.benarit.com"}}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	acc, err := c.GetAccount(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.AccountURL != "https://graylog.benarit.com" {
		t.Errorf("accountUrl mangled by client decode: got %q, want %q", acc.AccountURL, "https://graylog.benarit.com")
	}
}

// TestAccountNameRoundTrips guards BUG 2: the Account model must carry a name
// field with the correct json tag so create_entity's field passthrough reaches
// the accounts endpoint.
func TestAccountNameRoundTrips(t *testing.T) {
	var gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body Account
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotName = body.Name
			w.Header().Set("Location", "/api/2.1/accounts/77/")
			w.WriteHeader(http.StatusCreated)
			return
		}
		writeList(w, []Account{{ID: 77, Name: "Cloudflare"}}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	created, err := c.CreateAccount(context.Background(), &Account{Name: "Cloudflare"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if gotName != "Cloudflare" {
		t.Errorf("name not sent to POST /accounts/: got %q", gotName)
	}
	if created.Name != "Cloudflare" {
		t.Errorf("name not decoded back: got %q", created.Name)
	}
}
