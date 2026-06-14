package itportal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient returns a client pointed at the given test server.
func newTestClient(url string, opts ...Option) *Client {
	return NewClient(url, "secret-key", opts...)
}

func TestBuildAuthHeader(t *testing.T) {
	wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte(":mykey"))
	cases := map[string]string{
		"mykey":          wantBasic,
		"  mykey  ":      wantBasic,
		"Bearer abc123":  "Bearer abc123",
		"Basic deadbeef": "Basic deadbeef",
	}
	for in, want := range cases {
		if got := buildAuthHeader(in); got != want {
			t.Errorf("buildAuthHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLocationID(t *testing.T) {
	cases := map[string]int{
		"/api/2.1/companies/42/":            42,
		"/api/2.1/devices/7":                7,
		"https://x/api/2.1/types/device/9/": 9,
		"":                                  0,
		"/no/number/here/":                  0,
	}
	for in, want := range cases {
		if got := parseLocationID(in); got != want {
			t.Errorf("parseLocationID(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestVersionRewriteAndAuth(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		writeList(w, []Company{{ID: 1, Name: "Acme"}}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithAPIVersion("2.1"))
	if _, _, err := c.ListCompanies(context.Background(), nil); err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if gotPath != "/api/2.1/companies/" {
		t.Errorf("path = %q, want /api/2.1/companies/", gotPath)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret-key"))
	if gotAuth != want {
		t.Errorf("auth = %q, want %q", gotAuth, want)
	}
}

func TestPatchContentTypeAndEncryptionHeader(t *testing.T) {
	var gotCT, gotEnc, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotEnc = r.Header.Get("X-Encryption-Key")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithEncryptionKey("enc123"))
	if err := c.UpdateCompany(context.Background(), "5", map[string]interface{}{"name": "x"}); err != nil {
		t.Fatalf("UpdateCompany: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotCT != "application/merge-patch+json" {
		t.Errorf("content-type = %q, want application/merge-patch+json", gotCT)
	}
	if gotEnc != "enc123" {
		t.Errorf("X-Encryption-Key = %q, want enc123", gotEnc)
	}
}

func TestNoEncryptionHeaderWhenUnset(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["X-Encryption-Key"]
		writeList(w, []Company{}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, _, err := c.ListCompanies(context.Background(), nil); err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if present {
		t.Error("X-Encryption-Key should be absent when no key is configured")
	}
}

func TestCursorPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			writeList(w, []Company{{ID: 1}, {ID: 2}}, "CUR2")
		case "CUR2":
			writeList(w, []Company{{ID: 3}}, "")
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
			writeList(w, []Company{}, "")
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	all, err := c.ListAllCompanies(context.Background(), nil, 100)
	if err != nil {
		t.Fatalf("ListAllCompanies: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d companies, want 3", len(all))
	}
	if all[2].ID != 3 {
		t.Errorf("third id = %d, want 3", all[2].ID)
	}
}

func TestListAllRespectsMaxItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always advertise another page; maxItems must stop the loop.
		writeList(w, []Company{{ID: 1}, {ID: 2}}, "more")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	all, err := c.ListAllCompanies(context.Background(), nil, 3)
	if err != nil {
		t.Fatalf("ListAllCompanies: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d, want 3 (capped)", len(all))
	}
}

func TestGetOneReadsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/2.1/devices/9/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeList(w, []Device{{ID: 9, Name: "fw01", URL: "https://p/v4/app/devices/9"}}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	d, err := c.GetDevice(context.Background(), "9")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if d.ID != 9 || d.Name != "fw01" {
		t.Errorf("got %+v", d)
	}
}

func TestCreateParsesLocationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/api/2.1/types/device/77/")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	id, err := c.CreateType(context.Background(), "device", "Router")
	if err != nil {
		t.Fatalf("CreateType: %v", err)
	}
	if id != 77 {
		t.Errorf("id = %d, want 77", id)
	}
}

func TestCreateThenGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "/api/2.1/companies/100/")
			w.WriteHeader(http.StatusCreated)
			return
		}
		// GET back
		if r.URL.Path != "/api/2.1/companies/100/" {
			t.Errorf("get path = %q", r.URL.Path)
		}
		writeList(w, []Company{{ID: 100, Name: "NewCo", URL: "https://p/v4/app/companies/100"}}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	created, err := c.CreateCompany(context.Background(), &Company{Name: "NewCo"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	if created.ID != 100 || created.URL == "" {
		t.Errorf("got %+v, want id=100 with url", created)
	}
}

func TestErrorStatusSurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"400","errors":[{"message":"requiredField"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, _, err := c.ListCompanies(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error on 400")
	}
	if !strings.Contains(err.Error(), "requiredField") {
		t.Errorf("error %q should contain server body", err.Error())
	}
}

func TestGetKBReturnsArticle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/2.1/kbs/39/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeList(w, []KB{{
			ID:          39,
			Name:        "Backup runbook",
			Description: "short synopsis",
			Article:     "<h2>Steps</h2><p>note body</p>",
		}}, "")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	kb, err := c.GetKB(context.Background(), "39")
	if err != nil {
		t.Fatalf("GetKB: %v", err)
	}
	if kb.Article != "<h2>Steps</h2><p>note body</p>" {
		t.Errorf("Article = %q, want note body HTML", kb.Article)
	}
	if kb.Description != "short synopsis" {
		t.Errorf("Description = %q", kb.Description)
	}
}

// writeList writes a v2.1-style list envelope.
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
