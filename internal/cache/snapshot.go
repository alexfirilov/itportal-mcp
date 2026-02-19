// Package cache maintains an in-memory snapshot of all ITPortal documentation.
// The snapshot is rebuilt periodically and served as an MCP Resource, enabling
// Anthropic prompt caching to cache the full documentation context on the client side.
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// Snapshot is an immutable point-in-time view of all ITPortal documentation.
type Snapshot struct {
	GeneratedAt    time.Time
	Markdown       string // Full documentation as LLM-friendly markdown (no passwords)
	Companies      []itportal.Company
	Sites          []itportal.Site
	Devices        []itportal.Device
	KBs            []itportal.KB
	Contacts       []itportal.Contact
	Agreements     []itportal.Agreement
	IPNetworks     []itportal.IPNetwork
	Documents      []itportal.Document
	Accounts       []itportal.Account
	Facilities     []itportal.Facility
	Cabinets       []itportal.Cabinet
	Configurations []itportal.Configuration
}

// Cache holds the current snapshot and refreshes it on a configurable schedule.
type Cache struct {
	client          *itportal.Client
	limitPerEntity  int
	refreshInterval time.Duration
	logger          *slog.Logger
	current         atomic.Pointer[Snapshot]
}

// New creates a Cache and performs an initial synchronous snapshot build.
// Returns an error if the initial build fails (e.g. ITPortal is unreachable).
func New(ctx context.Context, client *itportal.Client, limitPerEntity int, refreshInterval time.Duration, logger *slog.Logger) (*Cache, error) {
	c := &Cache{
		client:          client,
		limitPerEntity:  limitPerEntity,
		refreshInterval: refreshInterval,
		logger:          logger,
	}

	snap, err := c.build(ctx)
	if err != nil {
		return nil, fmt.Errorf("initial snapshot build: %w", err)
	}
	c.current.Store(snap)
	logger.Info("initial snapshot built",
		"companies", len(snap.Companies),
		"sites", len(snap.Sites),
		"devices", len(snap.Devices),
		"kbs", len(snap.KBs),
		"contacts", len(snap.Contacts),
		"agreements", len(snap.Agreements),
		"ip_networks", len(snap.IPNetworks),
		"documents", len(snap.Documents),
		"accounts", len(snap.Accounts),
		"facilities", len(snap.Facilities),
		"cabinets", len(snap.Cabinets),
		"configurations", len(snap.Configurations),
	)
	return c, nil
}

// Get returns the current snapshot. Safe for concurrent use; never returns nil
// after New succeeds.
func (c *Cache) Get() *Snapshot {
	return c.current.Load()
}

// Refresh forces an immediate snapshot rebuild, blocking until complete.
func (c *Cache) Refresh(ctx context.Context) (*Snapshot, error) {
	snap, err := c.build(ctx)
	if err != nil {
		return nil, err
	}
	c.current.Store(snap)
	c.logger.Info("snapshot refreshed manually",
		"companies", len(snap.Companies),
		"sites", len(snap.Sites),
		"devices", len(snap.Devices),
		"kbs", len(snap.KBs),
		"contacts", len(snap.Contacts),
		"agreements", len(snap.Agreements),
		"ip_networks", len(snap.IPNetworks),
		"documents", len(snap.Documents),
		"accounts", len(snap.Accounts),
		"facilities", len(snap.Facilities),
		"cabinets", len(snap.Cabinets),
		"configurations", len(snap.Configurations),
	)
	return snap, nil
}

// StartBackgroundRefresh launches a goroutine that rebuilds the snapshot every
// refreshInterval. It respects ctx cancellation for clean shutdown.
func (c *Cache) StartBackgroundRefresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.logger.Info("background snapshot refresh started")
				snap, err := c.build(ctx)
				if err != nil {
					c.logger.Error("background snapshot refresh failed", "error", err)
					continue
				}
				c.current.Store(snap)
				c.logger.Info("background snapshot refresh complete",
					"companies", len(snap.Companies),
					"sites", len(snap.Sites),
					"devices", len(snap.Devices),
					"kbs", len(snap.KBs),
					"contacts", len(snap.Contacts),
					"agreements", len(snap.Agreements),
					"ip_networks", len(snap.IPNetworks),
					"documents", len(snap.Documents),
					"accounts", len(snap.Accounts),
					"facilities", len(snap.Facilities),
					"cabinets", len(snap.Cabinets),
					"configurations", len(snap.Configurations),
				)
			}
		}
	}()
}

// build fetches all entity types from ITPortal concurrently and assembles an immutable Snapshot.
func (c *Cache) build(ctx context.Context) (*Snapshot, error) {
	buildCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	lim := c.limitPerEntity
	var (
		companies      []itportal.Company
		sites          []itportal.Site
		devices        []itportal.Device
		kbs            []itportal.KB
		contacts       []itportal.Contact
		agreements     []itportal.Agreement
		ipNetworks     []itportal.IPNetwork
		documents      []itportal.Document
		accounts       []itportal.Account
		facilities     []itportal.Facility
		cabinets       []itportal.Cabinet
		configurations []itportal.Configuration
	)

	eg, egCtx := errgroup.WithContext(buildCtx)

	eg.Go(func() error {
		var err error
		companies, err = c.client.ListAllCompanies(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list companies: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		sites, err = c.client.ListAllSites(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list sites: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		devices, err = c.client.ListAllDevices(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list devices: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		kbs, err = c.client.ListAllKBs(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list KBs: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		contacts, err = c.client.ListAllContacts(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list contacts: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		agreements, err = c.client.ListAllAgreements(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list agreements: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		ipNetworks, err = c.client.ListAllIPNetworks(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list IP networks: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		documents, err = c.client.ListAllDocuments(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list documents: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		accounts, err = c.client.ListAllAccounts(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		facilities, err = c.client.ListAllFacilities(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list facilities: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		cabinets, err = c.client.ListAllCabinets(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list cabinets: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		configurations, err = c.client.ListAllConfigurations(egCtx, nil, lim)
		if err != nil {
			return fmt.Errorf("list configurations: %w", err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	snap := &Snapshot{
		GeneratedAt:    time.Now().UTC(),
		Companies:      companies,
		Sites:          sites,
		Devices:        devices,
		KBs:            kbs,
		Contacts:       contacts,
		Agreements:     agreements,
		IPNetworks:     ipNetworks,
		Documents:      documents,
		Accounts:       accounts,
		Facilities:     facilities,
		Cabinets:       cabinets,
		Configurations: configurations,
	}
	snap.Markdown = buildMarkdown(snap)
	return snap, nil
}

// buildMarkdown renders the snapshot as structured Markdown optimised for LLM consumption.
// Sensitive fields (passwords, 2FA codes, raw credentials) are intentionally omitted.
func buildMarkdown(s *Snapshot) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# ITPortal Documentation Snapshot\n\n")
	fmt.Fprintf(&b, "_Generated: %s UTC_\n\n", s.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "**Summary:** %d companies · %d sites · %d devices · %d KB articles · %d contacts · %d agreements · %d IP networks · %d documents · %d accounts · %d facilities · %d cabinets · %d configurations\n\n",
		len(s.Companies), len(s.Sites), len(s.Devices), len(s.KBs), len(s.Contacts), len(s.Agreements), len(s.IPNetworks),
		len(s.Documents), len(s.Accounts), len(s.Facilities), len(s.Cabinets), len(s.Configurations))
	b.WriteString("---\n\n")

	// ---- Companies ----
	fmt.Fprintf(&b, "## Companies (%d)\n\n", len(s.Companies))
	for _, co := range s.Companies {
		fmt.Fprintf(&b, "### %s (ID: %d)\n", co.Name, co.ID)
		if co.Abbreviation != "" {
			fmt.Fprintf(&b, "- **Code**: %s\n", co.Abbreviation)
		}
		if co.Status != "" {
			fmt.Fprintf(&b, "- **Status**: %s\n", co.Status)
		}
		if co.WebSite != "" {
			fmt.Fprintf(&b, "- **Website**: %s\n", co.WebSite)
		}
		if co.Description != "" {
			fmt.Fprintf(&b, "- **Description**: %s\n", truncate(co.Description, 300))
		}
		if co.Address != nil {
			fmt.Fprintf(&b, "- **Address**: %s\n", formatAddress(co.Address))
		}
		if co.StartDate != "" {
			fmt.Fprintf(&b, "- **Client Since**: %s\n", co.StartDate)
		}
		if co.Notes != "" {
			fmt.Fprintf(&b, "- **Notes**: %s\n", truncate(co.Notes, 300))
		}
		if co.RemoteAccessNotes != "" {
			fmt.Fprintf(&b, "- **Remote Access Notes**: %s\n", truncate(co.RemoteAccessNotes, 300))
		}
		if co.URL != "" {
			fmt.Fprintf(&b, "- **Portal Link**: %s\n", co.URL)
		}
		b.WriteString("\n")
	}

	// ---- Sites ----
	fmt.Fprintf(&b, "## Sites (%d)\n\n", len(s.Sites))
	for _, si := range s.Sites {
		companyCtx := ""
		if si.Company != nil {
			companyCtx = " — " + si.Company.Name
		}
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", si.Name, si.ID, companyCtx)
		if si.Company != nil {
			fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", si.Company.Name, si.Company.ID)
		}
		if si.Description != "" {
			fmt.Fprintf(&b, "- **Description**: %s\n", truncate(si.Description, 300))
		}
		if si.Address != nil {
			fmt.Fprintf(&b, "- **Address**: %s\n", formatAddress(si.Address))
		}
		if si.Contact != nil {
			fmt.Fprintf(&b, "- **Main Contact**: %s (ID: %d)\n", si.Contact.Name, si.Contact.ID)
		}
		if si.NumberOfPCs > 0 {
			fmt.Fprintf(&b, "- **Number of PCs**: %d\n", si.NumberOfPCs)
		}
		if si.URL != "" {
			fmt.Fprintf(&b, "- **Portal Link**: %s\n", si.URL)
		}
		b.WriteString("\n")
	}

	// ---- Devices ----
	fmt.Fprintf(&b, "## Devices (%d)\n\n", len(s.Devices))
	for _, d := range s.Devices {
		locationCtx := ""
		if d.Company != nil {
			locationCtx = d.Company.Name
			if d.Site != nil {
				locationCtx += " / " + d.Site.Name
			}
		}
		if locationCtx != "" {
			locationCtx = " — " + locationCtx
		}
		typeName := ""
		if d.Type != nil && d.Type.Name != "" {
			typeName = " [" + d.Type.Name + "]"
		}
		fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", d.Name, d.ID, typeName, locationCtx)
		if d.Company != nil {
			fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", d.Company.Name, d.Company.ID)
		}
		if d.Site != nil {
			fmt.Fprintf(&b, "- **Site**: %s (ID: %d)\n", d.Site.Name, d.Site.ID)
		}
		if d.Type != nil {
			fmt.Fprintf(&b, "- **Type**: %s\n", d.Type.Name)
		}
		hw := strings.TrimSpace(d.Manufacturer + " " + d.Model)
		if hw != "" {
			fmt.Fprintf(&b, "- **Hardware**: %s\n", hw)
		}
		if d.Serial != "" {
			fmt.Fprintf(&b, "- **Serial**: %s\n", d.Serial)
		}
		if d.Tag != "" {
			fmt.Fprintf(&b, "- **Tag**: %s\n", d.Tag)
		}
		if d.IMEI != "" {
			fmt.Fprintf(&b, "- **IMEI**: %s\n", d.IMEI)
		}
		if d.Description != "" {
			fmt.Fprintf(&b, "- **Description**: %s\n", truncate(d.Description, 300))
		}
		if d.Location != "" {
			fmt.Fprintf(&b, "- **Location**: %s\n", d.Location)
		}
		if d.InstallDate != "" {
			fmt.Fprintf(&b, "- **Install Date**: %s\n", d.InstallDate)
		}
		if d.WarrantyExpires != "" {
			fmt.Fprintf(&b, "- **Warranty Expires**: %s\n", d.WarrantyExpires)
		}
		if d.URL != "" {
			fmt.Fprintf(&b, "- **Portal Link**: %s\n", d.URL)
		}
		b.WriteString("\n")
	}

	// ---- Knowledge Base ----
	fmt.Fprintf(&b, "## Knowledge Base Articles (%d)\n\n", len(s.KBs))
	for _, kb := range s.KBs {
		companyCtx := ""
		if kb.Company != nil {
			companyCtx = " — " + kb.Company.Name
		}
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", kb.Name, kb.ID, companyCtx)
		if kb.Company != nil {
			fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", kb.Company.Name, kb.Company.ID)
		}
		if kb.Category != nil {
			fmt.Fprintf(&b, "- **Category**: %s\n", kb.Category.Name)
		}
		if kb.Description != "" {
			fmt.Fprintf(&b, "- **Content**: %s\n", truncate(kb.Description, 500))
		}
		if kb.Expires != "" {
			fmt.Fprintf(&b, "- **Expires**: %s\n", kb.Expires)
		}
		if kb.Modified != "" {
			fmt.Fprintf(&b, "- **Last Modified**: %s\n", kb.Modified)
		}
		if kb.URL != "" {
			fmt.Fprintf(&b, "- **Portal Link**: %s\n", kb.URL)
		}
		b.WriteString("\n")
	}

	// ---- Contacts ----
	fmt.Fprintf(&b, "## Contacts (%d)\n\n", len(s.Contacts))
	for _, co := range s.Contacts {
		fullName := strings.TrimSpace(co.FirstName + " " + co.LastName)
		if fullName == "" {
			fullName = fmt.Sprintf("Contact #%d", co.ID)
		}
		companyCtx := ""
		if co.Company != nil {
			companyCtx = " — " + co.Company.Name
		}
		fmt.Fprintf(&b, "### %s (ID: %d)%s\n", fullName, co.ID, companyCtx)
		if co.Company != nil {
			fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", co.Company.Name, co.Company.ID)
		}
		if co.Type != nil {
			fmt.Fprintf(&b, "- **Role**: %s\n", co.Type.Name)
		}
		if co.Email != "" {
			fmt.Fprintf(&b, "- **Email**: %s\n", co.Email)
		}
		if co.DirectNumber != "" {
			fmt.Fprintf(&b, "- **Direct**: %s\n", co.DirectNumber)
		}
		if co.Mobile != "" {
			fmt.Fprintf(&b, "- **Mobile**: %s\n", co.Mobile)
		}
		if co.Site != nil {
			fmt.Fprintf(&b, "- **Site**: %s\n", co.Site.Name)
		}
		b.WriteString("\n")
	}

	// ---- Agreements ----
	if len(s.Agreements) > 0 {
		fmt.Fprintf(&b, "## Agreements (%d)\n\n", len(s.Agreements))
		for _, ag := range s.Agreements {
			typeName := ""
			if ag.Type != nil {
				typeName = " [" + ag.Type.Name + "]"
			}
			companyCtx := ""
			if ag.Company != nil {
				companyCtx = " — " + ag.Company.Name
			}
			fmt.Fprintf(&b, "### Agreement ID: %d%s%s\n", ag.ID, typeName, companyCtx)
			if ag.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", ag.Company.Name, ag.Company.ID)
			}
			if ag.Description != "" {
				fmt.Fprintf(&b, "- **Description**: %s\n", truncate(ag.Description, 200))
			}
			if ag.Vendor != "" {
				fmt.Fprintf(&b, "- **Vendor**: %s\n", ag.Vendor)
			}
			if ag.DateExpires != "" {
				fmt.Fprintf(&b, "- **Expires**: %s\n", ag.DateExpires)
			}
			if ag.URL != "" {
				fmt.Fprintf(&b, "- **Portal Link**: %s\n", ag.URL)
			}
			b.WriteString("\n")
		}
	}

	// ---- IP Networks ----
	if len(s.IPNetworks) > 0 {
		fmt.Fprintf(&b, "## IP Networks (%d)\n\n", len(s.IPNetworks))
		for _, net := range s.IPNetworks {
			companyCtx := ""
			if net.Company != nil {
				companyCtx = " — " + net.Company.Name
			}
			fmt.Fprintf(&b, "### %s (ID: %d)%s\n", net.Name, net.ID, companyCtx)
			if net.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", net.Company.Name, net.Company.ID)
			}
			if net.Site != nil {
				fmt.Fprintf(&b, "- **Site**: %s\n", net.Site.Name)
			}
			if net.Network != "" || net.SubnetMask != "" {
				fmt.Fprintf(&b, "- **Network**: %s / %s\n", net.Network, net.SubnetMask)
			}
			if net.DefaultGateway != nil && net.DefaultGateway.IP != "" {
				fmt.Fprintf(&b, "- **Default Gateway**: %s\n", net.DefaultGateway.IP)
			}
			if net.DNSServer1 != nil && net.DNSServer1.IP != "" {
				fmt.Fprintf(&b, "- **DNS Primary**: %s\n", net.DNSServer1.IP)
			}
			if net.DNSServer2 != nil && net.DNSServer2.IP != "" {
				fmt.Fprintf(&b, "- **DNS Secondary**: %s\n", net.DNSServer2.IP)
			}
			if net.VlanID > 0 {
				fmt.Fprintf(&b, "- **VLAN**: %d\n", net.VlanID)
			}
			if net.Description != "" {
				fmt.Fprintf(&b, "- **Notes**: %s\n", truncate(net.Description, 200))
			}
			b.WriteString("\n")
		}
	}

	// ---- Documents ----
	if len(s.Documents) > 0 {
		fmt.Fprintf(&b, "## Documents (%d)\n\n", len(s.Documents))
		for _, doc := range s.Documents {
			companyCtx := ""
			if doc.Company != nil {
				companyCtx = " — " + doc.Company.Name
			}
			typeName := ""
			if doc.Type != nil {
				typeName = " [" + doc.Type.Name + "]"
			}
			fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", doc.Description, doc.ID, typeName, companyCtx)
			if doc.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", doc.Company.Name, doc.Company.ID)
			}
			if doc.Type != nil {
				fmt.Fprintf(&b, "- **Type**: %s\n", doc.Type.Name)
			}
			if doc.URLLink != "" {
				fmt.Fprintf(&b, "- **Link**: %s\n", doc.URLLink)
			}
			if doc.Modified != "" {
				fmt.Fprintf(&b, "- **Last Modified**: %s\n", doc.Modified)
			}
			if doc.URL != "" {
				fmt.Fprintf(&b, "- **Portal Link**: %s\n", doc.URL)
			}
			b.WriteString("\n")
		}
	}

	// ---- Accounts ----
	// Passwords and 2FA codes are intentionally omitted.
	if len(s.Accounts) > 0 {
		fmt.Fprintf(&b, "## Accounts (%d)\n\n", len(s.Accounts))
		for _, ac := range s.Accounts {
			companyCtx := ""
			if ac.Company != nil {
				companyCtx = " — " + ac.Company.Name
			}
			typeName := ""
			if ac.Type != nil {
				typeName = " [" + ac.Type.Name + "]"
			}
			heading := fmt.Sprintf("Account ID: %d%s%s", ac.ID, typeName, companyCtx)
			fmt.Fprintf(&b, "### %s\n", heading)
			if ac.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", ac.Company.Name, ac.Company.ID)
			}
			if ac.Type != nil {
				fmt.Fprintf(&b, "- **Type**: %s\n", ac.Type.Name)
			}
			if ac.Username != "" {
				fmt.Fprintf(&b, "- **Username**: %s\n", ac.Username)
			}
			if ac.AccountNumber != "" {
				fmt.Fprintf(&b, "- **Account Number**: %s\n", ac.AccountNumber)
			}
			if ac.Email != "" {
				fmt.Fprintf(&b, "- **Email**: %s\n", ac.Email)
			}
			if ac.Representative != "" {
				fmt.Fprintf(&b, "- **Representative**: %s\n", ac.Representative)
			}
			if ac.TechTelephone != "" {
				fmt.Fprintf(&b, "- **Tech Support**: %s\n", ac.TechTelephone)
			}
			if ac.SalesTelephone != "" {
				fmt.Fprintf(&b, "- **Sales**: %s\n", ac.SalesTelephone)
			}
			if ac.AccountURL != "" {
				fmt.Fprintf(&b, "- **Account URL**: %s\n", ac.AccountURL)
			}
			if ac.Expires != "" {
				fmt.Fprintf(&b, "- **Expires**: %s\n", ac.Expires)
			}
			if ac.Description != "" {
				fmt.Fprintf(&b, "- **Description**: %s\n", truncate(ac.Description, 300))
			}
			if ac.Notes != "" {
				fmt.Fprintf(&b, "- **Notes**: %s\n", truncate(ac.Notes, 300))
			}
			if ac.URL != "" {
				fmt.Fprintf(&b, "- **Portal Link**: %s\n", ac.URL)
			}
			b.WriteString("\n")
		}
	}

	// ---- Facilities ----
	if len(s.Facilities) > 0 {
		fmt.Fprintf(&b, "## Facilities (%d)\n\n", len(s.Facilities))
		for _, f := range s.Facilities {
			companyCtx := ""
			if f.Company != nil {
				companyCtx = " — " + f.Company.Name
			}
			typeName := ""
			if f.Type != nil {
				typeName = " [" + f.Type.Name + "]"
			}
			fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", f.Name, f.ID, typeName, companyCtx)
			if f.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", f.Company.Name, f.Company.ID)
			}
			if f.Site != nil {
				fmt.Fprintf(&b, "- **Site**: %s (ID: %d)\n", f.Site.Name, f.Site.ID)
			}
			if f.Type != nil {
				fmt.Fprintf(&b, "- **Type**: %s\n", f.Type.Name)
			}
			if f.Description != "" {
				fmt.Fprintf(&b, "- **Description**: %s\n", truncate(f.Description, 300))
			}
			if f.NumberOfUsers > 0 {
				fmt.Fprintf(&b, "- **Number of Users**: %d\n", f.NumberOfUsers)
			}
			if f.Address != nil {
				fmt.Fprintf(&b, "- **Address**: %s\n", formatAddress(f.Address))
			}
			if f.Notes != "" {
				fmt.Fprintf(&b, "- **Notes**: %s\n", truncate(f.Notes, 300))
			}
			if f.URL != "" {
				fmt.Fprintf(&b, "- **Portal Link**: %s\n", f.URL)
			}
			b.WriteString("\n")
		}
	}

	// ---- Cabinets ----
	if len(s.Cabinets) > 0 {
		fmt.Fprintf(&b, "## Cabinets (%d)\n\n", len(s.Cabinets))
		for _, cab := range s.Cabinets {
			companyCtx := ""
			if cab.Company != nil {
				companyCtx = " — " + cab.Company.Name
			}
			fmt.Fprintf(&b, "### %s (ID: %d)%s\n", cab.Name, cab.ID, companyCtx)
			if cab.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", cab.Company.Name, cab.Company.ID)
			}
			if cab.Site != nil {
				fmt.Fprintf(&b, "- **Site**: %s (ID: %d)\n", cab.Site.Name, cab.Site.ID)
			}
			if cab.Facility != nil {
				fmt.Fprintf(&b, "- **Facility**: %s (ID: %d)\n", cab.Facility.Name, cab.Facility.ID)
			}
			if cab.Contact != nil {
				fmt.Fprintf(&b, "- **Contact**: %s (ID: %d)\n", cab.Contact.Name, cab.Contact.ID)
			}
			if cab.Description != "" {
				fmt.Fprintf(&b, "- **Description**: %s\n", truncate(cab.Description, 300))
			}
			if cab.Address != nil {
				fmt.Fprintf(&b, "- **Address**: %s\n", formatAddress(cab.Address))
			}
			if cab.Notes != "" {
				fmt.Fprintf(&b, "- **Notes**: %s\n", truncate(cab.Notes, 300))
			}
			if cab.URL != "" {
				fmt.Fprintf(&b, "- **Portal Link**: %s\n", cab.URL)
			}
			b.WriteString("\n")
		}
	}

	// ---- Configurations ----
	if len(s.Configurations) > 0 {
		fmt.Fprintf(&b, "## Configurations (%d)\n\n", len(s.Configurations))
		for _, cfg := range s.Configurations {
			companyCtx := ""
			if cfg.Company != nil {
				companyCtx = " — " + cfg.Company.Name
			}
			typeName := ""
			if cfg.Type != nil {
				typeName = " [" + cfg.Type.Name + "]"
			}
			fmt.Fprintf(&b, "### %s (ID: %d)%s%s\n", cfg.Name, cfg.ID, typeName, companyCtx)
			if cfg.Company != nil {
				fmt.Fprintf(&b, "- **Company**: %s (ID: %d)\n", cfg.Company.Name, cfg.Company.ID)
			}
			if cfg.Type != nil {
				fmt.Fprintf(&b, "- **Type**: %s\n", cfg.Type.Name)
			}
			if cfg.Device != nil {
				fmt.Fprintf(&b, "- **Device**: %s (ID: %d)\n", cfg.Device.Name, cfg.Device.ID)
			}
			if cfg.InstallDate != "" {
				fmt.Fprintf(&b, "- **Install Date**: %s\n", cfg.InstallDate)
			}
			if cfg.DateExpires != "" {
				fmt.Fprintf(&b, "- **Expires**: %s\n", cfg.DateExpires)
			}
			if cfg.Notes != "" {
				fmt.Fprintf(&b, "- **Notes**: %s\n", truncate(cfg.Notes, 300))
			}
			if cfg.URL != "" {
				fmt.Fprintf(&b, "- **Portal Link**: %s\n", cfg.URL)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func formatAddress(a *itportal.Address) string {
	if a == nil {
		return ""
	}
	parts := []string{}
	if a.Address1 != "" {
		parts = append(parts, a.Address1)
	}
	if a.Address2 != "" {
		parts = append(parts, a.Address2)
	}
	city := strings.TrimSpace(a.City + " " + a.State + " " + a.Zip)
	if city != "" {
		parts = append(parts, city)
	}
	if a.Country != "" {
		parts = append(parts, a.Country)
	}
	return strings.Join(parts, ", ")
}

// truncate strips HTML and limits text to max runes for markdown embedding.
func truncate(s string, max int) string {
	// Normalise common HTML line breaks.
	s = strings.ReplaceAll(s, "<br>", " ")
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<br />", " ")
	s = strings.ReplaceAll(s, "</p>", " ")
	s = strings.ReplaceAll(s, "</div>", " ")
	// Strip remaining HTML tags.
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			sb.WriteRune(r)
		}
	}
	clean := strings.Join(strings.Fields(sb.String()), " ")
	runes := []rune(clean)
	if len(runes) <= max {
		return clean
	}
	return string(runes[:max]) + "…"
}
