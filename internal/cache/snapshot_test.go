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
