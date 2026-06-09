package cache

import (
	"fmt"
	"strings"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// The *Summary helpers build the compact one-line description shown in the index
// and search results. They deliberately stay short (≈1 line) so the index fits
// inside the consumer's tool-output limit even with hundreds of rows.

func joinSummary(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " · ")
}

func companySummary(c *itportal.Company) string {
	return joinSummary(c.Status, c.Abbreviation, c.WebSite)
}

func siteSummary(s *itportal.Site) string {
	parts := []string{}
	if s.Company != nil {
		parts = append(parts, s.Company.Name)
	}
	if s.NumberOfPCs > 0 {
		parts = append(parts, fmt.Sprintf("%d PCs", s.NumberOfPCs))
	}
	parts = append(parts, formatAddress(s.Address))
	return joinSummary(parts...)
}

func deviceSummary(d *itportal.Device) string {
	parts := []string{}
	if d.Type != nil {
		parts = append(parts, d.Type.Name)
	}
	hw := strings.TrimSpace(d.Manufacturer + " " + d.Model)
	parts = append(parts, hw)
	loc := ""
	if d.Company != nil {
		loc = d.Company.Name
		if d.Site != nil {
			loc += "/" + d.Site.Name
		}
	}
	parts = append(parts, loc)
	if d.Serial != "" {
		parts = append(parts, "SN "+d.Serial)
	}
	return joinSummary(parts...)
}

func kbSummary(kb *itportal.KB) string {
	parts := []string{}
	if kb.Company != nil {
		parts = append(parts, kb.Company.Name)
	}
	if kb.Category != nil {
		parts = append(parts, kb.Category.Name)
	}
	parts = append(parts, truncate(kb.Description, 120))
	return joinSummary(parts...)
}

func contactSummary(c *itportal.Contact) string {
	parts := []string{}
	if c.Company != nil {
		parts = append(parts, c.Company.Name)
	}
	if c.Type != nil {
		parts = append(parts, c.Type.Name)
	}
	parts = append(parts, c.Email, firstNonEmpty(c.DirectNumber, c.Mobile))
	return joinSummary(parts...)
}

func agreementSummary(a *itportal.Agreement) string {
	parts := []string{}
	if a.Type != nil {
		parts = append(parts, a.Type.Name)
	}
	if a.Company != nil {
		parts = append(parts, a.Company.Name)
	}
	parts = append(parts, a.Vendor)
	if a.DateExpires != "" {
		parts = append(parts, "expires "+a.DateExpires)
	}
	return joinSummary(parts...)
}

func ipNetworkSummary(n *itportal.IPNetwork) string {
	parts := []string{}
	if n.Company != nil {
		parts = append(parts, n.Company.Name)
	}
	if n.NetworkAddress != "" {
		net := n.NetworkAddress
		if n.SubnetMask != "" {
			net += "/" + n.SubnetMask
		}
		parts = append(parts, net)
	}
	if n.VlanID > 0 {
		parts = append(parts, fmt.Sprintf("VLAN %d", n.VlanID))
	}
	return joinSummary(parts...)
}

func documentSummary(d *itportal.Document) string {
	parts := []string{}
	if d.Type != nil {
		parts = append(parts, d.Type.Name)
	}
	if d.Company != nil {
		parts = append(parts, d.Company.Name)
	}
	parts = append(parts, d.URLLink)
	return joinSummary(parts...)
}

func accountSummary(a *itportal.Account) string {
	parts := []string{}
	if a.Type != nil {
		parts = append(parts, a.Type.Name)
	}
	if a.Company != nil {
		parts = append(parts, a.Company.Name)
	}
	parts = append(parts, a.Username, a.Email)
	return joinSummary(parts...)
}

func facilitySummary(f *itportal.Facility) string {
	parts := []string{}
	if f.Type != nil {
		parts = append(parts, f.Type.Name)
	}
	if f.Company != nil {
		parts = append(parts, f.Company.Name)
	}
	parts = append(parts, formatAddress(f.Address))
	return joinSummary(parts...)
}

func cabinetSummary(c *itportal.Cabinet) string {
	parts := []string{}
	if c.Company != nil {
		parts = append(parts, c.Company.Name)
	}
	if c.Site != nil {
		parts = append(parts, c.Site.Name)
	}
	if c.Facility != nil {
		parts = append(parts, c.Facility.Name)
	}
	return joinSummary(parts...)
}

func configurationSummary(c *itportal.Configuration) string {
	parts := []string{}
	if c.Type != nil {
		parts = append(parts, c.Type.Name)
	}
	if c.Company != nil {
		parts = append(parts, c.Company.Name)
	}
	if c.Device != nil {
		parts = append(parts, "on "+c.Device.Name)
	}
	if c.DateExpires != "" {
		parts = append(parts, "expires "+c.DateExpires)
	}
	return joinSummary(parts...)
}
