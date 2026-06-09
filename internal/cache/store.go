package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // pure-Go, cgo-free SQLite driver (FTS5 built in)

	"github.com/alexfirilov/itportal-mcp/internal/itportal"
)

// Store is an embedded SQLite view of a Snapshot. It normalises every entity into
// its own table, records inter-entity references in a relationships table, and
// maintains a unified `entities` index plus an FTS5 full-text index so callers can
// fetch a compact index and drill down by exact filter (id / name / ip / serial)
// or keyword search instead of scanning the megabyte markdown blob.
//
// A Store is built once per snapshot refresh and is immutable thereafter; reads
// are safe for concurrent use (database/sql guards the underlying *sql.DB).
type Store struct {
	db   *sql.DB
	path string // on-disk file, or "" for in-memory
}

// IndexRow is one line of the compact index: enough to identify an object and
// decide whether to drill into it, without loading its full record.
type IndexRow struct {
	Type    string `json:"type"`
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
	URL     string `json:"url,omitempty"`
}

// entityKinds is the ordered, canonical list of indexed entity types. The order
// drives the index output so it is stable across refreshes.
var entityKinds = []string{
	"company", "site", "device", "kb", "contact", "agreement",
	"ipnetwork", "document", "account", "facility", "cabinet", "configuration",
}

// StorePath returns the on-disk location for the snapshot database. It honours
// ITPORTAL_SNAPSHOT_DB when set, otherwise falls back to the user cache dir, then
// the OS temp dir. The returned directory is created if necessary.
func StorePath() string {
	if p := os.Getenv("ITPORTAL_SNAPSHOT_DB"); p != "" {
		return p
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "itportal-mcp")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "snapshot.db")
}

// BuildStore creates (or rebuilds) the SQLite database at path from snap. Passing
// an empty path builds a private in-memory database (used by tests). Any existing
// file at path is replaced so a refresh always reflects the latest snapshot.
func BuildStore(snap *Snapshot, path string) (*Store, error) {
	dsn := ":memory:"
	if path != "" {
		// Remove the stale file first so we never query half-old rows after a
		// schema change, then open fresh.
		_ = os.Remove(path)
		dsn = "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Serialise writes during the build; FTS5 + a single connection keep it simple.
	db.SetMaxOpenConns(1)

	s := &Store{db: db, path: path}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.load(snap); err != nil {
		db.Close()
		return nil, err
	}
	// Allow concurrent reads now that the build is complete.
	db.SetMaxOpenConns(0)
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

const schemaSQL = `
CREATE TABLE companies (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, status TEXT,
  abbreviation TEXT, website TEXT, address TEXT, notes TEXT, url TEXT
);
CREATE TABLE sites (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT,
  company_id INTEGER, company_name TEXT, address TEXT, contact TEXT, url TEXT
);
CREATE TABLE devices (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, type_name TEXT,
  company_id INTEGER, company_name TEXT, site_id INTEGER, site_name TEXT,
  manufacturer TEXT, model TEXT, serial TEXT, tag TEXT, imei TEXT,
  location TEXT, ips TEXT, url TEXT
);
CREATE TABLE kbs (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT,
  company_id INTEGER, company_name TEXT, category TEXT, content TEXT, url TEXT
);
CREATE TABLE contacts (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, role TEXT,
  company_id INTEGER, company_name TEXT, email TEXT, phone TEXT, url TEXT
);
CREATE TABLE agreements (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, type_name TEXT,
  company_id INTEGER, company_name TEXT, vendor TEXT, expires TEXT, serial TEXT, url TEXT
);
CREATE TABLE ipnetworks (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT,
  company_id INTEGER, company_name TEXT, site_name TEXT,
  network_address TEXT, subnet_mask TEXT, gateway TEXT, vlan INTEGER, url TEXT
);
CREATE TABLE documents (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, type_name TEXT,
  company_id INTEGER, company_name TEXT, link TEXT, url TEXT
);
CREATE TABLE accounts (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, type_name TEXT,
  company_id INTEGER, company_name TEXT, username TEXT, email TEXT,
  account_number TEXT, account_url TEXT, url TEXT
);
CREATE TABLE facilities (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, type_name TEXT,
  company_id INTEGER, company_name TEXT, site_name TEXT, address TEXT, url TEXT
);
CREATE TABLE cabinets (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT,
  company_id INTEGER, company_name TEXT, site_name TEXT,
  facility_name TEXT, contact_name TEXT, url TEXT
);
CREATE TABLE configurations (
  id INTEGER PRIMARY KEY, name TEXT, summary TEXT, type_name TEXT,
  company_id INTEGER, company_name TEXT, device_id INTEGER, device_name TEXT,
  install_date TEXT, expires TEXT, url TEXT
);

-- Unified compact index across every entity type.
CREATE TABLE entities (
  type TEXT NOT NULL, id INTEGER NOT NULL, name TEXT, summary TEXT, url TEXT,
  PRIMARY KEY (type, id)
);

-- Directed reference links derived from foreign keys embedded in each entity
-- (e.g. a configuration referencing its device, a device referencing its site).
CREATE TABLE relationships (
  src_type TEXT NOT NULL, src_id INTEGER NOT NULL, src_name TEXT,
  dst_type TEXT NOT NULL, dst_id INTEGER NOT NULL, dst_name TEXT,
  kind TEXT NOT NULL
);
CREATE INDEX idx_rel_src ON relationships(src_type, src_id);
CREATE INDEX idx_rel_dst ON relationships(dst_type, dst_id);

-- Full-text index over searchable text for every entity. The unindexed type/ref
-- columns carry the row identity back to the caller.
CREATE VIRTUAL TABLE entities_fts USING fts5(
  type UNINDEXED, ref_id UNINDEXED,
  name, summary, body,
  tokenize = 'unicode61'
);
`

func (s *Store) initSchema() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

// load populates every table from the snapshot inside a single transaction.
func (s *Store) load(snap *Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin load tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rolled back unless Commit succeeds

	idx, err := tx.Prepare(`INSERT INTO entities(type,id,name,summary,url) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer idx.Close()
	fts, err := tx.Prepare(`INSERT INTO entities_fts(type,ref_id,name,summary,body) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer fts.Close()
	rel, err := tx.Prepare(`INSERT INTO relationships(src_type,src_id,src_name,dst_type,dst_id,dst_name,kind) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer rel.Close()

	// index records the entity in the unified index and FTS table. body holds the
	// extra free-text (notes, descriptions, identifiers) that should match search
	// but does not belong in the one-line summary.
	index := func(typ string, id int, name, summary, url, body string) error {
		if _, err := idx.Exec(typ, id, name, summary, url); err != nil {
			return fmt.Errorf("index %s/%d: %w", typ, id, err)
		}
		if _, err := fts.Exec(typ, id, name, summary, body); err != nil {
			return fmt.Errorf("fts %s/%d: %w", typ, id, err)
		}
		return nil
	}
	link := func(srcType string, srcID int, srcName, dstType string, dstID int, dstName, kind string) error {
		if dstID == 0 {
			return nil
		}
		_, err := rel.Exec(srcType, srcID, srcName, dstType, dstID, dstName, kind)
		return err
	}

	for _, co := range snap.Companies {
		summary := companySummary(&co)
		if _, err := tx.Exec(`INSERT INTO companies(id,name,summary,status,abbreviation,website,address,notes,url) VALUES (?,?,?,?,?,?,?,?,?)`,
			co.ID, co.Name, summary, co.Status, co.Abbreviation, co.WebSite, formatAddress(co.Address), truncate(co.Notes, 500), co.URL); err != nil {
			return fmt.Errorf("insert company: %w", err)
		}
		body := strings.Join([]string{co.Abbreviation, co.WebSite, formatAddress(co.Address), truncate(co.Notes, 800), truncate(co.RemoteAccessNotes, 800), truncate(co.Description, 800)}, " ")
		if err := index("company", co.ID, co.Name, summary, co.URL, body); err != nil {
			return err
		}
	}

	for _, si := range snap.Sites {
		coID, coName := companyRef(si.Company)
		contact := ""
		if si.Contact != nil {
			contact = si.Contact.Name
		}
		summary := siteSummary(&si)
		if _, err := tx.Exec(`INSERT INTO sites(id,name,summary,company_id,company_name,address,contact,url) VALUES (?,?,?,?,?,?,?,?)`,
			si.ID, si.Name, summary, coID, coName, formatAddress(si.Address), contact, si.URL); err != nil {
			return fmt.Errorf("insert site: %w", err)
		}
		body := strings.Join([]string{coName, formatAddress(si.Address), contact, truncate(si.Description, 800)}, " ")
		if err := index("site", si.ID, si.Name, summary, si.URL, body); err != nil {
			return err
		}
		if err := link("site", si.ID, si.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, d := range snap.Devices {
		coID, coName := companyRef(d.Company)
		siteID, siteName := siteRef(d.Site)
		typeName := typeItemName(d.Type)
		summary := deviceSummary(&d)
		if _, err := tx.Exec(`INSERT INTO devices(id,name,summary,type_name,company_id,company_name,site_id,site_name,manufacturer,model,serial,tag,imei,location,ips,url) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.ID, d.Name, summary, typeName, coID, coName, siteID, siteName,
			d.Manufacturer, d.Model, d.Serial, d.Tag, d.IMEI, d.Location, "", d.URL); err != nil {
			return fmt.Errorf("insert device: %w", err)
		}
		body := strings.Join([]string{typeName, coName, siteName, d.Manufacturer, d.Model, d.Serial, d.Tag, d.IMEI, d.HostName, d.Domain, d.Location, truncate(d.Description, 800)}, " ")
		if err := index("device", d.ID, d.Name, summary, d.URL, body); err != nil {
			return err
		}
		if err := link("device", d.ID, d.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
		if err := link("device", d.ID, d.Name, "site", siteID, siteName, "located_at"); err != nil {
			return err
		}
	}

	for _, kb := range snap.KBs {
		coID, coName := companyRef(kb.Company)
		category := ""
		if kb.Category != nil {
			category = kb.Category.Name
		}
		summary := kbSummary(&kb)
		content := truncate(kb.Description, 2000)
		if _, err := tx.Exec(`INSERT INTO kbs(id,name,summary,company_id,company_name,category,content,url) VALUES (?,?,?,?,?,?,?,?)`,
			kb.ID, kb.Name, summary, coID, coName, category, content, kb.URL); err != nil {
			return fmt.Errorf("insert kb: %w", err)
		}
		body := strings.Join([]string{coName, category, content}, " ")
		if err := index("kb", kb.ID, kb.Name, summary, kb.URL, body); err != nil {
			return err
		}
		if err := link("kb", kb.ID, kb.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, c := range snap.Contacts {
		coID, coName := companyRef(c.Company)
		name := contactName(&c)
		role := typeContactName(c.Type)
		phone := firstNonEmpty(c.DirectNumber, c.Mobile, c.HomePhone)
		summary := contactSummary(&c)
		if _, err := tx.Exec(`INSERT INTO contacts(id,name,summary,role,company_id,company_name,email,phone,url) VALUES (?,?,?,?,?,?,?,?,?)`,
			c.ID, name, summary, role, coID, coName, c.Email, phone, c.URL); err != nil {
			return fmt.Errorf("insert contact: %w", err)
		}
		body := strings.Join([]string{coName, role, c.Email, c.DirectNumber, c.Mobile, c.HomePhone, truncate(c.Notes, 500)}, " ")
		if err := index("contact", c.ID, name, summary, c.URL, body); err != nil {
			return err
		}
		if err := link("contact", c.ID, name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, ag := range snap.Agreements {
		coID, coName := companyRef(ag.Company)
		typeName := ""
		if ag.Type != nil {
			typeName = ag.Type.Name
		}
		name := agreementName(&ag)
		summary := agreementSummary(&ag)
		if _, err := tx.Exec(`INSERT INTO agreements(id,name,summary,type_name,company_id,company_name,vendor,expires,serial,url) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			ag.ID, name, summary, typeName, coID, coName, ag.Vendor, ag.DateExpires, ag.SerialNumber, ag.URL); err != nil {
			return fmt.Errorf("insert agreement: %w", err)
		}
		body := strings.Join([]string{typeName, coName, ag.Vendor, ag.SerialNumber, truncate(ag.Description, 500), truncate(ag.Notes, 500)}, " ")
		if err := index("agreement", ag.ID, name, summary, ag.URL, body); err != nil {
			return err
		}
		if err := link("agreement", ag.ID, name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, net := range snap.IPNetworks {
		coID, coName := companyRef(net.Company)
		siteName := ""
		if net.Site != nil {
			siteName = net.Site.Name
		}
		gw := ""
		if net.DefaultGateway != nil {
			gw = net.DefaultGateway.IP
		}
		summary := ipNetworkSummary(&net)
		if _, err := tx.Exec(`INSERT INTO ipnetworks(id,name,summary,company_id,company_name,site_name,network_address,subnet_mask,gateway,vlan,url) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			net.ID, net.Name, summary, coID, coName, siteName, net.NetworkAddress, net.SubnetMask, gw, net.VlanID, net.URL); err != nil {
			return fmt.Errorf("insert ipnetwork: %w", err)
		}
		dns := ""
		if net.DNSServer1 != nil {
			dns += " " + net.DNSServer1.IP
		}
		if net.DNSServer2 != nil {
			dns += " " + net.DNSServer2.IP
		}
		body := strings.Join([]string{coName, siteName, net.NetworkAddress, net.SubnetMask, gw, dns, truncate(net.Description, 500)}, " ")
		if err := index("ipnetwork", net.ID, net.Name, summary, net.URL, body); err != nil {
			return err
		}
		if err := link("ipnetwork", net.ID, net.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, doc := range snap.Documents {
		coID, coName := companyRef(doc.Company)
		typeName := ""
		if doc.Type != nil {
			typeName = doc.Type.Name
		}
		name := documentName(&doc)
		summary := documentSummary(&doc)
		if _, err := tx.Exec(`INSERT INTO documents(id,name,summary,type_name,company_id,company_name,link,url) VALUES (?,?,?,?,?,?,?,?)`,
			doc.ID, name, summary, typeName, coID, coName, doc.URLLink, doc.URL); err != nil {
			return fmt.Errorf("insert document: %w", err)
		}
		body := strings.Join([]string{typeName, coName, doc.URLLink}, " ")
		if err := index("document", doc.ID, name, summary, doc.URL, body); err != nil {
			return err
		}
		if err := link("document", doc.ID, name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, ac := range snap.Accounts {
		coID, coName := companyRef(ac.Company)
		typeName := ""
		if ac.Type != nil {
			typeName = ac.Type.Name
		}
		name := accountName(&ac)
		summary := accountSummary(&ac)
		// Passwords / 2FA are intentionally never stored.
		if _, err := tx.Exec(`INSERT INTO accounts(id,name,summary,type_name,company_id,company_name,username,email,account_number,account_url,url) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			ac.ID, name, summary, typeName, coID, coName, ac.Username, ac.Email, ac.AccountNumber, ac.AccountURL, ac.URL); err != nil {
			return fmt.Errorf("insert account: %w", err)
		}
		body := strings.Join([]string{typeName, coName, ac.Username, ac.Email, ac.AccountNumber, ac.Representative, ac.AccountURL, truncate(ac.Description, 500), truncate(ac.Notes, 500)}, " ")
		if err := index("account", ac.ID, name, summary, ac.URL, body); err != nil {
			return err
		}
		if err := link("account", ac.ID, name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, f := range snap.Facilities {
		coID, coName := companyRef(f.Company)
		typeName := ""
		if f.Type != nil {
			typeName = f.Type.Name
		}
		siteName := ""
		if f.Site != nil {
			siteName = f.Site.Name
		}
		summary := facilitySummary(&f)
		if _, err := tx.Exec(`INSERT INTO facilities(id,name,summary,type_name,company_id,company_name,site_name,address,url) VALUES (?,?,?,?,?,?,?,?,?)`,
			f.ID, f.Name, summary, typeName, coID, coName, siteName, formatAddress(f.Address), f.URL); err != nil {
			return fmt.Errorf("insert facility: %w", err)
		}
		body := strings.Join([]string{typeName, coName, siteName, formatAddress(f.Address), truncate(f.Description, 500), truncate(f.Notes, 500)}, " ")
		if err := index("facility", f.ID, f.Name, summary, f.URL, body); err != nil {
			return err
		}
		if err := link("facility", f.ID, f.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
	}

	for _, cab := range snap.Cabinets {
		coID, coName := companyRef(cab.Company)
		siteName := ""
		if cab.Site != nil {
			siteName = cab.Site.Name
		}
		facilityName := ""
		if cab.Facility != nil {
			facilityName = cab.Facility.Name
		}
		contactName := ""
		if cab.Contact != nil {
			contactName = cab.Contact.Name
		}
		summary := cabinetSummary(&cab)
		if _, err := tx.Exec(`INSERT INTO cabinets(id,name,summary,company_id,company_name,site_name,facility_name,contact_name,url) VALUES (?,?,?,?,?,?,?,?,?)`,
			cab.ID, cab.Name, summary, coID, coName, siteName, facilityName, contactName, cab.URL); err != nil {
			return fmt.Errorf("insert cabinet: %w", err)
		}
		body := strings.Join([]string{coName, siteName, facilityName, contactName, formatAddress(cab.Address), truncate(cab.Description, 500), truncate(cab.Notes, 500)}, " ")
		if err := index("cabinet", cab.ID, cab.Name, summary, cab.URL, body); err != nil {
			return err
		}
		if err := link("cabinet", cab.ID, cab.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
		if cab.Facility != nil {
			if err := link("cabinet", cab.ID, cab.Name, "facility", cab.Facility.ID, facilityName, "in_facility"); err != nil {
				return err
			}
		}
	}

	for _, cfg := range snap.Configurations {
		coID, coName := companyRef(cfg.Company)
		typeName := ""
		if cfg.Type != nil {
			typeName = cfg.Type.Name
		}
		deviceID, deviceName := 0, ""
		if cfg.Device != nil {
			deviceID, deviceName = cfg.Device.ID, cfg.Device.Name
		}
		summary := configurationSummary(&cfg)
		if _, err := tx.Exec(`INSERT INTO configurations(id,name,summary,type_name,company_id,company_name,device_id,device_name,install_date,expires,url) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			cfg.ID, cfg.Name, summary, typeName, coID, coName, deviceID, deviceName, cfg.InstallDate, cfg.DateExpires, cfg.URL); err != nil {
			return fmt.Errorf("insert configuration: %w", err)
		}
		body := strings.Join([]string{typeName, coName, deviceName, truncate(cfg.Notes, 500)}, " ")
		if err := index("configuration", cfg.ID, cfg.Name, summary, cfg.URL, body); err != nil {
			return err
		}
		if err := link("configuration", cfg.ID, cfg.Name, "company", coID, coName, "belongs_to"); err != nil {
			return err
		}
		if err := link("configuration", cfg.ID, cfg.Name, "device", deviceID, deviceName, "configures"); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit load: %w", err)
	}
	return nil
}

// ---- reference helpers ----

func companyRef(c *itportal.CompanyReference) (int, string) {
	if c == nil {
		return 0, ""
	}
	return c.ID, c.Name
}

func siteRef(s *itportal.SiteReference) (int, string) {
	if s == nil {
		return 0, ""
	}
	return s.ID, s.Name
}

func typeItemName(t *itportal.TypeItem) string {
	if t == nil {
		return ""
	}
	return t.Name
}

func typeContactName(t *itportal.ContactType) string {
	if t == nil {
		return ""
	}
	return t.Name
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func contactName(c *itportal.Contact) string {
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name == "" {
		return "Contact #" + strconv.Itoa(c.ID)
	}
	return name
}

func agreementName(a *itportal.Agreement) string {
	if a.Description != "" {
		return truncate(a.Description, 80)
	}
	return "Agreement #" + strconv.Itoa(a.ID)
}

func documentName(d *itportal.Document) string {
	if d.Description != "" {
		return truncate(d.Description, 80)
	}
	return "Document #" + strconv.Itoa(d.ID)
}

func accountName(a *itportal.Account) string {
	if a.Username != "" {
		return a.Username
	}
	if a.Description != "" {
		return truncate(a.Description, 80)
	}
	return "Account #" + strconv.Itoa(a.ID)
}
