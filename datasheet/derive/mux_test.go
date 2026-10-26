package derive

import (
	"os"
	"strings"
	"testing"

	derivepb "github.com/panyam/agni/datasheet/gen/go/agni/v1/derive"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func runMuxFixture(t *testing.T, fixture, mpn string) (*parampb.PartSpec, *derivepb.RunManifest) {
	t.Helper()
	recipes, _ := loadArtifacts(t)
	spec, manifest, err := Run(loadDocFixture(t, fixture), recipes, nil, Identity{MPN: mpn})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return spec, manifest
}

func pinNamed(spec *parampb.PartSpec, name string) *parampb.Pin {
	for _, p := range spec.Pins {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func alternatesOf(p *parampb.Pin) string {
	var out []string
	for _, a := range p.GetAlternates() {
		out = append(out, a.Name)
	}
	return strings.Join(out, " | ")
}

func numbersString(p *parampb.Pin) string {
	var out []string
	for _, n := range p.GetNumbers() {
		out = append(out, n.PackageRef+"="+n.Number)
	}
	return strings.Join(out, " ")
}

// The one-row-per-pin layout, read against the page: every slot of a cell, the supply-domain
// column, a pin continued across a page break, and a function whose name contains a label.
func TestCellListReadsEveryMuxSlot(t *testing.T) {
	spec, manifest := runMuxFixture(t, "kw47-pinmux-docir.textproto", "KW47")
	for _, tc := range []struct{ pin, numbers, alternates string }{
		{"PTB3", "48qfn=1", "ADC0_B13 | PTB3 | LPSPI1_SOUT | LPUART1_RX | TPM1_CH3 | FLEXIO0_D29 | WUU0_P14"},
		{"PTB4", "48qfn=2", "PTB4 | LPSPI1_PCS3 | LPUART1_CTS_b | LPI2C1_SDA | I3C0_SDA | TRGMUX0_IN0 | FLEXIO0_D30 | WUU0_P15"},
		{"PTA1", "48qfn=9", "PTA1 | CMP1_OUT | LPUART0_RTS_b | RF_GPO_10 | TPM0_CH5 | FLEXIO0_D1 | SWD_CLK"},
		{"PTA4", "48qfn=10", "ADC0_A10/CMP0_IN0 | PTA4 | RF_GPO_9 | TPM0_CLKIN | TRACE_SWO | FLEXIO0_D4 | BOOT_CONFIG | WUU0_P2/RF_XTAL_OUT_ENABLE"},
		{"VDD_SWITCH", "48qfn=6", "VDD_SWITCH"},
	} {
		p := pinNamed(spec, tc.pin)
		if p == nil {
			t.Errorf("no pin %s", tc.pin)
			continue
		}
		if got := numbersString(p); got != tc.numbers {
			t.Errorf("%s numbers: got %q, want %q", tc.pin, got, tc.numbers)
		}
		if got := alternatesOf(p); got != tc.alternates {
			t.Errorf("%s alternates:\n got %s\nwant %s", tc.pin, got, tc.alternates)
		}
		if p.Prov.GetPage() == 0 || !strings.Contains(p.Prov.GetTableOrFigure(), "Package pinout") {
			t.Errorf("%s: provenance does not cite the table: %v", tc.pin, p.Prov)
		}
	}
	if n := len(spec.Pins); n != 12 {
		t.Errorf("%d pins, want the 12 rows of pages 58 to 60", n)
	}
	if len(gapsOfKind(manifest, "mux-row-orphan")) != 0 {
		t.Errorf("a continued pin was gapped as an orphan: %v", gapsOfKind(manifest, "mux-row-orphan"))
	}
}

// A function named after its own slot label ("LPTMR0_ALT3") is one function. The fixture's page
// 60 does not carry PTD2, so the cell is read through splitLabelled directly with the recipe's
// pattern.
func TestALabelInsideAFunctionNameIsNotASlot(t *testing.T) {
	recipes, _ := loadArtifacts(t)
	var r compiledMuxRule
	for _, rc := range recipes {
		if rc.Name == "nxp-kw47" {
			var err error
			if r, err = compileMuxRule(rc.MuxTables[0]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if r.rule == nil {
		t.Fatal("no nxp-kw47 recipe")
	}
	fns, residue := splitLabelled("ALT0 - ADC0_A6 ALT1 - PTD2 ALT2 - LPTMR0_ALT3 ALT3 - TAMPER0", r.label[0])
	if got := strings.Join(fns, " | "); got != "ADC0_A6 | PTD2 | LPTMR0_ALT3 | TAMPER0" || residue != "" {
		t.Errorf("got %q (residue %q)", got, residue)
	}
}

// A header the producer folded the previous pin's slots into cannot be found, and the table is
// gapped by name rather than read with a guessed column.
func TestAFoldedMuxHeaderGapsTheTable(t *testing.T) {
	spec, manifest := runMuxFixture(t, "kw47-pinmux-docir.textproto", "KW47")
	gaps := gapsOfKind(manifest, "unparsed-mux-table")
	if len(gaps) != 1 || gaps[0].Region != "p65.t1" || !strings.Contains(gaps[0].Detail, "Pinmux Assignment") {
		t.Fatalf("want one unparsed-mux-table gap on p65.t1 naming the column, got %v", gaps)
	}
	if pinNamed(spec, "PTC3") != nil {
		t.Error("a pin from the gapped table was emitted")
	}
}

// The one-row-per-function layout, read against the page: a pin is the rows from one filled
// reset-state cell to the next, and its numbers are the cells that start in those rows, wherever
// the producer put them.
func TestRowPerFunctionGroupsByTheStartColumn(t *testing.T) {
	spec, _ := runMuxFixture(t, "msp430fr2355-pinattr-docir.textproto", "MSP430FR2355")
	for _, tc := range []struct{ pin, numbers, alternates string }{
		// DBT 5 sits on the pin's fourth row, unspanned.
		{"P1.2", "pt=1 rha=40 rsm=32 dbt=5", "P1.2 | UCB0SIMO | UCB0SDA | TB0TRG | OA0- | A2 | Veref-"},
		{"RST", "pt=5 rha=4 dbt=9 rsm=4", "RST | NMI | SBWTDIO"},
		// "N/A" is a reset state, so each supply pin is a pin of its own.
		{"DVCC", "pt=6 rha=5 dbt=10 rsm=5", "DVCC"},
		{"DVSS", "pt=7 rha=6 dbt=11 rsm=6", "DVSS"},
		// RHA 24 spans eleven rows, and P1.7's numbers sit on its fifth row.
		{"P2.0", "pt=30 rha=24 dbt=27 rsm=18", "P2.0 | TB1.1 | COMP0.O"},
		{"P1.7", "pt=31 rha=25 dbt=28 rsm=19", "P1.7 | UCA0TXD | UCA0SIMO | TB0.2 | TDO | OA1+ | A7 | VREF+"},
		// "- -" does not start a pin, so TCLK stays on P1.6.
		{"P1.6", "rha=26 dbt=29 rsm=20 pt=32", "P1.6 | UCA0RXD | UCA0SOMI | TB0.1 | TDI | TCLK | OA1- | A6"},
	} {
		p := pinNamed(spec, tc.pin)
		if p == nil {
			t.Errorf("no pin %s", tc.pin)
			continue
		}
		if got := numbersString(p); got != tc.numbers {
			t.Errorf("%s numbers: got %q, want %q", tc.pin, got, tc.numbers)
		}
		if got := alternatesOf(p); got != tc.alternates {
			t.Errorf("%s alternates:\n got %s\nwant %s", tc.pin, got, tc.alternates)
		}
	}
	if pinNamed(spec, "TCLK") != nil {
		t.Error("TCLK was read as a pin of its own")
	}
	if len(spec.Packages) != 4 {
		t.Errorf("%d packages, want PT, RHA, DBT and RSM", len(spec.Packages))
	}
}

// Two pins' signals in one cell collect two designators in one package, which is gapped.
func TestTwoPinsInOneGroupAreGapped(t *testing.T) {
	_, manifest := runMuxFixture(t, "msp430fr2355-pinattr-docir.textproto", "MSP430FR2355")
	gaps := gapsOfKind(manifest, "mux-pin-boundary")
	if len(gaps) != 1 || gaps[0].Region != "p20.t1" || !strings.Contains(gaps[0].Detail, "pt") {
		t.Errorf("want one mux-pin-boundary gap on p20.t1, got %v", gaps)
	}
}

// A row whose function cells yield nothing is gapped, so a pin with no alternates reads as
// unread rather than as a pin with no functions.
func TestAPinWithNoFunctionsIsGapped(t *testing.T) {
	d := loadDocFixture(t, "kw47-pinmux-docir.textproto")
	for _, c := range d.Pages[0].Tables[0].Cells {
		if c.Row == 1 && c.Col == 2 {
			c.Text = "-"
		}
		if c.Row == 1 && c.Col == 4 {
			c.Text = "-"
		}
	}
	recipes, _ := loadArtifacts(t)
	spec, manifest, err := Run(d, recipes, nil, Identity{MPN: "KW47"})
	if err != nil {
		t.Fatal(err)
	}
	gaps := gapsOfKind(manifest, "unyielded-alternates")
	if len(gaps) != 1 || !strings.Contains(gaps[0].Detail, "PTB3") {
		t.Errorf("want one unyielded-alternates gap naming PTB3, got %v", gaps)
	}
	if p := pinNamed(spec, "PTB3"); p == nil || len(p.Alternates) != 0 {
		t.Errorf("PTB3 should be emitted with no alternates: %v", p)
	}
}

// A mux row joins the pin a pin-function table already emitted for the same terminal, rather
// than minting a second pin with the same designator.
func TestAMuxRowJoinsThePinTablesPin(t *testing.T) {
	spec := &parampb.PartSpec{
		Packages: []*parampb.Package{{Id: "48qfn"}},
		Pins: []*parampb.Pin{{Id: "ptb3", Name: "PTB3", Function: parampb.PinFunction_PIN_FUNCTION_BIDIRECTIONAL,
			Numbers: []*parampb.PinNumber{{PackageRef: "48qfn", Number: "1"}}}},
	}
	got := muxTerminal(spec, muxGroup{name: "PTB3", numbers: []*parampb.PinNumber{{PackageRef: "48qfn", Number: "1"}}})
	if got != spec.Pins[0] {
		t.Fatalf("the mux row did not join the existing pin")
	}
}

func TestMuxRecipesAreValidated(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"no layout", `name: "x" mux_tables { title_pattern: "t" function_columns { header: "f" } }`, "layout"},
		{"list without name", `name: "x" mux_tables { title_pattern: "t" layout: MUX_LAYOUT_CELL_LIST function_columns { header: "f" } }`, "name_column"},
		{"rows without start", `name: "x" mux_tables { title_pattern: "t" layout: MUX_LAYOUT_ROW_PER_FUNCTION function_columns { header: "f" } }`, "pin_start_column"},
		{"no functions", `name: "x" mux_tables { title_pattern: "t" layout: MUX_LAYOUT_CELL_LIST name_column: "n" }`, "function_columns"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(dir+"/r.textproto", []byte(tc.body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadRecipes(os.DirFS(dir))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error naming %s", tc.name, err, tc.want)
		}
	}
}

// A function name drops the notes a vendor hangs off it and keeps a slot naming two signals whole.
func TestCleanFunctionDropsNotesAndRejoinsAWrappedSlot(t *testing.T) {
	for in, want := range map[string]string{
		"CAN0_TX [1]":                 "CAN0_TX",
		"OA0- (6)":                    "OA0-",
		"P1.2 (RD)":                   "P1.2",
		"WUU0_P2/ RF_XTAL_OUT_ENABLE": "WUU0_P2/RF_XTAL_OUT_ENABLE",
		"ADC0_A10/CMP0_IN0":           "ADC0_A10/CMP0_IN0",
	} {
		if got := cleanFunction(in); got != want {
			t.Errorf("cleanFunction(%q) = %q, want %q", in, got, want)
		}
	}
}
