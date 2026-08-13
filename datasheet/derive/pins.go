package derive

import (
	"fmt"
	"regexp"
	"strings"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The pin-table path. A pin FUNCTION table yields terminals rather than values, so it
// shares the title-attachment and classification machinery with the parameter path and
// nothing else: its columns are data-dependent, its cells are multi-valued, and a row
// can mean either one pin or several.
//
// Everything here is shaped by reading real producer output rather than the printed
// page, because the two differ in ways that matter. See the fixture header for the
// document this was built against.

// compiledPinRule is one recipe pin-table rule, pattern compiled.
type compiledPinRule struct {
	re   *regexp.Regexp
	axis derivepb.PinColumnAxis
}

// Header vocabularies. Small and closed in practice: measured across a 63-document
// corpus, pin tables label these four roles with the spellings below and little else.
var (
	pinNameHeaders   = map[string]bool{"name": true, "pin name": true, "signal name": true, "symbol": true}
	pinIOHeaders     = map[string]bool{"i/o": true, "type": true, "i/o type": true, "buffer type": true, "dir": true}
	pinDescHeaders   = map[string]bool{"description": true, "function": true, "pin function": true}
	pinNumberHeaders = map[string]bool{
		"no.": true, "no": true, "pin no.": true, "pin no": true, "pin #": true,
		"pin#": true, "pin num": true, "number": true, "pin": true,
	}
	// A banded header cell introducing the numbering columns.
	pinBandHeaders = map[string]bool{"pin": true, "pins": true, "pin no.": true, "no.": true}
)

// numberColumn is one column of designators, and the packages it speaks for. A single
// header cell routinely names SEVERAL packages ("D, PW"), which share one column of
// numbers, so this is a slice rather than one id.
type numberColumn struct {
	col      int
	raw      string
	packages []string
}

// pinColumns is the resolved layout of one pin table.
type pinColumns struct {
	name, io, desc int
	numbers        []numberColumn
	banded         bool
}

func headerKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	// Producers flatten footnote markers and stray glyphs into header text
	// ("DESCRIPTION (1)", "Alternate  Functions 1").
	s = regexp.MustCompile(`\s*\(\d+\)\s*$`).ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// findPinColumns resolves a pin table's layout. Two shapes occur and both are common:
// a FLAT header where one column is labelled "NO."/"PIN", and a BANDED header where a
// spanning "PIN" cell sits above several sub-columns of designators. In the banded
// shape the sub-columns carry no role word at all, so they are identified by position
// (inside the band, and not the name column) rather than by vocabulary.
func findPinColumns(t *docpb.Table, axis derivepb.PinColumnAxis) (int32, pinColumns, bool) {
	cols := pinColumns{name: -1, io: -1, desc: -1}
	headerRow := int32(-1)
	for row := int32(0); row < min(t.Rows, 4) && headerRow < 0; row++ {
		for _, c := range t.Cells {
			if c.Row == row && pinNameHeaders[headerKey(c.Text)] {
				headerRow = row
			}
		}
	}
	if headerRow < 0 {
		return 0, cols, false
	}

	// A band is a spanning cell ABOVE the header row whose text is PIN-ish. Its span
	// bounds the designator columns.
	bandStart, bandEnd := -1, -1
	for _, c := range t.Cells {
		if c.Row < headerRow && c.ColSpan > 1 && pinBandHeaders[headerKey(c.Text)] {
			bandStart, bandEnd = int(c.Col), int(c.Col+c.ColSpan-1)
			cols.banded = true
		}
	}

	var unlabelled []*docpb.Cell
	for _, c := range t.Cells {
		if c.Row != headerRow {
			continue
		}
		k := headerKey(c.Text)
		switch {
		case pinNameHeaders[k] && cols.name < 0:
			cols.name = int(c.Col)
		case pinIOHeaders[k] && cols.io < 0:
			cols.io = int(c.Col)
		case pinDescHeaders[k] && cols.desc < 0:
			cols.desc = int(c.Col)
		case pinNumberHeaders[k] && !cols.banded:
			cols.numbers = append(cols.numbers, numberColumn{col: int(c.Col), raw: strings.TrimSpace(c.Text)})
		default:
			unlabelled = append(unlabelled, c)
		}
	}
	if cols.banded {
		for _, c := range unlabelled {
			if int(c.Col) < bandStart || int(c.Col) > bandEnd || int(c.Col) == cols.name {
				continue
			}
			cols.numbers = append(cols.numbers, numberColumn{col: int(c.Col), raw: strings.TrimSpace(c.Text)})
		}
	}
	// Package codes come off the header cell, and only when the recipe says these
	// columns ARE packages. Under any other axis the codes are recorded raw and no
	// Package is minted, which is what keeps a variant column from becoming a body.
	if axis == derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE {
		for i := range cols.numbers {
			cols.numbers[i].packages = splitPackageCodes(cols.numbers[i].raw)
		}
	}
	if cols.name < 0 || len(cols.numbers) == 0 && cols.io < 0 && cols.desc < 0 {
		return headerRow, cols, false
	}
	return headerRow, cols, true
}

// splitPackageCodes turns one header cell into the package codes it names. "D, PW" is
// two packages sharing one column of designators; a cell naming none yields none.
func splitPackageCodes(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" || !isPackageCode(p) {
			continue
		}
		out = append(out, strings.ToUpper(p))
	}
	return out
}

var packageCodeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,5}$`)

func isPackageCode(s string) bool { return packageCodeRe.MatchString(s) }

// absentNumber reports a designator cell meaning "this pin is not in that body".
// Producers render the printed em-dash as an ASCII hyphen, so matching the typographic
// character alone would read every absence as an unparsed cell.
func absentNumber(s string) bool {
	switch strings.TrimSpace(s) {
	case "", "-", "--", "—", "–", "N/A", "n/a", "NA":
		return true
	}
	return false
}

// splitDesignators splits a designator cell into its numbers. ONLY on commas, never on
// whitespace: producers flatten a footnote marker into the cell as a trailing token, so
// "8 2" is pin 8 carrying footnote 2 while "2, 3" is genuinely two terminals. Splitting
// on whitespace invents a pin on every footnoted row.
func splitDesignators(s string) []string {
	if absentNumber(s) {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		// Drop a trailing footnote marker on an otherwise numeric designator.
		if f := strings.Fields(p); len(f) == 2 && isAllDigits(f[0]) && isAllDigits(f[1]) {
			p = f[0]
		}
		out = append(out, p)
	}
	return out
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// pinFunctionOf maps a table's own type column onto the closed PinFunction vocabulary.
//
// The fallback to name and description is deliberately NARROW: only NO_CONNECT. Real
// tables leave the type column blank or dashed on supply and ground rows, so inferring
// POWER_INPUT from a name like "VCCA" would be this stage inventing a classification
// the document did not make. A no-connect is different because the document states it
// in words ("No connection. Not internally connected."), and because the multi-number
// split rule below depends on knowing it.
func pinFunctionOf(ioRaw, name, desc string) parampb.PinFunction {
	switch headerKey(ioRaw) {
	case "i/o", "io", "d_io", "a_io", "bidirectional", "input/output":
		return parampb.PinFunction_PIN_FUNCTION_BIDIRECTIONAL
	case "i", "in", "input", "a_in", "d_in", "digital input", "analog input":
		return parampb.PinFunction_PIN_FUNCTION_INPUT
	case "o", "out", "output", "d_out", "a_out", "digital output":
		return parampb.PinFunction_PIN_FUNCTION_OUTPUT
	case "p", "pwr", "power", "supply":
		return parampb.PinFunction_PIN_FUNCTION_POWER_INPUT
	case "g", "gnd", "ground":
		return parampb.PinFunction_PIN_FUNCTION_GROUND
	case "nc", "n.c.", "no connect":
		return parampb.PinFunction_PIN_FUNCTION_NO_CONNECT
	}
	if isNoConnect(name, desc) {
		return parampb.PinFunction_PIN_FUNCTION_NO_CONNECT
	}
	return parampb.PinFunction_PIN_FUNCTION_UNSPECIFIED
}

func isNoConnect(name, desc string) bool {
	n := headerKey(name)
	if n == "nc" || n == "n.c." || n == "nc/dnc" {
		return true
	}
	return strings.HasPrefix(headerKey(desc), "no connection")
}

// normalizePinName repairs a pin name whose SUBSCRIPT was flattened with an injected
// space. Producers render "VCCA" as "V CCA", and the name is the channel that resolves a
// design pin to a spec pin, so leaving it split would make the join fail against every
// symbol library on exactly the multi-supply parts pin binding exists for.
//
// The join is deliberately narrow: every field must be a short all-caps alphanumeric
// token, which "V CCA" is and "Thermal pad" is not. A pin name is one token by
// convention; a multi-word entry is a descriptive label and keeps its spaces.
func normalizePinName(s string) string {
	f := strings.Fields(s)
	if len(f) < 2 {
		return strings.TrimSpace(s)
	}
	for _, part := range f {
		if len(part) > 4 || part != strings.ToUpper(part) || !isAlnum(part) {
			return strings.TrimSpace(s)
		}
	}
	return strings.Join(f, "")
}

func isAlnum(s string) bool {
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return s != ""
}

// pinIDFor derives a spec-local id from a pin's name, uniquely. A SPLIT pin takes the
// designator that distinguishes it ("nc6", "nc9") rather than an ordinal, because the
// legs are what tell the two apart on the page; a bare collision counter would make the
// id say nothing a reader could check against the document.
func pinIDFor(name, designator string, taken map[string]bool) string {
	base := strings.Trim(nonIDChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "_"), "_")
	if base == "" {
		base = "pin"
	}
	if designator != "" {
		cand := base + strings.ToLower(nonIDChars.ReplaceAllString(designator, ""))
		if !taken[cand] {
			return cand
		}
		base = cand
	}
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s%d", base, n)
		if !taken[cand] {
			return cand
		}
	}
}

var nonIDChars = regexp.MustCompile(`[^a-z0-9]+`)

// extractPinTable turns one classified pin function table into Pins and, when the
// recipe says the designator columns are packages, Packages.
func extractPinTable(spec *parampb.PartSpec, manifest *derivepb.RunManifest, pg *docpb.Page, t *docpb.Table, axis derivepb.PinColumnAxis) {
	headerRow, cols, ok := findPinColumns(t, axis)
	if !ok {
		manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
			Kind: "unparsed-pin-table", Region: t.Id,
			Detail: "header row lacks a pin-name column, or has no designator/type/description column",
		})
		return
	}
	if len(cols.numbers) > 0 && axis != derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE {
		var raw []string
		for _, nc := range cols.numbers {
			raw = append(raw, nc.raw)
		}
		manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
			Kind: "pin-columns-uninterpreted", Region: t.Id,
			Detail: fmt.Sprintf("designator columns %q not read: recipe declares column_axis %s, so they are not packages",
				strings.Join(raw, " | "), axis),
		})
	}

	declared := map[string]bool{}
	for _, p := range spec.Packages {
		declared[p.Id] = true
	}
	if axis == derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE {
		for _, nc := range cols.numbers {
			for _, code := range nc.packages {
				id := strings.ToLower(code)
				if declared[id] {
					continue
				}
				declared[id] = true
				// Name is the code as printed. The body name ("TSSOP-14") lives in the
				// ordering-information table, not this one, so claiming it here would
				// be inventing a fact this table does not carry.
				spec.Packages = append(spec.Packages, &parampb.Package{Id: id, Name: code, MpnSuffix: code})
			}
		}
	}

	taken := map[string]bool{}
	for _, p := range spec.Pins {
		taken[p.Id] = true
	}
	cellText := func(row int32, col int) string {
		if col < 0 {
			return ""
		}
		for _, c := range t.Cells {
			rs := max(c.RowSpan, 1)
			cs := max(c.ColSpan, 1)
			if int32(col) >= c.Col && int32(col) < c.Col+cs && row >= c.Row && row < c.Row+rs {
				return strings.TrimSpace(c.Text)
			}
		}
		return ""
	}
	// A body row whose name cell SPANS the table is a section band ("LVCMOS CLOCK
	// INPUT"), not a terminal. Real tables group their pins this way.
	sectionRow := func(row int32) bool {
		for _, c := range t.Cells {
			if c.Row == row && int(c.Col) == cols.name && c.ColSpan > 1 {
				return true
			}
		}
		return false
	}

	for row := headerRow + 1; row < t.Rows; row++ {
		name := normalizePinName(cellText(row, cols.name))
		if name == "" || sectionRow(row) {
			continue
		}
		ioRaw := cellText(row, cols.io)
		desc := cellText(row, cols.desc)
		fn := pinFunctionOf(ioRaw, name, desc)

		// Designators per column, and the widest cell in the row.
		perCol := make([][]string, len(cols.numbers))
		widest := 0
		for i, nc := range cols.numbers {
			perCol[i] = splitDesignators(cellText(row, nc.col))
			if len(perCol[i]) > widest {
				widest = len(perCol[i])
			}
		}

		// A row carrying several designators per package is ambiguous by construction:
		// "GND 2, 5, 7" is one terminal bonded to three legs, while "NC 6, 9" is two
		// terminals that happen to share a printed name. The table cannot tell them
		// apart, so the split is keyed on the one function the document states in
		// words, and EVERY such row is gapped whichever way it went.
		split := widest > 1 && fn == parampb.PinFunction_PIN_FUNCTION_NO_CONNECT
		if widest > 1 {
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "multi-designator-row", Region: t.Id,
				Detail: fmt.Sprintf("row %d (%s): %d designators per package; %s", row, name, widest,
					map[bool]string{true: "split into separate pins (no-connect)", false: "kept as one pin on several legs"}[split]),
			})
		}

		emit := 1
		if split {
			emit = widest
		}
		for k := range emit {
			pin := &parampb.Pin{
				Name:        name,
				Function:    fn,
				Description: desc,
				Prov: &parampb.ParamProvenance{
					DocRef: "src", Page: pg.Number, TableOrFigure: t.Title,
					Method: Version, Confidence: Confidence,
				},
			}
			if ioRaw != "" {
				pin.Attributes = map[string]string{"function_raw": ioRaw}
			}
			if axis == derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE {
				for i, nc := range cols.numbers {
					var picked []string
					if split {
						if k < len(perCol[i]) {
							picked = perCol[i][k : k+1]
						}
					} else {
						picked = perCol[i]
					}
					for _, d := range picked {
						for _, code := range nc.packages {
							pin.Numbers = append(pin.Numbers, &parampb.PinNumber{
								PackageRef: strings.ToLower(code), Number: d,
							})
						}
					}
				}
			}
			first := ""
			if len(pin.Numbers) > 0 {
				first = pin.Numbers[0].Number
			}
			id := pinIDFor(name, map[bool]string{true: first, false: ""}[split], taken)
			taken[id] = true
			pin.Id = id
			spec.Pins = append(spec.Pins, pin)
		}
	}
}
