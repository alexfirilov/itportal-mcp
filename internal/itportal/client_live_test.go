package itportal

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLiveReadOnly exercises the client against a real ITPortal instance using
// credentials from the environment. It performs READ-ONLY calls only (no create,
// update or delete). It is skipped unless ITPORTAL_BASE_URL and ITPORTAL_API_KEY
// are set, so it never runs during normal `go test ./...`.
//
//	set -a; source .env; set +a
//	go test ./internal/itportal -run TestLiveReadOnly -v
func TestLiveReadOnly(t *testing.T) {
	base := os.Getenv("ITPORTAL_BASE_URL")
	key := os.Getenv("ITPORTAL_API_KEY")
	if base == "" || key == "" {
		t.Skip("ITPORTAL_BASE_URL / ITPORTAL_API_KEY not set; skipping live test")
	}

	version := os.Getenv("ITPORTAL_API_VERSION")
	c := NewClient(base, key,
		WithAPIVersion(version),
		WithEncryptionKey(os.Getenv("ITPORTAL_ENCRYPTION_KEY")),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. List companies (validates auth + version path + list envelope).
	companies, total, err := c.ListCompanies(ctx, &ListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	t.Logf("ListCompanies: got %d (page), reported total/count=%d", len(companies), total)
	if len(companies) == 0 {
		t.Log("no companies returned — portal may be empty; continuing")
	}

	// 2. Fetch one company (validates single-GET data.results[0] shape + v4 url).
	if len(companies) > 0 {
		id := strconv.Itoa(companies[0].ID)
		got, err := c.GetCompany(ctx, id)
		if err != nil {
			t.Fatalf("GetCompany(%s): %v", id, err)
		}
		t.Logf("GetCompany(%s): name=%q url=%q", id, got.Name, got.URL)
		if got.ID != companies[0].ID {
			t.Errorf("GetCompany id mismatch: got %d want %d", got.ID, companies[0].ID)
		}
		if got.URL != "" && !strings.Contains(got.URL, "/v4/") {
			t.Logf("note: company url %q does not contain /v4/ (v2.1 expected /v4/app/...)", got.URL)
		}
	}

	// 3. Cursor pagination across companies (validates nextCursor follow).
	all, err := c.ListAllCompanies(ctx, nil, 250)
	if err != nil {
		t.Fatalf("ListAllCompanies: %v", err)
	}
	t.Logf("ListAllCompanies(max 250): collected %d", len(all))
	seen := map[int]bool{}
	for _, co := range all {
		if seen[co.ID] {
			t.Errorf("duplicate company id %d — cursor pagination likely looping", co.ID)
			break
		}
		seen[co.ID] = true
	}

	// 4. Device types (validates GET /types/{kind}/).
	types, err := c.ListTypes(ctx, "device")
	if err != nil {
		t.Fatalf("ListTypes(device): %v", err)
	}
	t.Logf("ListTypes(device): %d types", len(types))

	// 5. Devices list + a relationships read (read-only v2.1 sub-resource).
	devices, _, err := c.ListDevices(ctx, &ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	t.Logf("ListDevices: got %d", len(devices))
	if len(devices) > 0 {
		id := strconv.Itoa(devices[0].ID)
		rels, err := c.ListRelationships(ctx, "devices", id)
		if err != nil {
			t.Fatalf("ListRelationships(devices/%s): %v", id, err)
		}
		t.Logf("ListRelationships(devices/%s): %d link(s)", id, len(rels))

		// Switch port ranges (validates the switchPortRanges endpoint + nested decode).
		ranges, err := c.ListSwitchPortRanges(ctx, id)
		if err != nil {
			t.Fatalf("ListSwitchPortRanges(devices/%s): %v", id, err)
		}
		t.Logf("ListSwitchPortRanges(devices/%s): %d range(s)", id, len(ranges))
	}

	// 6. Populated switch: set ITPORTAL_TEST_SWITCH_ID to a switch device id to
	// verify a real range decodes (name, port span, nested port descriptions).
	if swID := os.Getenv("ITPORTAL_TEST_SWITCH_ID"); swID != "" {
		ranges, err := c.ListSwitchPortRanges(ctx, swID)
		if err != nil {
			t.Fatalf("ListSwitchPortRanges(devices/%s): %v", swID, err)
		}
		if len(ranges) == 0 {
			t.Errorf("switch %s reported 0 port ranges — expected at least one", swID)
		}
		for _, r := range ranges {
			t.Logf("range id=%d name=%q ports %d-%d, %d switchPort(s), desc=%q",
				r.ID, r.Name, r.StartingPort, r.EndingPort, len(r.SwitchPorts), r.Description)
			for _, sp := range r.SwitchPorts {
				if sp.Description != "" {
					t.Logf("   port %d: %q", sp.PortNumber, sp.Description)
				}
			}
		}
	}
}
