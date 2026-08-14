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
	// labelled marks a column found by its own header word ("NO.", "PIN") rather than
	// by sitting inside a band. Such a column is THE numbering of a single-package
	// table and names no package, so it must never be read as one: doing so mints a
	// package called "pin" and then collides every designator inside it.
	labelled bool
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
			cols.numbers = append(cols.numbers, numberColumn{col: int(c.Col), raw: strings.TrimSpace(c.Text), labelled: true})
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

	// A TWO-ROW HEADER puts the outer columns a row ABOVE the one naming the pin, and
	// scanning only the name's row loses them. The common shape:
	//
	//	row 0:  PIN (spanning)          TYPE    DESCRIPTION
	//	row 1:  NO.        NAME
	//	row 2:  1          TXD          DI      Transmit data input
	//
	// The header row is located by finding the pin NAME, which is on the inner row, so
	// TYPE and DESCRIPTION were invisible and every pin came out untyped with no prose.
	// That is not a rare layout: it is what a table gets as soon as it bands its
	// designator columns, and it silently cost the type column on documents that HAVE one.
	//
	// Only columns still unset are filled, so the name's own row always wins, and the
	// scan is bounded by headerRow, which the search above caps at the table's first few
	// rows. A stray cell cannot masquerade as a header here because these vocabularies
	// match whole keys, not substrings.
	for _, c := range t.Cells {
		if c.Row >= headerRow {
			continue
		}
		switch k := headerKey(c.Text); {
		case pinIOHeaders[k] && cols.io < 0:
			cols.io = int(c.Col)
		case pinDescHeaders[k] && cols.desc < 0:
			cols.desc = int(c.Col)
		}
	}
	// Package codes come off the header cell, and only when the recipe says these
	// columns ARE packages. Under any other axis the codes are recorded raw and no
	// Package is minted, which is what keeps a variant column from becoming a body.
	if axis == derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE {
		for i := range cols.numbers {
			if cols.numbers[i].labelled {
				continue
			}
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
	// The footnote marker a vendor hangs off a header cell ("LCCC (1)", "SOIC, ...,
	// CFP (1)") belongs to the cell, not to the last code in it. Stripping it here
	// rather than per-code is what keeps the final package in a list from being
	// silently dropped.
	s = footnoteSuffix.ReplaceAllString(strings.TrimSpace(s), "")
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(footnoteSuffix.ReplaceAllString(strings.TrimSpace(part), ""))
		if p == "" || !isPackageCode(p) {
			continue
		}
		out = append(out, strings.ToUpper(p))
	}
	return out
}

var footnoteSuffix = regexp.MustCompile(`\s*\(\d+\)\s*$`)

var packageCodeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,11}$`)

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
// The fallback reads the DESCRIPTION, never the name, and that line is the whole design.
// Inferring POWER_INPUT from a name like "VCCA" would be this stage inventing a
// classification the document did not make; reading it from a description that opens
// "A-port supply voltage" is repeating one the document DID make, in the column real
// tables put it in when they leave the type column dashed. A name is our guess, a
// description is the vendor's sentence.
func pinFunctionOf(ioRaw, name, desc string) parampb.PinFunction {
	switch headerKey(ioRaw) {
	// The abbreviated spellings (di / do / bi/o) are as standard as the long ones and turn
	// up across the corpus; they were simply absent. Vendor-specific tokens are NOT added
	// here on sight: a token this stage does not know now says so in the untyped-pin gap,
	// naming itself, so widening this list stays evidence-driven rather than speculative.
	case "i/o", "io", "d_io", "a_io", "bi/o", "bio", "bidirectional", "input/output":
		return parampb.PinFunction_PIN_FUNCTION_BIDIRECTIONAL
	case "i", "in", "input", "a_in", "d_in", "di", "digital input", "analog input":
		return parampb.PinFunction_PIN_FUNCTION_INPUT
	case "o", "out", "output", "d_out", "a_out", "do", "digital output":
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
	return pinFunctionFromDescription(desc)
}

// pinFunctionFromDescription types a supply or ground pin from the vendor's own sentence,
// for the common table that leaves its type column dashed on exactly those rows.
//
// DELIBERATELY DUMB, and it should stay that way. It matches how a description OPENS, in
// the handful of phrasings that are near-universal, and answers UNSPECIFIED for everything
// else. The temptation is to search the whole sentence for "ground" or "supply", and that
// is the failure: "Connect to ground through a 10 kΩ resistor" describes a signal with a
// pulldown, and typing that pin GROUND would stamp a false ground onto a net every rail
// rule then quantifies over. An untyped pin is the status quo and costs nothing; a wrongly
// typed one is a confident wrong answer.
//
// What this deliberately does NOT try to do is be complete. Everything it declines lands in
// the run manifest as an untyped-pin gap, which is the curation worklist: a human, or an
// assistant searching the document on their behalf, sees the pin and its prose and decides.
// Widening this matcher to swallow the long tail would trade a visible gap for an invisible
// guess, which is the wrong direction.
//
// The lexicon is local rather than shared with classify's rail naming, and that is a
// deviation from what OUT_OF_SCOPE suggested. Those vocabularies turn out to be different
// languages: classify matches net NAMES (VCC, +3V3, start-anchored tokens), this matches
// English prose. Sharing would mean one of them carrying patterns that can never fire.
func pinFunctionFromDescription(desc string) parampb.PinFunction {
	d := headerKey(desc)
	switch {
	case d == "ground" || strings.HasPrefix(d, "ground reference") ||
		strings.HasPrefix(d, "ground pin") || strings.HasPrefix(d, "device ground"):
		return parampb.PinFunction_PIN_FUNCTION_GROUND
	case strings.HasPrefix(d, "supply voltage") || strings.HasPrefix(d, "power supply") ||
		strings.HasPrefix(d, "supply input") || strings.Contains(firstClause(d), "supply voltage"):
		return parampb.PinFunction_PIN_FUNCTION_POWER_INPUT
	}
	return parampb.PinFunction_PIN_FUNCTION_UNSPECIFIED
}

// firstClause is the description up to its first clause break, so a qualified opening still
// reads as an opening. "A-port supply voltage 1.2V <= VCCA <= 3.6V" is the vendor typing the
// pin and then bounding it; "Enable input. Tie to supply voltage for normal operation" is a
// signal that merely mentions one, and its first clause says so.
//
// A sentence break is a period FOLLOWED BY A SPACE, not any period. Pin descriptions are
// full of decimals ("1.2V"), and splitting on those would cut "A-port supply voltage 1.2V"
// down to "a-port supply voltage 1" — which happens to still match here, and would stop
// matching the moment a vendor led with the number.
func firstClause(d string) string {
	cut := len(d)
	for _, sep := range []string{". ", ",", ";"} {
		if i := strings.Index(d, sep); i > 0 && i < cut {
			cut = i
		}
	}
	return d[:cut]
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

	if axis == derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE {
		for _, nc := range cols.numbers {
			if nc.labelled {
				manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
					Kind: "pin-numbering-unattributed", Region: t.Id,
					Detail: fmt.Sprintf("column %q is this table's single numbering and names no package, so its designators are not attributed; declare the package in the recipe if this document ships in one body", nc.raw),
				})
				continue
			}
			if len(nc.packages) == 0 {
				manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
					Kind: "package-header-unparsed", Region: t.Id,
					Detail: fmt.Sprintf("header cell %q yielded no package code, so its designators are dropped", nc.raw),
				})
			}
		}
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

		// A pin nobody could type is the curation worklist's whole reason for existing. The
		// classifier is deliberately narrow, so declining is the ORDINARY outcome on a table
		// whose type column is dashed and whose prose is not one of the standard openings, and
		// an undeclared decline is indistinguishable from a pin that carries no function at
		// all. Gapping it carries the pin's own sentence, which is what a human (or an
		// assistant searching the document for them) needs in order to decide.
		if fn == parampb.PinFunction_PIN_FUNCTION_UNSPECIFIED {
			// Say WHICH channel came up empty, because the two want opposite fixes. A row whose
			// type column holds a token this stage does not know ("DI") is a vocabulary gap and
			// the token is the whole answer; a row with no type column at all is a prose
			// judgement someone has to make. Reporting "no type column" for both, as this first
			// did, sends a reader looking for a missing column that is right there.
			why := fmt.Sprintf("type column reads %q, which is not in the vocabulary", ioRaw)
			if strings.TrimSpace(ioRaw) == "" {
				why = "no type column and no standard description opening"
			}
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "untyped-pin", Region: t.Id,
				Detail: fmt.Sprintf("row %d (%s): %s; description: %q", row, name, why, desc),
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
			// A pin table spanning several pages arrives as several tables that restate
			// rows, so the same terminal is met twice. Two pins are the SAME terminal
			// when they share a name AND a designator; that second half is what keeps
			// the no-connect split intact, since nc6 and nc9 share a name and no leg.
			if prev := sameTerminal(spec, pin); prev != nil {
				mergeNumbers(prev, pin)
				continue
			}
			id := pinIDFor(name, map[bool]string{true: first, false: ""}[split], taken)
			taken[id] = true
			pin.Id = id
			spec.Pins = append(spec.Pins, pin)
		}
	}
}

// sameTerminal finds an already-emitted pin that is the same terminal as the candidate:
// same name, and at least one identical (package, designator) pair. Name alone is not
// enough, because a part may print one name on several terminals, which is exactly the
// case the no-connect split exists for.
func sameTerminal(spec *parampb.PartSpec, cand *parampb.Pin) *parampb.Pin {
	if len(cand.Numbers) == 0 {
		return nil
	}
	for _, p := range spec.Pins {
		if p.Name != cand.Name {
			continue
		}
		for _, a := range p.Numbers {
			for _, b := range cand.Numbers {
				if a.PackageRef == b.PackageRef && a.Number == b.Number {
					return p
				}
			}
		}
	}
	return nil
}

// mergeNumbers folds a restated row's designators into the terminal already emitted,
// keeping the union. A continuation table may carry a body the first statement omitted.
func mergeNumbers(dst, src *parampb.Pin) {
	have := map[string]bool{}
	for _, n := range dst.Numbers {
		have[n.PackageRef+"\x00"+n.Number] = true
	}
	for _, n := range src.Numbers {
		if k := n.PackageRef + "\x00" + n.Number; !have[k] {
			dst.Numbers = append(dst.Numbers, n)
			have[k] = true
		}
	}
}
