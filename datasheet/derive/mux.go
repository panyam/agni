package derive

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/panyam/agni/core/ident"
	"github.com/panyam/agni/core/num"
	derivepb "github.com/panyam/agni/datasheet/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The pin-mux path (agni issue 667). A pin-mux table says which peripheral functions each pin
// can be switched to, and it adds PinAlternates to the spec's pins: to a pin a pin table already
// emitted when one is the same terminal, and to a pin it creates otherwise.
//
// The table has no shared header vocabulary across vendors, so the recipe names every column
// (derive.proto, MuxTableRule). The two layouts are each one vendor's, and the fixtures under
// testdata/ name the document each was built against.

// compiledMuxRule is one recipe mux-table rule with its patterns compiled.
type compiledMuxRule struct {
	re    *regexp.Regexp
	rule  *derivepb.MuxTableRule
	label []*regexp.Regexp // per function column; nil when the whole cell is one function
}

func compileMuxRule(r *derivepb.MuxTableRule) (compiledMuxRule, error) {
	out := compiledMuxRule{rule: r}
	re, err := regexp.Compile(r.TitlePattern)
	if err != nil {
		return out, fmt.Errorf("mux title_pattern %q: %w", r.TitlePattern, err)
	}
	out.re = re
	switch r.Layout {
	case derivepb.MuxLayout_MUX_LAYOUT_CELL_LIST:
		if r.NameColumn == "" {
			return out, fmt.Errorf("mux table %q: a CELL_LIST layout needs name_column", r.TitlePattern)
		}
	case derivepb.MuxLayout_MUX_LAYOUT_ROW_PER_FUNCTION:
		if r.PinStartColumn == "" {
			return out, fmt.Errorf("mux table %q: a ROW_PER_FUNCTION layout needs pin_start_column", r.TitlePattern)
		}
	default:
		return out, fmt.Errorf("mux table %q: layout is unspecified", r.TitlePattern)
	}
	if len(r.FunctionColumns) == 0 {
		return out, fmt.Errorf("mux table %q: no function_columns", r.TitlePattern)
	}
	for _, fc := range r.FunctionColumns {
		var lre *regexp.Regexp
		if fc.LabelPattern != "" {
			if lre, err = regexp.Compile(fc.LabelPattern); err != nil {
				return out, fmt.Errorf("mux table %q: label_pattern %q: %w", r.TitlePattern, fc.LabelPattern, err)
			}
		}
		out.label = append(out.label, lre)
	}
	return out, nil
}

func classifyMux(title string, rules []compiledMuxRule) (compiledMuxRule, bool) {
	for _, r := range rules {
		if r.re.MatchString(title) {
			return r, true
		}
	}
	return compiledMuxRule{}, false
}

// muxState carries the last pin a mux table wrote across tables, because a table broken over a
// page continues its last pin at the top of the next page with no name and no designator.
type muxState struct {
	last *parampb.Pin
}

// muxHeaderKey is a mux header compared the way the recipe names it: lowercased, whitespace
// collapsed, and every trailing footnote marker removed ("SIGNAL NAME (1) (2)").
func muxHeaderKey(s string) string {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	for {
		t := footnoteSuffix.ReplaceAllString(s, "")
		if t == s {
			return s
		}
		s = t
	}
}

// trailingParen is a note after a function name, a footnote marker ("OA0- (6)", "CAN0_TX [1]")
// or a reset-default marker ("P1.2 (RD)"), none of which is part of the name.
var trailingParen = regexp.MustCompile(`\s*(\([^()]*\)|\[[^\[\]]*\])\s*$`)

func cleanFunction(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for {
		t := trailingParen.ReplaceAllString(s, "")
		if t == s {
			break
		}
		s = t
	}
	// A slot naming two signals wraps after its slash ("WUU0_P2/ RF_XTAL_OUT_ENABLE").
	return strings.ReplaceAll(s, "/ ", "/")
}

// muxColumns is a mux table's resolved layout. dataStart is the first row below every declared
// header.
type muxColumns struct {
	name      int
	start     int
	numbers   []muxNumber
	functions []int // parallel to the rule's function_columns; -1 when absent
	dataStart int32
}

type muxNumber struct {
	col     int
	pkg     string
	present bool
}

func findMuxColumns(t *docpb.Table, r *derivepb.MuxTableRule) (muxColumns, []string) {
	cols := muxColumns{name: -1, start: -1}
	at := func(header string) (int, int32) {
		want := muxHeaderKey(header)
		for _, c := range t.Cells {
			if c.Row < 4 && muxHeaderKey(c.Text) == want {
				return int(c.Col), c.Row
			}
		}
		return -1, -1
	}
	var missing []string
	last := int32(-1)
	note := func(header string, col int, row int32) {
		if col < 0 {
			missing = append(missing, header)
			return
		}
		last = max(last, row)
	}
	if r.NameColumn != "" {
		col, row := at(r.NameColumn)
		cols.name = col
		note(r.NameColumn, col, row)
	}
	for _, nc := range r.NumberColumns {
		col, row := at(nc.Header)
		pkg := nc.Package
		if pkg == "" {
			pkg = strings.ToLower(strings.TrimSpace(nc.Header))
		}
		cols.numbers = append(cols.numbers, muxNumber{col: col, pkg: pkg, present: col >= 0})
		note(nc.Header, col, row)
	}
	if r.PinStartColumn != "" {
		col, row := at(r.PinStartColumn)
		cols.start = col
		note(r.PinStartColumn, col, row)
	}
	for _, fc := range r.FunctionColumns {
		col, row := at(fc.Header)
		cols.functions = append(cols.functions, col)
		note(fc.Header, col, row)
	}
	cols.dataStart = last + 1
	return cols, missing
}

// tableGrid answers what text covers a cell position, honouring spans, and which cells START at
// one, which is how a designator is attributed to the pin whose rows it starts in.
type tableGrid struct{ t *docpb.Table }

func (g tableGrid) text(row int32, col int) string {
	if col < 0 {
		return ""
	}
	for _, c := range g.t.Cells {
		rs, cs := max(c.RowSpan, 1), max(c.ColSpan, 1)
		if num.Int32(col) >= c.Col && num.Int32(col) < c.Col+cs && row >= c.Row && row < c.Row+rs {
			return strings.TrimSpace(c.Text)
		}
	}
	return ""
}

func (g tableGrid) startingAt(row int32, col int) string {
	for _, c := range g.t.Cells {
		if c.Row == row && int(c.Col) == col {
			return strings.TrimSpace(c.Text)
		}
	}
	return ""
}

// muxGroup is one pin's rows in a table: its name, its designators per package, and its
// functions. A group with no name and no designator continues the pin above it.
type muxGroup struct {
	name      string
	numbers   []*parampb.PinNumber
	functions []string
	firstRow  int32
}

// extractMuxTable reads one classified pin-mux table into the spec's pins.
func extractMuxTable(spec *parampb.PartSpec, manifest *derivepb.RunManifest, st *muxState, pg *docpb.Page, t *docpb.Table, r compiledMuxRule) {
	cols, missing := findMuxColumns(t, r.rule)
	if len(missing) > 0 {
		manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
			Kind: "unparsed-mux-table", Region: t.Id,
			Detail: fmt.Sprintf("the recipe names columns this table does not have: %s", strings.Join(missing, ", ")),
		})
		return
	}
	g := tableGrid{t}
	var groups []muxGroup
	switch r.rule.Layout {
	case derivepb.MuxLayout_MUX_LAYOUT_CELL_LIST:
		groups = cellListGroups(manifest, t, g, cols, r)
	case derivepb.MuxLayout_MUX_LAYOUT_ROW_PER_FUNCTION:
		groups = rowPerFunctionGroups(t, g, cols)
	}

	declared := map[string]bool{}
	for _, p := range spec.Packages {
		declared[p.Id] = true
	}
	for _, nc := range cols.numbers {
		if !declared[nc.pkg] {
			declared[nc.pkg] = true
			spec.Packages = append(spec.Packages, &parampb.Package{Id: nc.pkg, Name: strings.ToUpper(nc.pkg)})
		}
	}

	for _, grp := range groups {
		if grp.name == "" && len(grp.numbers) == 0 {
			if st.last == nil {
				manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
					Kind: "mux-row-orphan", Region: t.Id,
					Detail: fmt.Sprintf("row %d lists functions %q with no pin above it to continue", grp.firstRow, strings.Join(grp.functions, ", ")),
				})
				continue
			}
			addAlternates(st.last, grp.functions)
			continue
		}
		if grp.name == "" {
			grp.name = firstOr(grp.functions, "")
		}
		if len(grp.functions) == 0 {
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "unyielded-alternates", Region: t.Id,
				Detail: fmt.Sprintf("row %d (%s): the function columns yielded no function", grp.firstRow, grp.name),
			})
		}
		if pkgs := repeatedPackages(grp.numbers); len(pkgs) > 0 {
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "mux-pin-boundary", Region: t.Id,
				Detail: fmt.Sprintf("row %d (%s) collects several designators in %s, so the table may run two pins together here",
					grp.firstRow, grp.name, strings.Join(pkgs, ", ")),
			})
		}
		pin := muxTerminal(spec, grp)
		if pin == nil {
			taken := map[string]bool{}
			for _, p := range spec.Pins {
				taken[p.Id] = true
			}
			pin = &parampb.Pin{
				Id:   pinIDFor(grp.name, "", taken),
				Name: grp.name,
				Prov: &parampb.ParamProvenance{
					DocRef: "src", Page: pg.Number, TableOrFigure: t.Title,
					Method: Version, Confidence: Confidence,
				},
			}
			spec.Pins = append(spec.Pins, pin)
		}
		mergeNumbers(pin, &parampb.Pin{Numbers: unclaimed(spec, pin, grp.numbers, manifest, t.Id)})
		addAlternates(pin, grp.functions)
		st.last = pin
	}
}

// cellListGroups reads the one-row-per-pin layout. Each function cell splits at the column's
// label, and text a label does not introduce is gapped rather than guessed at.
func cellListGroups(manifest *derivepb.RunManifest, t *docpb.Table, g tableGrid, cols muxColumns, r compiledMuxRule) []muxGroup {
	var out []muxGroup
	for row := cols.dataStart; row < t.Rows; row++ {
		grp := muxGroup{firstRow: row, name: strings.TrimSpace(g.text(row, cols.name))}
		for _, nc := range cols.numbers {
			for _, d := range splitDesignators(g.text(row, nc.col)) {
				grp.numbers = append(grp.numbers, &parampb.PinNumber{PackageRef: nc.pkg, Number: d})
			}
		}
		for i, col := range cols.functions {
			fns, residue := splitLabelled(g.text(row, col), r.label[i])
			grp.functions = append(grp.functions, fns...)
			if residue != "" {
				manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
					Kind: "mux-cell-unparsed", Region: t.Id,
					Detail: fmt.Sprintf("row %d (%s), column %q: %q is not introduced by a label, so it was not read as a function",
						row, grp.name, r.rule.FunctionColumns[i].Header, residue),
				})
			}
		}
		if grp.name == "" && len(grp.numbers) == 0 && len(grp.functions) == 0 {
			continue
		}
		out = append(out, grp)
	}
	return out
}

// splitLabelled splits a CELL_LIST cell into its functions. A nil label reads the whole cell as
// one function. The residue is text before the first label, or a whole cell no label matched.
func splitLabelled(cell string, label *regexp.Regexp) (fns []string, residue string) {
	if absentNumber(cell) {
		return nil, ""
	}
	if label == nil {
		return []string{cleanFunction(cell)}, ""
	}
	locs := label.FindAllStringIndex(cell, -1)
	if len(locs) == 0 {
		return nil, strings.TrimSpace(cell)
	}
	residue = strings.TrimSpace(cell[:locs[0][0]])
	for i, loc := range locs {
		end := len(cell)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		v := strings.TrimSpace(cell[loc[1]:end])
		v = strings.TrimSpace(strings.TrimPrefix(v, "-"))
		if v = cleanFunction(v); v != "" && !absentNumber(v) {
			fns = append(fns, v)
		}
	}
	return fns, residue
}

// rowPerFunctionGroups reads the one-row-per-function layout. A pin begins on each row whose
// pin_start_column cell is filled, and owns the rows down to the next such row. Its designators
// are the number cells that START in its rows, whatever they span (see
// MuxTableRule.pin_start_column for why spans are not trusted). Rows above the first start
// continue the previous table's last pin.
func rowPerFunctionGroups(t *docpb.Table, g tableGrid, cols muxColumns) []muxGroup {
	var out []muxGroup
	for row := cols.dataStart; row < t.Rows; row++ {
		if startsPin(g, row, cols.start) || len(out) == 0 {
			out = append(out, muxGroup{firstRow: row})
		}
		grp := &out[len(out)-1]
		for _, nc := range cols.numbers {
			for _, d := range splitDesignators(g.startingAt(row, nc.col)) {
				grp.numbers = append(grp.numbers, &parampb.PinNumber{PackageRef: nc.pkg, Number: d})
			}
		}
		for _, col := range cols.functions {
			if s := g.startingAt(row, col); s != "" && !absentNumber(s) {
				grp.functions = append(grp.functions, cleanFunction(s))
			}
		}
	}
	// A group above the first start row holds a continued pin, which has no name of its own.
	for i := range out {
		if i > 0 || startsPin(g, out[0].firstRow, cols.start) {
			out[i].name = firstOr(out[i].functions, "")
		}
	}
	return out
}

// startsPin reports whether a row's start-column cell is filled. Only dashes mean "not this
// pin's first row" ("- -" is two of them a producer merged); "N/A" is a value, and TI prints it
// as the reset state of a supply pin.
func startsPin(g tableGrid, row int32, col int) bool {
	return strings.Trim(g.startingAt(row, col), "-—– ") != ""
}

func firstOr(s []string, def string) string {
	if len(s) > 0 {
		return s[0]
	}
	return def
}

// muxTerminal finds the pin already in the spec that a mux row describes: one sharing a
// (package, designator) pair, or, when the row has no designators, one whose name matches under
// ident.PinName. Nil means the row is a pin the spec does not have yet.
func muxTerminal(spec *parampb.PartSpec, grp muxGroup) *parampb.Pin {
	for _, p := range spec.Pins {
		for _, a := range p.Numbers {
			for _, b := range grp.numbers {
				if a.PackageRef == b.PackageRef && a.Number == b.Number {
					return p
				}
			}
		}
	}
	if len(grp.numbers) > 0 {
		return nil
	}
	key := ident.PinName(grp.name)
	for _, p := range spec.Pins {
		if ident.PinName(p.Name) == key {
			return p
		}
	}
	return nil
}

// addAlternates appends functions to a pin in order, skipping one it already lists, since a pin
// restated on a continuation page lists its functions again.
func addAlternates(p *parampb.Pin, fns []string) {
	have := map[string]bool{}
	for _, a := range p.Alternates {
		have[a.Name] = true
	}
	for _, f := range fns {
		if !have[f] {
			have[f] = true
			p.Alternates = append(p.Alternates, &parampb.PinAlternate{Name: f})
		}
	}
}

// repeatedPackages lists the packages in which a group holds more than one designator. A pin on
// several legs is real (a ground), but in a pin-mux table it more often means two pins read as one.
func repeatedPackages(nums []*parampb.PinNumber) []string {
	n := map[string]int{}
	var out []string
	for _, d := range nums {
		if n[d.PackageRef]++; n[d.PackageRef] == 2 {
			out = append(out, d.PackageRef)
		}
	}
	return out
}

// unclaimed drops the designators another pin already holds, gapping each, since a designator
// belongs to one pin in a package and a mux row that collides with a pin table has misread one
// of the two.
func unclaimed(spec *parampb.PartSpec, pin *parampb.Pin, nums []*parampb.PinNumber, manifest *derivepb.RunManifest, region string) []*parampb.PinNumber {
	var out []*parampb.PinNumber
	for _, d := range nums {
		owner := ""
		for _, p := range spec.Pins {
			if p == pin {
				continue
			}
			for _, n := range p.Numbers {
				if n.PackageRef == d.PackageRef && n.Number == d.Number {
					owner = p.Name
				}
			}
		}
		if owner != "" {
			manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
				Kind: "mux-number-conflict", Region: region,
				Detail: fmt.Sprintf("%s: designator %s in %s is already %s's, so it was not added", pin.Name, d.Number, d.PackageRef, owner),
			})
			continue
		}
		out = append(out, d)
	}
	return out
}
