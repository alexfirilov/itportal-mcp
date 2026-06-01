package itportal

import "testing"

func TestBuildPortalURL(t *testing.T) {
	const base = "https://portal.example"
	cases := []struct {
		name     string
		itemType string
		id       int
		want     string
	}{
		{"device", "device", 42, "https://portal.example/v4/app/devices/42"},
		{"company", "company", 7, "https://portal.example/v4/app/companies/7"},
		{"ipnetwork", "ipnetwork", 3, "https://portal.example/v4/app/ipnetworks/3"},
		{"kb alias", "knowledgebase", 5, "https://portal.example/v4/app/kbs/5"},
		{"zero id", "device", 0, ""},
		{"unknown type", "widget", 9, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildPortalURL(base, tc.itemType, tc.id); got != tc.want {
				t.Errorf("BuildPortalURL(%q, %d) = %q, want %q", tc.itemType, tc.id, got, tc.want)
			}
		})
	}
}

func TestBuildPortalURLTrimsTrailingSlash(t *testing.T) {
	if got := BuildPortalURL("https://portal.example/", "device", 1); got != "https://portal.example/v4/app/devices/1" {
		t.Errorf("trailing slash not trimmed: %q", got)
	}
}
