package itportal

import (
	"strconv"
	"strings"
)

// PortalPathSegment maps a normalised (lowercase, no underscores) entity type to
// its segment in the ITPortal v4 web-app URL (/v4/app/<segment>/<id>). Returns ""
// for unknown types. Confirmed against the live portal for devices and companies;
// the rest follow the plural API collection names.
func PortalPathSegment(itemType string) string {
	switch strings.ToLower(strings.ReplaceAll(itemType, "_", "")) {
	case "company":
		return "companies"
	case "site":
		return "sites"
	case "device":
		return "devices"
	case "kb", "knowledgebase":
		return "kbs"
	case "contact":
		return "contacts"
	case "account":
		return "accounts"
	case "agreement":
		return "agreements"
	case "document":
		return "documents"
	case "facility":
		return "facilities"
	case "cabinet":
		return "cabinets"
	case "configuration":
		return "configurations"
	case "ipnetwork":
		return "ipnetworks"
	}
	return ""
}

// BuildPortalURL constructs the v4 web-app deep-link for an object, or "" when the
// id is zero or the type is unknown. Callers should prefer an API-provided url and
// only fall back to this when that url is empty.
func BuildPortalURL(base, itemType string, id int) string {
	seg := PortalPathSegment(itemType)
	if seg == "" || id == 0 {
		return ""
	}
	return strings.TrimRight(base, "/") + "/v4/app/" + seg + "/" + strconv.Itoa(id)
}
