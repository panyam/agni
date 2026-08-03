// Package derive is the deterministic extraction stage of the datasheet pipeline
// (docs/24-derivation.md): PartSpec = f(document, toolchain, recipes, patches). It
// consumes a doc-IR (agni.v1.doc), classifies tables through declarative recipes,
// tokenizes rows into parameter-IR rows, applies pinned human patches LAST (so a
// verified fix can never regress), and emits the PartSpec together with a
// RunManifest that pins the inputs and lists every gap — what the run saw and did
// not extract. Pure data-in data-out, no I/O (CONSTRAINTS C1); loaders take fs.FS.
//
// Trust posture: derived rows carry method "derive/v0" and confidence < 1 (only a
// human verification earns 1.0), and rows from tables with no test-conditions
// channel stay ConditionCoverage UNSPECIFIED — under-specified until verified —
// because a stress table's header defaults ("TA = 25C unless otherwise noted") are
// conditions this stage cannot prove it captured.
package derive

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/doc"
	"github.com/panyam/agni/param"
)

// Version is the derive stage's toolchain pin, recorded in every RunManifest. Bump
// on any behavior change: the golden agreement tests are the regression gate.
const Version = "derive/v0"

// Confidence is stamped on every derived parameter. Deliberately below 1: the
// verification queue (WS10-002 follow-up) upgrades a human-confirmed row to
// method "human-verified", confidence 1.
const Confidence = 0.9

// Identity is the part identity the operator supplies for a derivation: the doc-IR
// does not know what part it describes, the seeder does.
type Identity struct {
	MPN          string
	Manufacturer string
	DeviceClass  string
}

// Run derives a PartSpec from a doc-IR: attach titles to untitled tables (nearest
// heading above — real producers emit datasheet tables untitled), classify them
// through the recipes matching the document, apply patches, tokenize rows, validate,
// and emit spec + manifest. The error return is for structural failures (an invalid
// recipe regex, a spec that fails param.Validate — a bug, not a data gap); data-level
// shortfalls are never errors, they are manifest gaps.
func Run(d *docpb.Document, recipes []*derivepb.Recipe, patches []*derivepb.Patch, id Identity) (*parampb.PartSpec, *derivepb.RunManifest, error) {
	if id.MPN == "" {
		return nil, nil, errors.New("derive: identity.MPN is required (the join key of the emitted spec)")
	}
	work := proto.Clone(d).(*docpb.Document)

	manifest := &derivepb.RunManifest{
		DocContentHash: work.ContentHash,
		DocProducer:    work.Producer,
		DeriveVersion:  Version,
		Mpn:            id.MPN,
		Manufacturer:   id.Manufacturer,
	}
	spec := &parampb.PartSpec{
		Mpn:          id.MPN,
		Manufacturer: id.Manufacturer,
		DeviceClass:  id.DeviceClass,
		Docs: []*parampb.SourceDoc{{
			Id:      "src",
			Title:   work.Title,
			Vendor:  id.Manufacturer,
			Locator: work.ContentHash,
		}},
	}

	rules, err := matchRecipes(work.Title, recipes, manifest)
	if err != nil {
		return nil, nil, err
	}
	applied, err := applyPatches(work, patches, manifest)
	if err != nil {
		return nil, nil, err
	}
	_ = applied

	for _, pg := range work.Pages {
		for _, t := range pg.Tables {
			kind := parampb.LimitKind_LIMIT_KIND_UNSPECIFIED
			candidates := candidateTitles(pg, t)
			for _, title := range candidates {
				if k := classify(title, rules); k != parampb.LimitKind_LIMIT_KIND_UNSPECIFIED {
					kind, t.Title = k, title
					break
				}
			}
			if kind == parampb.LimitKind_LIMIT_KIND_UNSPECIFIED {
				near := ""
				if len(candidates) > 0 {
					near = candidates[0]
				}
				manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
					Kind: "unclassified-table", Region: t.Id,
					Detail: fmt.Sprintf("no recipe rule matched any candidate title (nearest: %q)", near),
				})
				continue
			}
			extractTable(spec, manifest, pg, t, kind)
		}
	}
	manifest.ParametersEmitted = int32(len(spec.Parameters))
	if err := param.Validate(spec); err != nil {
		return nil, nil, fmt.Errorf("derive: emitted spec fails validation (a derive bug, not a data gap): %w", err)
	}
	return spec, manifest, nil
}

// candidateTitles returns the plausible titles for a table, best-first, for
// classification to try in order: the producer-attached title when present, then
// band cells (wide merged header cells in the top rows — real parsers fold a
// datasheet's section band INTO the table), then heading-like text blocks above the
// table (within 72pt, short, nearest first). Classification picks the FIRST
// candidate a recipe rule matches, which is what makes a note line sitting between
// the section heading and the table ("TA = 25C unless otherwise noted") harmless:
// it is a candidate, it just never matches a rule.
func candidateTitles(pg *docpb.Page, t *docpb.Table) []string {
	var out []string
	if t.Title != "" {
		out = append(out, t.Title)
	}
	for _, text := range bandCells(t) {
		out = append(out, text)
	}
	if t.Bbox != nil {
		type cand struct {
			gap  float64
			text string
		}
		var above []cand
		for _, tb := range pg.TextBlocks {
			if tb.Bbox == nil || strings.TrimSpace(tb.Text) == "" || len(tb.Text) > 80 {
				continue
			}
			gap := t.Bbox.Y - (tb.Bbox.Y + tb.Bbox.Height)
			// Small negative tolerance: real detected boxes touch or overlap by a
			// point or two (the BSS138 abs-max heading overlaps its table by 0.12pt).
			if gap < -6 || gap > 72 {
				continue
			}
			above = append(above, cand{gap, tb.Text})
		}
		sort.Slice(above, func(i, j int) bool { return above[i].gap < above[j].gap })
		for _, c := range above {
			out = append(out, c.text)
		}
	}
	return out
}

// bandCells returns the texts of band cells: cells in the rows above the header row
// that span more than one column (a folded-in section band). Their non-title texts
// are table-level conditions (see extractTable).
func bandCells(t *docpb.Table) []string {
	header := findHeaderRow(t)
	var out []string
	for _, c := range t.Cells {
		if c.Row < header && c.ColSpan > 1 && strings.TrimSpace(c.Text) != "" {
			out = append(out, strings.TrimSpace(c.Text))
		}
	}
	return out
}

// findHeaderRow locates the column-header row: the first of the top rows whose
// cells match at least two recognized column names. Real parsers put band rows
// above it; hand fixtures have it at row 0. Returns 0 when nothing matches (the
// detectColumns miss then lands the table in gaps).
func findHeaderRow(t *docpb.Table) int32 {
	for row := int32(0); row < min(t.Rows, 4); row++ {
		hits := 0
		for _, c := range t.Cells {
			if c.Row != row {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(c.Text)) {
			case "symbol", "parameter", "characteristic", "test conditions", "conditions",
				"min", "min.", "typ", "typ.", "max", "max.", "units", "unit", "ratings", "rating":
				hits++
			}
		}
		if hits >= 2 {
			return row
		}
	}
	return 0
}

// compiledRule is one recipe table rule with its pattern compiled and its limit-kind
// name resolved.
type compiledRule struct {
	re   *regexp.Regexp
	kind parampb.LimitKind
}

func matchRecipes(title string, recipes []*derivepb.Recipe, manifest *derivepb.RunManifest) ([]compiledRule, error) {
	var rules []compiledRule
	for _, r := range recipes {
		if r.DocTitlePattern == "" {
			continue
		}
		docRe, err := regexp.Compile(r.DocTitlePattern)
		if err != nil {
			return nil, fmt.Errorf("derive: recipe %s: doc_title_pattern: %w", r.Name, err)
		}
		if !docRe.MatchString(title) {
			continue
		}
		manifest.Recipes = append(manifest.Recipes, r.Name)
		for _, tr := range r.Tables {
			re, err := regexp.Compile(tr.TitlePattern)
			if err != nil {
				return nil, fmt.Errorf("derive: recipe %s: title_pattern %q: %w", r.Name, tr.TitlePattern, err)
			}
			kind, ok := parampb.LimitKind_value[tr.LimitKind]
			if !ok || kind == 0 {
				return nil, fmt.Errorf("derive: recipe %s: unknown limit_kind %q", r.Name, tr.LimitKind)
			}
			rules = append(rules, compiledRule{re, parampb.LimitKind(kind)})
		}
	}
	if len(rules) == 0 {
		manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
			Kind: "no-recipe", Detail: fmt.Sprintf("no recipe matched document title %q", title),
		})
	}
	return rules, nil
}

func classify(title string, rules []compiledRule) parampb.LimitKind {
	for _, r := range rules {
		if r.re.MatchString(title) {
			return r.kind
		}
	}
	return parampb.LimitKind_LIMIT_KIND_UNSPECIFIED
}

// applyPatches overwrites cell text for every patch whose (doc hash, pre-patch table
// content hash) key matches; patches that match the document but no table are
// recorded as "patch-unapplied" gaps (a revision or re-detection invalidated them).
func applyPatches(d *docpb.Document, patches []*derivepb.Patch, manifest *derivepb.RunManifest) (int, error) {
	applied := 0
	for _, p := range patches {
		if p.DocContentHash != d.ContentHash {
			continue
		}
		done := false
		for _, pg := range d.Pages {
			for _, t := range pg.Tables {
				if t.ContentHash != p.TableContentHash {
					continue
				}
				for _, c := range t.Cells {
					if c.Row == p.Row && c.Col == p.Col {
						c.Text = p.Text
						done = true
					}
				}
				if !done && p.Row >= 0 && p.Col >= 0 && p.Row < t.Rows && p.Col < t.Cols {
					// Insert-if-absent: a producer that mis-placed a value leaves the
					// correct position empty; the correction pair is a clear plus an
					// insert (the real LM1117 abs-max case).
					t.Cells = append(t.Cells, &docpb.Cell{Row: p.Row, Col: p.Col, Text: p.Text})
					done = true
				}
			}
		}
		if done {
			manifest.PatchesApplied = append(manifest.PatchesApplied, p.Name)
			applied++
		} else {
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "patch-unapplied", Detail: fmt.Sprintf("patch %s: no table with content hash %s", p.Name, p.TableContentHash),
			})
		}
	}
	sort.Strings(manifest.PatchesApplied)
	return applied, nil
}

// columns maps header names to column indexes for one table. Recognized headers are
// generic vendor conventions; a recipe-level override waits for a sheet that needs
// one.
type columns struct {
	symbol, name, cond, min, typ, max, unit, ratings int
}

func detectColumns(t *docpb.Table, headerRow int32) columns {
	c := columns{symbol: -1, name: -1, cond: -1, min: -1, typ: -1, max: -1, unit: -1, ratings: -1}
	for _, cell := range t.Cells {
		if cell.Row != headerRow {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(cell.Text)) {
		case "symbol":
			c.symbol = int(cell.Col)
		case "parameter", "characteristic":
			c.name = int(cell.Col)
		case "test conditions", "conditions":
			c.cond = int(cell.Col)
		case "min", "min.":
			c.min = int(cell.Col)
		case "typ", "typ.":
			c.typ = int(cell.Col)
		case "max", "max.":
			c.max = int(cell.Col)
		case "units", "unit":
			c.unit = int(cell.Col)
		case "ratings", "rating", "value":
			c.ratings = int(cell.Col)
		}
	}
	return c
}

// extractTable turns one classified table's rows into parameters. Merged cells
// (a symbol spanning its condition-set rows) carry down through their span; rows
// that yield no value bound land in gaps, never as empty parameters.
func extractTable(spec *parampb.PartSpec, manifest *derivepb.RunManifest, pg *docpb.Page, t *docpb.Table, kind parampb.LimitKind) {
	headerRow := findHeaderRow(t)
	cols := detectColumns(t, headerRow)
	// Band texts other than the chosen title are table-level conditions ("TA = 25C
	// unless otherwise noted"): every row inherits them, raw when they do not parse,
	// which keeps such rows machine-incomparable until verified — the honest state.
	var tableConds []*parampb.Condition
	for _, band := range bandCells(t) {
		if band == t.Title {
			continue
		}
		tableConds = append(tableConds, parseCondition(band))
	}
	if cols.symbol < 0 && cols.name < 0 {
		// TI-shaped tables label rows in an unlabeled column 0 ("Maximum input
		// voltage (VIN to GND) | MIN | MAX | UNIT"): fall back to column 0 as the
		// name column when it is not already claimed as a value column. Symbol stays
		// empty; the row is honest name-only data.
		if cols.ratings != 0 && cols.min != 0 && cols.typ != 0 && cols.max != 0 && cols.unit != 0 && cols.cond != 0 {
			cols.name = 0
		}
	}
	if (cols.symbol < 0 && cols.name < 0) || (cols.ratings < 0 && cols.min < 0 && cols.typ < 0 && cols.max < 0) {
		manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
			Kind: "unparsed-row", Region: t.Id,
			Detail: "header row lacks a symbol/name column or any value column",
		})
		return
	}
	cellAt := func(row int32, col int) *docpb.Cell {
		if col < 0 {
			return nil
		}
		// Exact hit first, else a merged cell whose span covers this row.
		if c := doc.CellAt(t, row, int32(col)); c != nil {
			return c
		}
		for _, c := range t.Cells {
			rs := max(c.RowSpan, 1)
			if c.Col == int32(col) && c.Row < row && c.Row+rs > row {
				return c
			}
		}
		return nil
	}
	text := func(row int32, col int) string {
		if c := cellAt(row, col); c != nil {
			return strings.TrimSpace(c.Text)
		}
		return ""
	}

	for row := headerRow + 1; row < t.Rows; row++ {
		symbol := normalizeSymbol(text(row, cols.symbol))
		name := text(row, cols.name)
		if symbol == "" && name == "" {
			continue
		}
		p := &parampb.Parameter{
			Name:      name,
			Symbol:    symbol,
			LimitKind: kind,
			Unit:      text(row, cols.unit),
			Prov: &parampb.ParamProvenance{
				DocRef: "src", Page: pg.Number, TableOrFigure: t.Title,
				Method: Version, Confidence: Confidence,
			},
		}
		val := &parampb.RangeValue{}
		if cols.ratings >= 0 {
			min, max, ok := parseRatings(text(row, cols.ratings))
			if !ok {
				manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
					Kind: "unparsed-row", Region: t.Id,
					Detail: fmt.Sprintf("row %d (%s): ratings cell %q did not parse", row, symbol, text(row, cols.ratings)),
				})
				continue
			}
			val.Min, val.Max = min, max
		} else {
			val.Min = parseNumberCell(text(row, cols.min))
			val.Typ = parseNumberCell(text(row, cols.typ))
			val.Max = parseNumberCell(text(row, cols.max))
		}
		if val.Min == nil && val.Typ == nil && val.Max == nil {
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "unparsed-row", Region: t.Id,
				Detail: fmt.Sprintf("row %d (%s): no value bound parsed", row, symbol),
			})
			continue
		}
		p.Value = val

		p.Conditions = append(p.Conditions, tableConds...)
		if cols.cond >= 0 {
			for part := range strings.SplitSeq(text(row, cols.cond), ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				c := parseCondition(part)
				if c.Eq == nil && c.Min == nil && c.Max == nil {
					manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
						Kind: "raw-condition", Region: t.Id,
						Detail: fmt.Sprintf("row %d (%s): condition kept as text: %q", row, symbol, part),
					})
				}
				p.Conditions = append(p.Conditions, c)
			}
		}
		if cols.cond >= 0 || len(tableConds) > 0 {
			// The condition channels present (column and/or band) were captured in
			// full, structured or raw, so the list is asserted complete; raw-only
			// members still make the row machine-incomparable, which is the intended
			// middle trust state.
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE
		}
		// No conditions channel at all: leave coverage UNSPECIFIED (under-specified
		// until a human verifies). Never UNCONDITIONAL: defaults this stage cannot
		// prove captured may qualify every row.

		spec.Parameters = append(spec.Parameters, p)
	}
}
