package cache

import (
	"strings"
	"testing"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// sampleSnapshot returns a small but representative snapshot exercising every
// entity type, references between objects, and searchable fields (IP, serial).
func sampleSnapshot() *Snapshot {
	return &Snapshot{
		Companies: []itportal.Company{
			{ID: 1, Name: "Acme Corp", Status: "Active", Abbreviation: "ACME", WebSite: "acme.example"},
			{ID: 2, Name: "Globex", Status: "Active"},
		},
		Sites: []itportal.Site{
			{ID: 10, Name: "Acme HQ", Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}, NumberOfPCs: 42},
		},
		Devices: []itportal.Device{
			{ID: 100, Name: "fw01", Manufacturer: "Fortinet", Model: "FG-60F", Serial: "FGT60F123456",
				Type: &itportal.TypeItem{Name: "Firewall"}, Description: "Edge firewall for HQ",
				Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"},
				Site:    &itportal.SiteReference{ID: 10, Name: "Acme HQ"}},
			{ID: 101, Name: "sw-core", Manufacturer: "Cisco", Model: "C9300", Serial: "FCW2200ABCD",
				Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		KBs: []itportal.KB{
			{ID: 200, Name: "VPN Setup Runbook", Description: "How to configure the IPsec VPN tunnel on the Fortinet firewall.",
				Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}, Category: &itportal.KBCategory{Name: "Networking"}},
		},
		Contacts: []itportal.Contact{
			{ID: 300, FirstName: "Ada", LastName: "Byte", Email: "ada@acme.example", Mobile: "555-0100",
				Type: &itportal.ContactType{Name: "IT Manager"}, Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		Agreements: []itportal.Agreement{
			{ID: 400, Description: "Managed Services Contract", Vendor: "MSP Inc", DateExpires: "2027-01-01",
				Type: &itportal.AgreementType{Name: "Managed Services"}, Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		IPNetworks: []itportal.IPNetwork{
			{ID: 500, Name: "HQ LAN", NetworkAddress: "10.0.0.0", SubnetMask: "255.255.255.0", VlanID: 10,
				DefaultGateway: &itportal.IPRef{IP: "10.0.0.1"},
				Company:        &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		Documents: []itportal.Document{
			{ID: 600, Description: "Network Diagram", Type: &itportal.DocumentType{Name: "Diagram"},
				Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		Accounts: []itportal.Account{
			{ID: 700, Username: "svc-backup", Password: "SUPER-SECRET-PW", TwoFACode: "999111", Email: "svc@acme.example",
				Type: &itportal.AccountType{Name: "Service Account"}, Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		Facilities: []itportal.Facility{
			{ID: 800, Name: "DC-East", Type: &itportal.FacilityType{Name: "Data Center"},
				Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}},
		},
		Cabinets: []itportal.Cabinet{
			{ID: 900, Name: "Rack-A1", Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"},
				Facility: &itportal.FacilityReference{ID: 800, Name: "DC-East"}},
		},
		Configurations: []itportal.Configuration{
			{ID: 1000, Name: "fw01-baseline", Type: &itportal.ConfigurationType{Name: "Backup"},
				Device:  &itportal.DeviceReference{ID: 100, Name: "fw01"},
				Company: &itportal.CompanyReference{ID: 1, Name: "Acme Corp"}, DateExpires: "2026-12-31"},
		},
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := BuildStore(sampleSnapshot(), "") // in-memory
	if err != nil {
		t.Fatalf("BuildStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestBuildStoreLoadsAllEntities verifies the snapshot→SQLite load populates the
// unified index with the correct per-type counts.
func TestBuildStoreLoadsAllEntities(t *testing.T) {
	st := newTestStore(t)

	counts, err := st.Counts()
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	want := map[string]int{
		"company": 2, "site": 1, "device": 2, "kb": 1, "contact": 1, "agreement": 1,
		"ipnetwork": 1, "document": 1, "account": 1, "facility": 1, "cabinet": 1, "configuration": 1,
	}
	for typ, n := range want {
		if counts[typ] != n {
			t.Errorf("count[%s] = %d, want %d", typ, counts[typ], n)
		}
	}

	rows, total, err := st.Index("", 0, 0)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if total != 14 {
		t.Errorf("index total = %d, want 14", total)
	}
	if len(rows) != 14 {
		t.Errorf("index rows = %d, want 14", len(rows))
	}
	// Index order must start with companies (canonical type order).
	if rows[0].Type != "company" {
		t.Errorf("first index row type = %q, want company", rows[0].Type)
	}
}

// TestStoreNoSecrets guards that passwords/2FA never reach the SQLite store or its
// FTS index.
func TestStoreNoSecrets(t *testing.T) {
	st := newTestStore(t)
	for _, secret := range []string{"SUPER-SECRET-PW", "999111"} {
		if rs, _ := st.Search(secret, "", 50); len(rs) > 0 {
			t.Errorf("secret %q is searchable in the store (leaked)", secret)
		}
	}
	// The non-secret username must still be searchable.
	if rs, _ := st.Search("svc-backup", "", 50); len(rs) == 0 {
		t.Error("expected non-secret username svc-backup to be searchable")
	}
}

// TestSearchByIP resolves a device/network by exact IP address.
func TestSearchByIP(t *testing.T) {
	st := newTestStore(t)
	rs, err := st.Search("10.0.0.0", "", 50)
	if err != nil {
		t.Fatalf("Search ip: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("expected at least one hit for IP 10.0.0.0")
	}
	found := false
	for _, r := range rs {
		if r.Type == "ipnetwork" && r.ID == 500 {
			found = true
		}
	}
	if !found {
		t.Errorf("IP search did not return the HQ LAN ipnetwork; got %+v", rs)
	}
}

// TestSearchBySerial resolves a device by exact serial number.
func TestSearchBySerial(t *testing.T) {
	st := newTestStore(t)
	rs, err := st.Search("FGT60F123456", "", 50)
	if err != nil {
		t.Fatalf("Search serial: %v", err)
	}
	if len(rs) != 1 || rs[0].Type != "device" || rs[0].ID != 100 {
		t.Fatalf("serial search = %+v, want device 100", rs)
	}
}

// TestSearchByName resolves an object by exact name.
func TestSearchByName(t *testing.T) {
	st := newTestStore(t)
	rs, err := st.Search("fw01", "", 50)
	if err != nil {
		t.Fatalf("Search name: %v", err)
	}
	if len(rs) == 0 || rs[0].ID != 100 {
		t.Fatalf("name search = %+v, want device 100 first", rs)
	}
}

// TestSearchFTSKeyword exercises the FTS5 keyword path (a word that only appears
// in a description / body, not a name or exact field).
func TestSearchFTSKeyword(t *testing.T) {
	st := newTestStore(t)

	// "IPsec" appears only inside the KB article description.
	rs, err := st.Search("IPsec", "", 50)
	if err != nil {
		t.Fatalf("Search keyword: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("expected FTS hit for keyword IPsec")
	}
	if rs[0].Type != "kb" {
		t.Errorf("IPsec hit type = %q, want kb", rs[0].Type)
	}

	// Manufacturer keyword should match the device via FTS body.
	rs, err = st.Search("fortinet", "", 50)
	if err != nil {
		t.Fatalf("Search fortinet: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("expected hits for keyword fortinet")
	}
}

// TestSearchTypeFilter restricts results to a single entity type.
func TestSearchTypeFilter(t *testing.T) {
	st := newTestStore(t)
	rs, err := st.Search("acme", "company", 50)
	if err != nil {
		t.Fatalf("Search type filter: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("expected company hits for acme")
	}
	for _, r := range rs {
		if r.Type != "company" {
			t.Errorf("type filter leaked %q result", r.Type)
		}
	}
}

// TestRelationshipsDerived verifies inter-entity references are captured as links.
func TestRelationshipsDerived(t *testing.T) {
	st := newTestStore(t)

	// Configuration 1000 → device 100 (configures), → company 1 (belongs_to).
	rels, err := st.Relationships("configuration", 1000)
	if err != nil {
		t.Fatalf("Relationships: %v", err)
	}
	var sawDevice bool
	for _, r := range rels {
		if r.Direction == "out" && r.Type == "device" && r.ID == 100 && r.Kind == "configures" {
			sawDevice = true
		}
	}
	if !sawDevice {
		t.Errorf("configuration→device link missing; got %+v", rels)
	}

	// Device 100 should see the incoming configuration link.
	devRels, err := st.Relationships("device", 100)
	if err != nil {
		t.Fatalf("Relationships device: %v", err)
	}
	var sawIncoming bool
	for _, r := range devRels {
		if r.Direction == "in" && r.Type == "configuration" && r.ID == 1000 {
			sawIncoming = true
		}
	}
	if !sawIncoming {
		t.Errorf("device incoming configuration link missing; got %+v", devRels)
	}
}

// TestIndexPagination checks that the compact index paginates deterministically.
func TestIndexPagination(t *testing.T) {
	st := newTestStore(t)

	page1, total, err := st.Index("", 5, 0)
	if err != nil {
		t.Fatalf("Index page1: %v", err)
	}
	if total != 14 {
		t.Errorf("total = %d, want 14", total)
	}
	if len(page1) != 5 {
		t.Errorf("page1 len = %d, want 5", len(page1))
	}

	page2, _, err := st.Index("", 5, 5)
	if err != nil {
		t.Fatalf("Index page2: %v", err)
	}
	// Pages must not overlap.
	seen := map[string]bool{}
	for _, r := range page1 {
		seen[r.Type+":"+itoa(r.ID)] = true
	}
	for _, r := range page2 {
		if seen[r.Type+":"+itoa(r.ID)] {
			t.Errorf("page2 overlaps page1 at %s/%d", r.Type, r.ID)
		}
	}
}

// TestIndexTypeFilter restricts the index to one entity type.
func TestIndexTypeFilter(t *testing.T) {
	st := newTestStore(t)
	rows, total, err := st.Index("device", 0, 0)
	if err != nil {
		t.Fatalf("Index device: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("device index = %d rows (total %d), want 2", len(rows), total)
	}
	for _, r := range rows {
		if r.Type != "device" {
			t.Errorf("type filter leaked %q", r.Type)
		}
		if r.Summary == "" {
			t.Errorf("device %d missing summary", r.ID)
		}
	}
}

// TestIndexRowSizing asserts each compact index row stays small enough that the
// whole index fits the output limit. Summaries must be one short line.
func TestIndexRowSizing(t *testing.T) {
	st := newTestStore(t)
	rows, _, err := st.Index("", 0, 0)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	for _, r := range rows {
		if strings.Contains(r.Summary, "\n") {
			t.Errorf("%s/%d summary contains a newline (not one-line)", r.Type, r.ID)
		}
		// A single index row should be well under ~400 bytes (name + summary + url).
		if size := len(r.Name) + len(r.Summary) + len(r.URL); size > 400 {
			t.Errorf("%s/%d index row too large: %d bytes", r.Type, r.ID, size)
		}
	}
}

// TestSectionJSONPagination verifies per-section rows paginate and carry full
// columns without secrets.
func TestSectionJSONPagination(t *testing.T) {
	st := newTestStore(t)

	rows, total, err := st.SectionJSON("devices", 1, 0)
	if err != nil {
		t.Fatalf("SectionJSON: %v", err)
	}
	if total != 2 {
		t.Errorf("devices total = %d, want 2", total)
	}
	if len(rows) != 1 {
		t.Errorf("limited page len = %d, want 1", len(rows))
	}
	// Full column present.
	if _, ok := rows[0]["serial"]; !ok {
		t.Errorf("device row missing serial column; got keys %v", keys(rows[0]))
	}

	// Accounts section must never carry password/2fa columns.
	accs, _, err := st.SectionJSON("accounts", 0, 0)
	if err != nil {
		t.Fatalf("SectionJSON accounts: %v", err)
	}
	for _, a := range accs {
		for k := range a {
			if strings.Contains(strings.ToLower(k), "password") || strings.Contains(k, "2fa") {
				t.Errorf("account section exposes secret column %q", k)
			}
		}
	}
}

func TestSectionJSONUnknown(t *testing.T) {
	st := newTestStore(t)
	if _, _, err := st.SectionJSON("widgets", 0, 0); err == nil {
		t.Error("expected error for unknown section")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
