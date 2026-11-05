package formats

import (
	"bufio"
	"os"
	"sort"
	"strings"
	"testing"
)

// clayR6 is Clay R6 from the cadence-exports samples artifact (MIT, agni-samples PROVENANCE.md).
// CLAYR6.brd was saved by Allegro 16.6. pstxnet.dat is the netlist Capture handed Allegro for it.
const clayR6 = "../../tools/samples/cadence/clay-r6/"

// TestAllegro166ReadsAsParasdrsExports holds parasdr's 16.6 board to the ODX that Allegro exported
// from it. TestODXReadsAsItsIPC2581Twin holds that ODX to the IPC-2581 export. Every pin is on the
// same net, every pad sits at the same point on the same side, and the board has the same vias.
//
// A1 is a pinless assembly symbol, so the ODX lists no component for it. A pad Allegro draws as a
// shape symbol reads as custom, which ODX spells as a rounded rectangle (agni issue 1356). A hole
// with no land reads with no shape, which ODX draws as a circle of the drill. So a pad's shape and
// size are compared only where the board names a primitive shape. The board leaves the five
// mechanical pins unnamed, and ODX numbers them as mechPins says.
func TestAllegro166ReadsAsParasdrsExports(t *testing.T) {
	brd, odx := parasdr+"parasdr_layout.brd", parasdr+"parasdr_odb++.xml"
	if _, err := os.Stat(brd); err != nil {
		t.Fatalf("parasdr's board is missing; run `make samples-oracle`: %v", err)
	}
	l := &Loader{}
	bd, err := l.ReadDesign(brd)
	if err != nil {
		t.Fatal(err)
	}
	od, err := l.ReadDesign(odx)
	if err != nil {
		t.Fatal(err)
	}
	if got := bd.GetAttributes()["allegro_format"]; got != "16.6" {
		t.Errorf("allegro_format %q, want 16.6", got)
	}

	bp, op := pinNets(bd), pinNets(od)
	if len(bp) != 432 || len(op) != 432 {
		t.Fatalf("pins on nets: board %d, ODX %d, want 432 each", len(bp), len(op))
	}
	if diffs := diffRows("pin nets", bp, op); len(diffs) > 0 {
		t.Errorf("%d pin differences:\n  %s", len(diffs), capLines(diffs))
	}

	var boardParts []string
	for _, row := range refValues(bd) {
		if !strings.HasPrefix(row, "A1=") {
			boardParts = append(boardParts, row)
		}
	}
	if len(boardParts) != 404 {
		t.Fatalf("board parts other than A1: %d, want 404", len(boardParts))
	}
	odxRefs := map[string]bool{}
	for _, c := range od.GetComponents() {
		odxRefs[c.GetRefDes()] = true
	}
	for _, row := range boardParts {
		if ref, _, _ := strings.Cut(row, "="); !odxRefs[ref] {
			t.Errorf("board part %s is not in the ODX", ref)
		}
	}

	bg, err := l.BoardGeometry(brd)
	if err != nil {
		t.Fatal(err)
	}
	og, err := l.BoardGeometry(odx)
	if err != nil {
		t.Fatal(err)
	}
	unnamed := map[string]bool{}
	for _, odxPin := range mechPins {
		unnamed[odxPin] = true
	}
	// A shape-symbol pad and a hole with no land key on pin, net, drill and side alone.
	shapeless := func(items []placed, board []placed) []placed {
		loose := map[string]bool{}
		for _, p := range board {
			if f := strings.Fields(p.key); f[2] == "custom" || f[2] == "0x0" {
				loose[f[0]] = true
			}
		}
		var out []placed
		for _, p := range items {
			f := strings.Fields(p.key)
			if unnamed[f[0]] {
				f[0] = f[0][:strings.IndexByte(f[0], '.')] + ".?"
				p.key = strings.Join(f, " ")
			}
			if loose[f[0]] {
				p.key = f[0] + " " + f[1] + " " + f[len(f)-2] + " " + f[len(f)-1]
			}
			out = append(out, p)
		}
		return out
	}
	bpads, opads := padItems(bg), padItems(og)
	if len(bpads) != 505 || len(opads) != 505 {
		t.Fatalf("pads: board %d, ODX %d, want 505 each", len(bpads), len(opads))
	}
	if diffs := diffPlaced("pads", shapeless(bpads, bpads), shapeless(opads, bpads)); len(diffs) > 0 {
		t.Errorf("%d pad differences:\n  %s", len(diffs), capLines(diffs))
	}
	primitive := 0
	for _, p := range bpads {
		if f := strings.Fields(p.key); f[2] == "circle" || f[2] == "rect" {
			primitive++
		}
	}
	if primitive != 325 {
		t.Errorf("%d pads compared by shape and size, want 325", primitive)
	}

	vias := 0
	for _, nc := range bg.GetNets() {
		vias += len(nc.GetVias())
	}
	if vias != 420 {
		t.Errorf("board has %d vias, want the ODX's 420", vias)
	}
}

// TestAllegro166ReadsAsClaysNetlist holds Clay R6's 16.6 board to pstxnet.dat. Capture wrote the
// netlist a little after Allegro saved the board. 552 of its 571 pins sit on the same net on the
// board, and each of the other 19 is listed here. Seven are no-connect pins, which Capture gathers
// on a net named NC. Eight are on C2, TP24 and TP25, which the board doesn't place yet, and on the
// two WiFi nets they join. Four are R9's and U5's buzzer pins, which the netlist moved, so the
// board puts those and two more U5 pins on other nets. Any other difference fails.
func TestAllegro166ReadsAsClaysNetlist(t *testing.T) {
	brd := clayR6 + "CLAYR6.brd"
	if _, err := os.Stat(brd); err != nil {
		t.Fatalf("Clay R6's board is missing; run `make samples-oracle`: %v", err)
	}
	want := readPstxnet(t, clayR6+"pstxnet.dat")
	if len(want) != 571 {
		t.Fatalf("pstxnet.dat lists %d pins, want 571", len(want))
	}
	d, err := (&Loader{}).ReadDesign(brd)
	if err != nil {
		t.Fatal(err)
	}
	got := pinNets(d)
	if len(got) != 558 {
		t.Fatalf("the board puts %d pins on nets, want 558", len(got))
	}

	expected := map[string]bool{
		// Not placed on the board yet.
		"kicad only: C2.1=USBPWR": true, "kicad only: C2.2=GND_EARTH": true,
		"kicad only: TP24.1=WIFI_GPIO5": true, "kicad only: TP25.1=WIFI_GPIO4": true,
		"kicad only: U5.93=WIFI_GPIO4": true, "kicad only: U5.94=WIFI_GPIO5": true,
		"kicad only: U7.16=WIFI_GPIO4": true, "kicad only: U7.24=WIFI_GPIO5": true,
		// The buzzer pins the netlist moved.
		"kicad only: R9.1=BUZZER_SWITCH": true, "odb++ only: R9.1=BUZZER_IN": true,
		"kicad only: U5.71=BUZZER_SWITCH": true, "odb++ only: U5.71=PTC1/LLWU_P6/ADC0_SE15/SPI0_PCS": true,
		"kicad only: U5.72=BUZZER_LINE": true, "odb++ only: U5.72=PTC2/ADC0_SE4B/CMP1_IN0/SPI0_PC": true,
		"odb++ only: U5.93=PTD0/LLWU_P12/SPI0_PCS0/UART2_R": true, "odb++ only: U5.94=BUZZER_LINE": true,
		"kicad only: U5.97=PTD4/LLWU_P14/SPI0_PCS1/UART0_R": true, "odb++ only: U5.97=BUZZER_IN": true,
	}
	var unexpected []string
	seen := 0
	for _, line := range diffRows("pins", want, got) {
		row := strings.TrimPrefix(line, "pins: ")
		switch {
		case expected[row]:
			seen++
		case strings.HasPrefix(row, "kicad only: ") && strings.HasSuffix(row, "=NC"):
			seen++
		default:
			unexpected = append(unexpected, line)
		}
	}
	if len(unexpected) > 0 {
		t.Errorf("%d pins differ from pstxnet.dat:\n  %s", len(unexpected), capLines(unexpected))
	}
	if seen != len(expected)+7 {
		t.Errorf("%d of the %d known differences showed, so the list above is stale", seen, len(expected)+7)
	}

	values := map[string]string{}
	for _, c := range d.GetComponents() {
		values[c.GetRefDes()] = c.GetAttributes()["Value"]
	}
	if values["R9"] != "1.0K" || values["U5"] != "MK64FN1M0VLL12" {
		t.Errorf("values R9=%q U5=%q, want 1.0K and MK64FN1M0VLL12", values["R9"], values["U5"])
	}
}

// TestAllegro166DrawsClaysOutline reads Clay R6's board edge. Clay draws it as the design outline of
// a placed mechanical symbol, a 2-inch square around the origin, and not on the board's own lists.
func TestAllegro166DrawsClaysOutline(t *testing.T) {
	g, err := (&Loader{}).BoardGeometry(clayR6 + "CLAYR6.brd")
	if err != nil {
		t.Fatal(err)
	}
	paths := g.GetOutline().GetPaths()
	if len(paths) != 1 || len(paths[0].GetPoints()) != 5 {
		t.Fatalf("outline %v, want one closed rectangle", paths)
	}
	const half = 25400000 // 1000 mil
	for _, p := range paths[0].GetPoints() {
		if (p.GetX() != half && p.GetX() != -half) || (p.GetY() != half && p.GetY() != -half) {
			t.Errorf("outline corner %v, want (+-%d, +-%d)", p, half, half)
		}
	}
	if len(g.GetPlacements()) != 146 {
		t.Errorf("%d placements, want 146", len(g.GetPlacements()))
	}
}

// readPstxnet reads Capture's expanded netlist as `REF.PIN=NET` rows. Each NET_NAME line is
// followed by the quoted net name, and each pin on that net is a `NODE_NAME <ref> <pin>` line.
func readPstxnet(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 -- a fixed path in the samples corpus
	if err != nil {
		t.Fatalf("%v; run `make samples-oracle`", err)
	}
	defer f.Close()
	var out []string
	net, nextIsNet := "", false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case nextIsNet:
			net, nextIsNet = strings.Trim(strings.TrimSpace(line), "'"), false
		case line == "NET_NAME":
			nextIsNet = true
		case strings.HasPrefix(line, "NODE_NAME"):
			if f := strings.Fields(line); len(f) == 3 {
				out = append(out, f[1]+"."+f[2]+"="+net)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}
