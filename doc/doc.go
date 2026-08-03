// Package doc loads, validates, and queries doc-IR Documents (agni.v1.doc): the
// intermediate decomposition of a source document (datasheet PDF, app note) that
// sits between the raw bytes and the parameter-IR. The schema lives in
// protos/agni/v1/doc/doc.proto; design and the two-tier query interface are in
// docs/21-document-ir.md. This package is tier 1: the deterministic in-process
// query surface recipes, tests, and revision diffing use. The service tier
// (corpus-wide lookup, full-text search) arrives with the extraction store.
package doc

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
)

// Load parses one Document in textproto form (the fixture and hand-authoring
// format; producers may emit binary proto instead and unmarshal directly). It only
// parses; call Validate for the semantic invariants.
func Load(r io.Reader) (*docpb.Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	d := &docpb.Document{}
	if err := prototext.Unmarshal(data, d); err != nil {
		return nil, fmt.Errorf("doc: parse Document: %w", err)
	}
	return d, nil
}

// Validate checks the invariants every doc-IR must hold before consumers may trust
// it: a document content hash and producer, page numbers unique and within
// page_count, region ids unique across the whole document, cells inside their
// table's grid with no duplicate positions, detection confidence in (0, 1], and
// every table's content_hash equal to TableHash (so revision diffing can trust
// stored hashes without recomputing). All violations are reported, joined into one
// error.
func Validate(d *docpb.Document) error {
	var errs []error
	if d.ContentHash == "" {
		errs = append(errs, errors.New("document has no content_hash (the derivation key)"))
	}
	if d.Producer == "" {
		errs = append(errs, errors.New("document has no producer"))
	}
	pages := map[int32]bool{}
	ids := map[string]bool{}
	uniq := func(id, what string) {
		if id == "" {
			errs = append(errs, fmt.Errorf("%s has no id", what))
			return
		}
		if ids[id] {
			errs = append(errs, fmt.Errorf("duplicate region id %q", id))
		}
		ids[id] = true
	}
	for _, pg := range d.Pages {
		if pg.Number < 1 || (d.PageCount > 0 && pg.Number > d.PageCount) {
			errs = append(errs, fmt.Errorf("page number %d outside [1, %d]", pg.Number, d.PageCount))
		}
		if pages[pg.Number] {
			errs = append(errs, fmt.Errorf("duplicate page number %d", pg.Number))
		}
		pages[pg.Number] = true
		for _, tb := range pg.TextBlocks {
			uniq(tb.Id, "text block")
		}
		for _, f := range pg.Figures {
			uniq(f.Id, "figure")
			if f.Confidence <= 0 || f.Confidence > 1 {
				errs = append(errs, fmt.Errorf("figure %s: confidence %v outside (0, 1]", f.Id, f.Confidence))
			}
		}
		for _, t := range pg.Tables {
			uniq(t.Id, "table")
			if t.Confidence <= 0 || t.Confidence > 1 {
				errs = append(errs, fmt.Errorf("table %s: confidence %v outside (0, 1]", t.Id, t.Confidence))
			}
			if t.Rows < 1 || t.Cols < 1 {
				errs = append(errs, fmt.Errorf("table %s: empty grid %dx%d", t.Id, t.Rows, t.Cols))
			}
			seen := map[[2]int32]bool{}
			for _, c := range t.Cells {
				rs, cs := span(c.RowSpan), span(c.ColSpan)
				if c.Row < 0 || c.Col < 0 || c.Row+rs > t.Rows || c.Col+cs > t.Cols {
					errs = append(errs, fmt.Errorf("table %s: cell (%d,%d) span (%d,%d) outside grid %dx%d",
						t.Id, c.Row, c.Col, rs, cs, t.Rows, t.Cols))
				}
				pos := [2]int32{c.Row, c.Col}
				if seen[pos] {
					errs = append(errs, fmt.Errorf("table %s: duplicate cell at grid (%d,%d)", t.Id, c.Row, c.Col))
				}
				seen[pos] = true
			}
			if want := TableHash(t); t.ContentHash != want {
				errs = append(errs, fmt.Errorf("table %s: content_hash %q does not match content (want %s)",
					t.Id, t.ContentHash, want))
			}
		}
	}
	return errors.Join(errs...)
}

// TablesMatching returns every table in page order whose title matches re. The
// recipe-layer primitive: recipes select tables by title pattern, never by id
// (ids are not stable across producer versions).
func TablesMatching(d *docpb.Document, re *regexp.Regexp) []*docpb.Table {
	var out []*docpb.Table
	for _, pg := range d.Pages {
		for _, t := range pg.Tables {
			if re.MatchString(t.Title) {
				out = append(out, t)
			}
		}
	}
	return out
}

// TableByID returns the table with the given region id, or nil. Ids address regions
// within one derivation (crops, review queue items), not across derivations.
func TableByID(d *docpb.Document, id string) *docpb.Table {
	for _, pg := range d.Pages {
		for _, t := range pg.Tables {
			if t.Id == id {
				return t
			}
		}
	}
	return nil
}

// FigureByID returns the figure with the given region id, or nil.
func FigureByID(d *docpb.Document, id string) *docpb.Figure {
	for _, pg := range d.Pages {
		for _, f := range pg.Figures {
			if f.Id == id {
				return f
			}
		}
	}
	return nil
}

// CellAt returns the cell whose top-left grid position is (row, col), or nil. A
// merged cell appears only at its top-left position; positions covered by a span
// return nil on purpose, so consumers see the merge instead of a phantom duplicate.
func CellAt(t *docpb.Table, row, col int32) *docpb.Cell {
	for _, c := range t.Cells {
		if c.Row == row && c.Col == col {
			return c
		}
	}
	return nil
}

// CellText returns the text at (row, col), or "" when the position is empty or
// covered by a merged cell's span.
func CellText(t *docpb.Table, row, col int32) string {
	if c := CellAt(t, row, col); c != nil {
		return c.Text
	}
	return ""
}

// PageText returns the page's text blocks joined by newlines, in document order.
// The full-text-search source: an index built over PageText covers everything the
// producer read outside tables, without re-parsing the source document.
func PageText(d *docpb.Document, number int32) string {
	for _, pg := range d.Pages {
		if pg.Number != number {
			continue
		}
		parts := make([]string, 0, len(pg.TextBlocks))
		for _, tb := range pg.TextBlocks {
			parts = append(parts, tb.Text)
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// TableHash is the content identity of a table: a sha256 over its grid shape and
// cell (position, span, text) tuples in grid order, excluding bboxes, ids,
// confidence, and header flags, which are derivation artifacts. Two derivations of
// the same printed table hash equal even if detection nudged coordinates, which is
// what lets revision diffing skip unchanged tables (WS10-007).
func TableHash(t *docpb.Table) string {
	cells := make([]*docpb.Cell, len(t.Cells))
	copy(cells, t.Cells)
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Row != cells[j].Row {
			return cells[i].Row < cells[j].Row
		}
		return cells[i].Col < cells[j].Col
	})
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d", t.Rows, t.Cols)
	for _, c := range cells {
		fmt.Fprintf(h, "\x1f%d,%d,%d,%d\x1e%s", c.Row, c.Col, span(c.RowSpan), span(c.ColSpan), c.Text)
	}
	for _, fn := range t.Footnotes {
		fmt.Fprintf(h, "\x1ffn\x1e%s", fn)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// FindTableForProv resolves a parameter provenance locator (page number plus the
// table label as the encoder wrote it) to a doc-IR table. The label matches a
// table when it equals the title, or when either contains the other
// (case-insensitive): provenance written from a section-qualified reading
// ("Electrical Characteristics - On Characteristics") still resolves to the table
// titled "Electrical Characteristics". Returns nil when nothing on that page
// matches; callers treat that as a broken citation, not a soft miss.
func FindTableForProv(d *docpb.Document, page int32, label string) *docpb.Table {
	l := strings.ToLower(label)
	for _, pg := range d.Pages {
		if pg.Number != page {
			continue
		}
		for _, t := range pg.Tables {
			title := strings.ToLower(t.Title)
			if title != "" && (title == l || strings.Contains(l, title) || strings.Contains(title, l)) {
				return t
			}
		}
	}
	return nil
}

// span normalizes a proto span value: 0 (unset) means 1.
func span(s int32) int32 {
	if s < 1 {
		return 1
	}
	return s
}
