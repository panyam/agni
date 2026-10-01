// Package derive is the deterministic extraction stage of the datasheet pipeline:
// PartSpec = f(document, toolchain, recipes, patches). It consumes a doc-IR
// (agni.v1.doc), classifies tables through declarative recipes, tokenizes rows into
// parameter-IR rows, applies pinned human patches LAST so a verified fix cannot
// regress, and emits the PartSpec with a RunManifest that pins the inputs and lists
// every gap the run saw and did not extract. Pure data-in data-out, no I/O
// (CONSTRAINTS C1); loaders take fs.FS.
//
// The stages and the trust posture are in
// docsite/content/architecture/datasheet-layer.md#how-a-partspec-is-derived-from-a-document.
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
	"github.com/panyam/agni/datasheet/doc"
	"github.com/panyam/agni/core/param"
)

// Version is the derive stage's toolchain pin, recorded in every RunManifest. Bump
// it on any behavior change; the golden agreement tests are the regression gate.
const Version = "derive/v0"

// Confidence is stamped on every derived parameter. Below 1 because only a human
// verification (param.MarkVerified) earns 1.0.
const Confidence = 0.9

// Identity is the part identity the operator supplies for a derivation, because the
// doc-IR does not know what part it describes.
type Identity struct {
	MPN          string
	Manufacturer string
	DeviceClass  string
	// Locator is where this corpus keeps the document, for SourceDoc.locator. Run
	// takes a decoded Document and not a path, so it cannot know. Empty means the
	// corpus records no location.
	Locator string
}

// Run derives a PartSpec from a doc-IR: attach titles to untitled tables, classify
// them through the recipes matching the document, apply patches, tokenize rows,
// validate, and emit spec + manifest. The error return is for structural failures
// (an invalid recipe regex, or a spec that fails param.Validate, which is a derive
// bug). Data-level shortfalls are never errors; they are manifest gaps.
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
			Id: "src",
			// Title is NOT filled from the doc-IR. SourceDoc.title is the vendor's document
			// number and revision as printed, while producers fill Document.title with the PART
			// number, so copying it gave a citation that could not name its revision (agni issue
			// 290). It stays empty and gapUnidentifiedDocument records the refusal.
			Vendor: id.Manufacturer,
			// The revision this spec describes. param.VerificationOfIn compares a verification's
			// pinned hash against this one, so leaving it empty makes staleness "unknown" forever
			// (#285).
			ContentHash: work.ContentHash,
			Locator:     id.Locator,
		}},
	}
	gapUnidentifiedDocument(work, manifest)

	// matchRecipes reads the doc-IR title, which is the RIGHT use of it, because a recipe
	// selects documents by the part they describe and not by revision.
	rules, pinRules, err := matchRecipes(work.Title, recipes, manifest)
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
			// A pin function table carries terminals, not values, so it is offered to
			// the pin rules only after no parameter rule claimed it.
			pinTable := false
			if kind == parampb.LimitKind_LIMIT_KIND_UNSPECIFIED {
				for _, title := range candidates {
					if axis, ok := classifyPin(title, pinRules); ok {
						t.Title, pinTable = title, true
						extractPinTable(spec, manifest, pg, t, axis)
						break
					}
				}
			}
			if pinTable {
				continue
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
	manifest.PinsEmitted = int32(len(spec.Pins))
	manifest.PackagesEmitted = int32(len(spec.Packages))
	if err := param.Validate(spec); err != nil {
		return nil, nil, fmt.Errorf("derive: emitted spec fails validation (a derive bug, not a data gap): %w", err)
	}
	return spec, manifest, nil
}

// candidateTitles returns the plausible titles for a table, best-first: the
// producer-attached title, then band cells (wide merged header cells in the top rows,
// since real parsers fold a section band INTO the table), then short text blocks
// within 72pt above the table, nearest first. Classification takes the FIRST one a
// recipe rule matches, so a note line between heading and table ("TA = 25C unless
// otherwise noted") is a candidate that never matches.
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
			// Small negative tolerance, since real detected boxes touch or overlap by a
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

// bandCells returns the texts of band cells, the cells above the header row that
// span more than one column (a folded-in section band). Their non-title texts are
// table-level conditions (see extractTable).
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

// findHeaderRow locates the column-header row, the first of the top four rows whose
// cells match at least two recognized column names. Real parsers put band rows
// above it and hand fixtures have it at row 0. Returns 0 when nothing matches, and
// the detectColumns miss then lands the table in gaps.
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

func matchRecipes(title string, recipes []*derivepb.Recipe, manifest *derivepb.RunManifest) ([]compiledRule, []compiledPinRule, error) {
	var rules []compiledRule
	var pinRules []compiledPinRule
	for _, r := range recipes {
		if r.DocTitlePattern == "" {
			continue
		}
		docRe, err := regexp.Compile(r.DocTitlePattern)
		if err != nil {
			return nil, nil, fmt.Errorf("derive: recipe %s: doc_title_pattern: %w", r.Name, err)
		}
		if !docRe.MatchString(title) {
			continue
		}
		manifest.Recipes = append(manifest.Recipes, r.Name)
		for _, tr := range r.Tables {
			re, err := regexp.Compile(tr.TitlePattern)
			if err != nil {
				return nil, nil, fmt.Errorf("derive: recipe %s: title_pattern %q: %w", r.Name, tr.TitlePattern, err)
			}
			kind, ok := parampb.LimitKind_value[tr.LimitKind]
			if !ok || kind == 0 {
				return nil, nil, fmt.Errorf("derive: recipe %s: unknown limit_kind %q", r.Name, tr.LimitKind)
			}
			rules = append(rules, compiledRule{re, parampb.LimitKind(kind)})
		}
		for _, pr := range r.PinTables {
			re, err := regexp.Compile(pr.TitlePattern)
			if err != nil {
				return nil, nil, fmt.Errorf("derive: recipe %s: pin title_pattern %q: %w", r.Name, pr.TitlePattern, err)
			}
			pinRules = append(pinRules, compiledPinRule{re, pr.ColumnAxis})
		}
	}
	if len(rules) == 0 && len(pinRules) == 0 {
		manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
			Kind: "no-recipe", Detail: fmt.Sprintf("no recipe matched document title %q", title),
		})
	}
	return rules, pinRules, nil
}

// classifyPin reports whether a title names a pin function table, and what the recipe
// says its designator columns mean. The bool is the match, because an axis of UNSPECIFIED
// is a legitimate answer ("this is a pin table, do not read its columns as packages") and
// cannot double as the not-matched signal the way LimitKind's zero value does.
func classifyPin(title string, rules []compiledPinRule) (derivepb.PinColumnAxis, bool) {
	for _, r := range rules {
		if r.re.MatchString(title) {
			return r.axis, true
		}
	}
	return derivepb.PinColumnAxis_PIN_COLUMN_AXIS_UNSPECIFIED, false
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
					// Insert if absent. A producer that mis-placed a value leaves the
					// correct position empty, so the correction pair is a clear plus an
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
// generic vendor conventions, with no recipe-level override.
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
	// unless otherwise noted"). Every row inherits them, raw when they do not parse,
	// which keeps such rows machine-incomparable until verified.
	var tableConds []*parampb.Condition
	for _, band := range bandCells(t) {
		if band == t.Title {
			continue
		}
		tableConds = append(tableConds, parseCondition(band))
	}
	if cols.symbol < 0 && cols.name < 0 {
		// TI-shaped tables label rows in an unlabeled column 0 ("Maximum input
		// voltage (VIN to GND) | MIN | MAX | UNIT"), so fall back to column 0 as the
		// name column when no value column claims it. Symbol stays empty.
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
			// full, structured or raw, so the list is asserted complete. Raw-only
			// members still make the row machine-incomparable.
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE
		}
		// With no conditions channel, coverage stays UNSPECIFIED and never
		// UNCONDITIONAL, because header defaults this stage cannot prove captured
		// may qualify every row (datasheet-layer.md#trust-defaults).

		spec.Parameters = append(spec.Parameters, p)
	}
}
