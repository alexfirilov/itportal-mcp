package cache

import (
	"strings"
	"testing"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

func TestBuildMarkdownIncludesEntities(t *testing.T) {
	snap := &Snapshot{
		Companies: []itportal.Company{{ID: 1, Name: "Acme", Status: "Active"}},
		Devices: []itportal.Device{{
			ID: 9, Name: "fw01", Manufacturer: "Fortinet", Model: "FG-60F",
			Company: &itportal.CompanyReference{ID: 1, Name: "Acme"},
		}},
		IPNetworks: []itportal.IPNetwork{{
			ID: 3, Name: "LAN", NetworkAddress: "10.0.0.0", SubnetMask: "255.255.255.0",
		}},
	}
	md := buildMarkdown(snap)

	for _, want := range []string{"## Companies (1)", "Acme", "## Devices (1)", "fw01", "Fortinet FG-60F", "## IP Networks (1)", "10.0.0.0 / 255.255.255.0"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestTruncateStripsHTML(t *testing.T) {
	got := truncate("<p>hello <b>world</b></p>", 100)
	if got != "hello world" {
		t.Errorf("truncate stripped HTML = %q, want %q", got, "hello world")
	}
	long := strings.Repeat("a", 50)
	if out := truncate(long, 10); len([]rune(out)) != 11 { // 10 runes + ellipsis
		t.Errorf("truncate length = %d, want 11", len([]rune(out)))
	}
}

func TestBackfillPortalURLs(t *testing.T) {
	s := &Snapshot{
		Companies:      []itportal.Company{{ID: 1, Name: "Acme"}},
		Sites:          []itportal.Site{{ID: 2, Name: "HQ"}},
		Devices:        []itportal.Device{{ID: 9, Name: "fw01"}, {ID: 10, Name: "sw01", URL: "https://api-given/x"}},
		KBs:            []itportal.KB{{ID: 5, Name: "Runbook"}},
		Contacts:       []itportal.Contact{{ID: 4, FirstName: "Ada"}},
		Agreements:     []itportal.Agreement{{ID: 6}},
		IPNetworks:     []itportal.IPNetwork{{ID: 3, Name: "LAN"}},
		Documents:      []itportal.Document{{ID: 7, Description: "SOP"}},
		Accounts:       []itportal.Account{{ID: 8}},
		Facilities:     []itportal.Facility{{ID: 11, Name: "DC1"}},
		Cabinets:       []itportal.Cabinet{{ID: 12, Name: "Rack1"}},
		Configurations: []itportal.Configuration{{ID: 13, Name: "Cfg"}},
	}
	backfillPortalURLs(s, "https://portal.example")

	checks := []struct {
		got  string
		want string
	}{
		{s.Companies[0].URL, "https://portal.example/v4/app/companies/1"},
		{s.Sites[0].URL, "https://portal.example/v4/app/sites/2"},
		{s.Devices[0].URL, "https://portal.example/v4/app/devices/9"},
		{s.KBs[0].URL, "https://portal.example/v4/app/kbs/5"},
		{s.Contacts[0].URL, "https://portal.example/v4/app/contacts/4"},
		{s.Agreements[0].URL, "https://portal.example/v4/app/agreements/6"},
		{s.IPNetworks[0].URL, "https://portal.example/v4/app/ipnetworks/3"},
		{s.Documents[0].URL, "https://portal.example/v4/app/documents/7"},
		{s.Accounts[0].URL, "https://portal.example/v4/app/accounts/8"},
		{s.Facilities[0].URL, "https://portal.example/v4/app/facilities/11"},
		{s.Cabinets[0].URL, "https://portal.example/v4/app/cabinets/12"},
		{s.Configurations[0].URL, "https://portal.example/v4/app/configurations/13"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("backfilled url = %q, want %q", c.got, c.want)
		}
	}
	// API-provided URL must be preserved, not overwritten.
	if s.Devices[1].URL != "https://api-given/x" {
		t.Errorf("device 10 url overwritten = %q, want API value preserved", s.Devices[1].URL)
	}
}

func TestBuildMarkdownRendersHeadingLinks(t *testing.T) {
	snap := &Snapshot{
		Devices:    []itportal.Device{{ID: 9, Name: "fw01", URL: "https://portal.example/v4/app/devices/9"}},
		Contacts:   []itportal.Contact{{ID: 4, FirstName: "Ada", LastName: "Byte", URL: "https://portal.example/v4/app/contacts/4"}},
		IPNetworks: []itportal.IPNetwork{{ID: 3, Name: "LAN", URL: "https://portal.example/v4/app/ipnetworks/3"}},
	}
	md := buildMarkdown(snap)

	for _, want := range []string{
		"### [fw01](https://portal.example/v4/app/devices/9) (ID: 9)",
		"### [Ada Byte](https://portal.example/v4/app/contacts/4) (ID: 4)",
		"### [LAN](https://portal.example/v4/app/ipnetworks/3) (ID: 3)",
		"- **Portal Link**: https://portal.example/v4/app/contacts/4",
		"- **Portal Link**: https://portal.example/v4/app/ipnetworks/3",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestBuildMarkdownHeadingWithoutURLStaysPlain(t *testing.T) {
	snap := &Snapshot{Devices: []itportal.Device{{ID: 9, Name: "fw01"}}}
	md := buildMarkdown(snap)
	if !strings.Contains(md, "### fw01 (ID: 9)") {
		t.Errorf("plain heading missing; got:\n%s", md)
	}
}

func TestHeadingLinkEscapesBrackets(t *testing.T) {
	got := headingLink("UPS [APC]", "https://portal.example/v4/app/devices/9")
	want := `[UPS \[APC\]](https://portal.example/v4/app/devices/9)`
	if got != want {
		t.Errorf("headingLink = %q, want %q", got, want)
	}
	if headingLink("plain", "") != "plain" {
		t.Errorf("empty-url case should return plain name unchanged")
	}
}

// TestSnapshotMarkdownNoSecrets guards the security promise: passwords/2FA never
// appear in the snapshot even when present on the source records.
func TestSnapshotMarkdownNoSecrets(t *testing.T) {
	snap := &Snapshot{
		Accounts: []itportal.Account{{
			ID: 1, Username: "svc", Password: "SUPER-SECRET-PW", TwoFACode: "999111",
			Company: &itportal.CompanyReference{ID: 1, Name: "Acme"},
		}},
	}
	md := buildMarkdown(snap)
	if strings.Contains(md, "SUPER-SECRET-PW") || strings.Contains(md, "999111") {
		t.Error("snapshot markdown leaked a secret")
	}
	if !strings.Contains(md, "svc") {
		t.Error("expected non-secret username to be present")
	}
}
