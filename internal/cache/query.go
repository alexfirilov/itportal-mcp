package cache

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
)

// SearchResult is one hit returned by Search: the unified-index identity plus a
// short snippet of where the query matched.
type SearchResult struct {
	Type    string `json:"type"`
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
	URL     string `json:"url,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// Counts returns the per-type entity counts from the unified index.
func (s *Store) Counts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT type, COUNT(*) FROM entities GROUP BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}

// Index returns the compact index. When typ is non-empty it is restricted to that
// entity type; otherwise every type is returned in canonical order. limit/offset
// paginate; limit <= 0 means no limit.
func (s *Store) Index(typ string, limit, offset int) ([]IndexRow, int, error) {
	where, args := "", []any{}
	if typ != "" {
		where = "WHERE type = ?"
		args = append(args, typ)
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM entities "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Order by the canonical type sequence, then name, for a stable page.
	q := "SELECT type, id, name, summary, url FROM entities " + where +
		" ORDER BY " + typeOrderCase() + ", name COLLATE NOCASE, id"
	if limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []IndexRow{}
	for rows.Next() {
		var r IndexRow
		if err := rows.Scan(&r.Type, &r.ID, &r.Name, &r.Summary, &r.URL); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// typeOrderCase produces a CASE expression that sorts rows by entityKinds order.
func typeOrderCase() string {
	var b strings.Builder
	b.WriteString("CASE type")
	for i, k := range entityKinds {
		fmt.Fprintf(&b, " WHEN '%s' THEN %d", k, i)
	}
	b.WriteString(" ELSE 99 END")
	return b.String()
}

// Search resolves a query against the store. It prefers precise structured
// lookups (exact id, IP address, serial, or exact name) and otherwise falls back
// to an FTS5 keyword match. typ optionally restricts results to one entity type.
// limit <= 0 applies a sane default.
func (s *Store) Search(query, typ string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}
	if limit <= 0 {
		limit = 50
	}

	// 1. Exact device lookup by IP address.
	if net.ParseIP(query) != nil {
		if rs, err := s.byDeviceIP(query, limit); err != nil {
			return nil, err
		} else if len(rs) > 0 {
			return rs, nil
		}
	}

	// 2. Exact device lookup by serial number.
	if rs, err := s.byColumn("devices", "serial", query, "device", typ, limit); err != nil {
		return nil, err
	} else if len(rs) > 0 {
		return rs, nil
	}

	// 3. Exact name match across the index (e.g. a known hostname / company name).
	if rs, err := s.byExactName(query, typ, limit); err != nil {
		return nil, err
	} else if len(rs) > 0 {
		return rs, nil
	}

	// 4. FTS keyword search.
	return s.fts(query, typ, limit)
}

// byDeviceIP finds devices whose stored IP list contains ip. Device IPs are not
// part of the snapshot build, so this matches against the FTS body and the
// network address; it stays correct even when the precise per-device IP table is
// empty by also searching ipnetwork rows.
func (s *Store) byDeviceIP(ip string, limit int) ([]SearchResult, error) {
	// Use FTS for the IP token — unicode61 splits on dots so an IP is a phrase.
	return s.fts(`"`+ip+`"`, "", limit)
}

// byColumn returns index rows for entities of entType whose given column on table
// exactly equals val (case-insensitive). typ, if set and different from entType,
// suppresses the lookup.
func (s *Store) byColumn(table, column, val, entType, typ string, limit int) ([]SearchResult, error) {
	if typ != "" && typ != entType {
		return nil, nil
	}
	q := fmt.Sprintf(`SELECT e.type, e.id, e.name, e.summary, e.url
		FROM %s t JOIN entities e ON e.type = ? AND e.id = t.id
		WHERE t.%s = ? COLLATE NOCASE LIMIT ?`, table, column)
	return s.scanResults(q, entType, val, limit)
}

func (s *Store) byExactName(name, typ string, limit int) ([]SearchResult, error) {
	args := []any{name}
	q := `SELECT type, id, name, summary, url FROM entities WHERE name = ? COLLATE NOCASE`
	if typ != "" {
		q += " AND type = ?"
		args = append(args, typ)
	}
	q += " LIMIT ?"
	args = append(args, limit)
	return s.scanResults(q, args...)
}

// fts runs a full-text query. The raw query is sanitised into a safe FTS5 MATCH
// expression (terms ANDed, each prefix-matched) unless the caller already passed
// a quoted phrase.
func (s *Store) fts(query, typ string, limit int) ([]SearchResult, error) {
	match := buildMatch(query)
	if match == "" {
		return nil, nil
	}
	args := []any{match}
	q := `SELECT f.type, f.ref_id, f.name, f.summary, e.url,
		snippet(entities_fts, 4, '[', ']', '…', 12) AS snip
		FROM entities_fts f
		JOIN entities e ON e.type = f.type AND e.id = f.ref_id
		WHERE entities_fts MATCH ?`
	if typ != "" {
		q += " AND f.type = ?"
		args = append(args, typ)
	}
	q += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()
	out := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		var snip sql.NullString
		if err := rows.Scan(&r.Type, &r.ID, &r.Name, &r.Summary, &r.URL, &snip); err != nil {
			return nil, err
		}
		if snip.Valid {
			r.Snippet = strings.Join(strings.Fields(snip.String), " ")
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) scanResults(q string, args ...any) ([]SearchResult, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Type, &r.ID, &r.Name, &r.Summary, &r.URL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// buildMatch converts a free-text query into a safe FTS5 MATCH expression. If the
// query is already a quoted phrase it is passed through; otherwise each
// alphanumeric token is double-quoted (to neutralise FTS operators) and given a
// prefix wildcard, and the tokens are ANDed together.
func buildMatch(query string) string {
	query = strings.TrimSpace(query)
	if strings.HasPrefix(query, `"`) && strings.HasSuffix(query, `"`) && len(query) >= 2 {
		return query // explicit phrase
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !(r == '.' || r == '-' || r == '_' || isAlphaNum(r))
	})
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, ".-_")
		if f == "" {
			continue
		}
		terms = append(terms, `"`+f+`"*`)
	}
	return strings.Join(terms, " AND ")
}

func isAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// Relationships returns the directed links for a given entity (both incoming and
// outgoing), so a caller can see what an object is connected to.
func (s *Store) Relationships(typ string, id int) ([]Relationship, error) {
	rows, err := s.db.Query(`
		SELECT 'out' AS dir, dst_type, dst_id, dst_name, kind FROM relationships WHERE src_type = ? AND src_id = ?
		UNION ALL
		SELECT 'in' AS dir, src_type, src_id, src_name, kind FROM relationships WHERE dst_type = ? AND dst_id = ?`,
		typ, id, typ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Relationship{}
	for rows.Next() {
		var r Relationship
		if err := rows.Scan(&r.Direction, &r.Type, &r.ID, &r.Name, &r.Kind); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Relationship is one edge from the relationships table as seen from a focal
// entity. Direction is "out" (focal → other) or "in" (other → focal).
type Relationship struct {
	Direction string `json:"direction"`
	Type      string `json:"type"`
	ID        int    `json:"id"`
	Name      string `json:"name,omitempty"`
	Kind      string `json:"kind"`
}

// SectionJSON returns the full rows of one entity table as a JSON-friendly slice
// of maps, paginated. It is used to back the per-section snapshot resources.
func (s *Store) SectionJSON(table string, limit, offset int) ([]map[string]any, int, error) {
	if !validTable(table) {
		return nil, 0, fmt.Errorf("unknown section %q", table)
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := "SELECT * FROM " + table + " ORDER BY name COLLATE NOCASE, id"
	args := []any{}
	if limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}
	out := []map[string]any{}
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, 0, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			v := cells[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			// Drop empty strings to keep the payload compact.
			if sv, ok := v.(string); ok && sv == "" {
				continue
			}
			m[c] = v
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

// sectionTables maps a snapshot section name to its backing table.
var sectionTables = map[string]string{
	"companies":      "companies",
	"sites":          "sites",
	"devices":        "devices",
	"kbs":            "kbs",
	"contacts":       "contacts",
	"agreements":     "agreements",
	"ipnetworks":     "ipnetworks",
	"documents":      "documents",
	"accounts":       "accounts",
	"facilities":     "facilities",
	"cabinets":       "cabinets",
	"configurations": "configurations",
}

func validTable(t string) bool {
	for _, v := range sectionTables {
		if v == t {
			return true
		}
	}
	return false
}
