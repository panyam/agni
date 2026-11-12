// Command kicadcut cuts a hierarchical KiCad project down to a few parts, for a fixture that keeps
// a real board's drawing and copper without its size (agni issue 1323).
//
//	go run ./tools/kicadcut -src <project dir> -root <root>.kicad_sch -out <dir> \
//	    -sheet usb_debug_pd=105,55,170,115 -sheet power=240,28,290,110
//
// Each -sheet names a sub-sheet file (without .kicad_sch) and a box in that sheet's millimetres.
// The cut keeps what the box holds: symbols, labels, junctions and no-connects whose position is in
// it, and wires wholly inside it, plus the library symbols the kept symbols use. The root keeps
// only the sheet blocks for the cut sheets, so every kept symbol's instance path still resolves.
// The board keeps the footprints of the kept parts, the tracks and vias on their nets inside a
// margin around them, and the board outline. It drops zones, because a pour reaches across the
// whole board.
//
// A cut ends some of the design's nets, and the cut schematic then names them for itself
// (Net-(Q2-D)) where the board still carries the whole design's name (VCC_IN). With -netlist, the
// board's nets take the names that kicad-cli's netlist of the cut gives their pads, as KiCad's
// Update PCB from Schematic would. A net keeps its number, so its tracks and vias follow it.
//
// It works on the text, cutting top-level lists out by their spans, so what it keeps is
// byte-for-byte what KiCad wrote. The one edit is inside a footprint, where it drops graphics on
// the User layers. One vendor library draws hundreds of artwork pieces there per part, which agni
// does not draw (agni 946) and which made a 20-part board 1.9 MB.
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type box struct{ x0, y0, x1, y1 float64 }

func (b box) has(x, y float64) bool { return x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1 }

type sheets map[string]box

func (s sheets) String() string { return fmt.Sprint(map[string]box(s)) }

func (s sheets) Set(v string) error {
	name, rest, ok := strings.Cut(v, "=")
	f := strings.Split(rest, ",")
	if !ok || len(f) != 4 {
		return fmt.Errorf("want name=x0,y0,x1,y1, got %q", v)
	}
	var n [4]float64
	for i, p := range f {
		x, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return err
		}
		n[i] = x
	}
	s[name] = box{n[0], n[1], n[2], n[3]}
	return nil
}

func main() {
	src := flag.String("src", "", "the KiCad project directory")
	root := flag.String("root", "", "the root schematic's file name")
	out := flag.String("out", "", "the directory to write the cut into")
	stem := flag.String("stem", "", "the cut project's file stem (default: the root's)")
	margin := flag.Float64("margin", 4, "millimetres of copper kept around the kept footprints")
	netlist := flag.String("netlist", "", "kicad-cli's kicadxml netlist of the cut, to rename the board's nets by")
	cut := sheets{}
	flag.Var(cut, "sheet", "name=x0,y0,x1,y1, repeatable")
	flag.Parse()
	if *src == "" || *root == "" || *out == "" || len(cut) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *stem == "" {
		*stem = strings.TrimSuffix(*root, ".kicad_sch")
	}
	if err := run(*src, *root, *out, *stem, *netlist, *margin, cut); err != nil {
		fmt.Fprintln(os.Stderr, "kicadcut:", err)
		os.Exit(1)
	}
}

func run(src, root, out, stem, netlist string, margin float64, cut sheets) error {
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	refs := map[string]bool{}
	names := make([]string, 0, len(cut))
	for n := range cut {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		text, kept, err := cutSheet(filepath.Join(src, n+".kicad_sch"), cut[n])
		if err != nil {
			return err
		}
		for _, r := range kept {
			refs[r] = true
		}
		if err := os.WriteFile(filepath.Join(out, n+".kicad_sch"), []byte(text), 0o600); err != nil {
			return err
		}
		fmt.Printf("%s: %d parts %v\n", n, len(kept), kept)
	}
	rootText, err := cutRoot(filepath.Join(src, root), names)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, stem+".kicad_sch"), []byte(rootText), 0o600); err != nil {
		return err
	}
	pcb := filepath.Join(src, strings.TrimSuffix(root, ".kicad_sch")+".kicad_pcb")
	board, stats, err := cutBoard(pcb, refs, margin)
	if err != nil {
		return err
	}
	fmt.Println("board:", stats)
	if netlist != "" {
		renamed, n, err := renameNets(board, netlist)
		if err != nil {
			return err
		}
		board = renamed
		fmt.Printf("board: %d nets renamed to the cut schematic's names\n", n)
	}
	if err := os.WriteFile(filepath.Join(out, stem+".kicad_pcb"), []byte(board), 0o600); err != nil {
		return err
	}
	// The project file carries the net classes and the board's design rules.
	pro, err := os.ReadFile(filepath.Join(src, strings.TrimSuffix(root, ".kicad_sch")+".kicad_pro")) // #nosec G304 -- a developer tool reading the project its own -src flag names
	if err != nil {
		return nil // a project without a .kicad_pro still cuts, with KiCad's default rules
	}
	return os.WriteFile(filepath.Join(out, stem+".kicad_pro"), pro, 0o600) // #nosec G703 -- writes into the directory its own -out flag names
}

// span is one top-level list of a file, by its byte offsets.
type span struct{ a, b int }

// topItems lists the direct children of the file's root list. It skips quoted strings, so a
// paren inside a property value does not count.
func topItems(s string) []span {
	var out []span
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
			if depth == 2 {
				start = i
			}
		case ')':
			if depth == 2 {
				out = append(out, span{start, i + 1})
			}
			depth--
		}
	}
	return out
}

var (
	headRe  = regexp.MustCompile(`^\(\s*([^\s()]+)`)
	atRe    = regexp.MustCompile(`\(at ([-\d.]+) ([-\d.]+)`)
	xyRe    = regexp.MustCompile(`\(xy ([-\d.]+) ([-\d.]+)\)`)
	libIDRe = regexp.MustCompile(`\(lib_id "([^"]+)"\)`)
	refRe   = regexp.MustCompile(`\(property "Reference" "([^"]+)"`)
	symRe   = regexp.MustCompile(`^\(symbol "([^"]+)"`)
	fileRe  = regexp.MustCompile(`\(property "Sheetfile" "([^"]+)"`)
	netRe   = regexp.MustCompile(`\(net (\d+)`)
	layerRe = regexp.MustCompile(`\(layer "([^"]+)"\)`)
	startRe = regexp.MustCompile(`\((?:start|end|mid|at|center) ([-\d.]+) ([-\d.]+)`)
)

func head(t string) string {
	if m := headRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	return ""
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// header is the file-level settings every cut keeps whole.
var header = map[string]bool{
	"version": true, "generator": true, "generator_version": true, "uuid": true, "paper": true,
	"title_block": true, "sheet_instances": true, "embedded_fonts": true,
}

func cutSheet(path string, b box) (string, []string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a developer tool reading the project its own -src flag names
	if err != nil {
		return "", nil, err
	}
	s := string(raw)
	var keep []string
	var libs string
	used := map[string]bool{}
	var refs []string
	for _, sp := range topItems(s) {
		t := s[sp.a:sp.b]
		switch h := head(t); {
		case h == "lib_symbols":
			libs = t
		case header[h]:
			keep = append(keep, t)
		case h == "wire" || h == "bus" || h == "polyline":
			pts := xyRe.FindAllStringSubmatch(t, -1)
			in := len(pts) > 0
			for _, p := range pts {
				in = in && b.has(num(p[1]), num(p[2]))
			}
			if in {
				keep = append(keep, t)
			}
		case h == "symbol" || h == "junction" || h == "no_connect" || h == "label" ||
			h == "global_label" || h == "hierarchical_label" || h == "text" || h == "bus_entry":
			m := atRe.FindStringSubmatch(t)
			if m == nil || !b.has(num(m[1]), num(m[2])) {
				continue
			}
			keep = append(keep, t)
			if h == "symbol" {
				used[libIDRe.FindStringSubmatch(t)[1]] = true
				if r := refRe.FindStringSubmatch(t); r != nil && !strings.HasPrefix(r[1], "#") {
					refs = append(refs, r[1])
				}
			}
		}
	}
	sort.Strings(refs)
	return assemble(keep, keepLibs(libs, used)), refs, nil
}

// keepLibs rebuilds a lib_symbols list holding only the library symbols a cut uses.
func keepLibs(libs string, used map[string]bool) string {
	if libs == "" {
		return "\t(lib_symbols)"
	}
	inner := "(x" + strings.TrimPrefix(libs, "(lib_symbols") // the same nesting, under a stand-in root
	var parts []string
	for _, sp := range topItems(inner) {
		t := inner[sp.a:sp.b]
		if m := symRe.FindStringSubmatch(t); m != nil && used[m[1]] {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return "\t(lib_symbols)"
	}
	return "\t(lib_symbols\n\t\t" + strings.Join(parts, "\n\t\t") + "\n\t)"
}

// assemble writes the kept items back as a schematic, the header first and lib_symbols after it,
// where KiCad puts them.
func assemble(keep []string, libs string) string {
	var hdr, body []string
	for _, t := range keep {
		if h := head(t); header[h] && h != "sheet_instances" && h != "embedded_fonts" {
			hdr = append(hdr, t)
		} else {
			body = append(body, t)
		}
	}
	return "(kicad_sch\n\t" + strings.Join(hdr, "\n\t") + "\n" + libs + "\n\t" + strings.Join(body, "\n\t") + "\n)\n"
}

func cutRoot(path string, names []string) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a developer tool reading the project its own -src flag names
	if err != nil {
		return "", err
	}
	s := string(raw)
	want := map[string]bool{}
	for _, n := range names {
		want[n+".kicad_sch"] = true
	}
	var keep []string
	for _, sp := range topItems(s) {
		t := s[sp.a:sp.b]
		switch h := head(t); {
		case header[h]:
			keep = append(keep, t)
		case h == "sheet":
			if m := fileRe.FindStringSubmatch(t); m != nil && want[m[1]] {
				keep = append(keep, t)
			}
		}
	}
	return assemble(keep, "\t(lib_symbols)"), nil
}

// boardHeader is the board-level settings every cut keeps whole.
var boardHeader = map[string]bool{
	"version": true, "generator": true, "generator_version": true, "general": true, "paper": true,
	"title_block": true, "layers": true, "setup": true, "property": true, "net": true, "embedded_fonts": true,
}

func cutBoard(path string, refs map[string]bool, margin float64) (string, string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a developer tool reading the project its own -src flag names
	if err != nil {
		return "", "", err
	}
	s := string(raw)
	items := topItems(s)
	// First the kept footprints, their nets, and a box around them.
	nets := map[string]bool{}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	footprints := map[int]bool{}
	for i, sp := range items {
		t := s[sp.a:sp.b]
		if head(t) != "footprint" {
			continue
		}
		r := refRe.FindStringSubmatch(t)
		if r == nil || !refs[r[1]] {
			continue
		}
		footprints[i] = true
		for _, n := range netRe.FindAllStringSubmatch(t, -1) {
			if n[1] != "0" {
				nets[n[1]] = true
			}
		}
		if m := atRe.FindStringSubmatch(t); m != nil {
			x, y := num(m[1]), num(m[2])
			x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
		}
	}
	if len(footprints) == 0 {
		return "", "", fmt.Errorf("no footprint on the board matches the cut's parts")
	}
	b := box{x0 - margin, y0 - margin, x1 + margin, y1 + margin}
	var keep []string
	count := map[string]int{}
	for i, sp := range items {
		t := s[sp.a:sp.b]
		h := head(t)
		ok := false
		switch {
		case boardHeader[h]:
			ok = true
		case h == "footprint":
			ok = footprints[i]
			if ok {
				t = stripUserLayers(t)
			}
		case h == "segment" || h == "arc" || h == "via":
			n := netRe.FindStringSubmatch(t)
			ok = n != nil && nets[n[1]]
			for _, p := range startRe.FindAllStringSubmatch(t, -1) {
				ok = ok && b.has(num(p[1]), num(p[2]))
			}
		case strings.HasPrefix(h, "gr_"):
			l := layerRe.FindStringSubmatch(t)
			ok = l != nil && l[1] == "Edge.Cuts"
		}
		if ok {
			keep = append(keep, t)
			count[h]++
		}
	}
	stats := fmt.Sprintf("%d footprints, %d segments, %d arcs, %d vias, %d outline pieces, %d nets kept with copper, box %.1f,%.1f to %.1f,%.1f mm",
		count["footprint"], count["segment"], count["arc"], count["via"],
		count["gr_line"]+count["gr_arc"]+count["gr_poly"]+count["gr_rect"]+count["gr_circle"], len(nets), b.x0, b.y0, b.x1, b.y1)
	return "(kicad_pcb\n\t" + strings.Join(keep, "\n\t") + "\n)\n", stats, nil
}

// stripUserLayers drops a footprint's direct children that sit on a User layer.
func stripUserLayers(fp string) string {
	inner := "(x" + strings.TrimPrefix(fp, "(footprint")
	var b strings.Builder
	last := 0
	for _, sp := range topItems(inner) {
		t := inner[sp.a:sp.b]
		if l := layerRe.FindStringSubmatch(t); l != nil && strings.HasPrefix(l[1], "User.") && strings.Count(t, "(layer ") == 1 {
			// Drop the item and the indentation before it.
			cut := strings.LastIndexByte(inner[:sp.a], '\n')
			if cut < last {
				cut = sp.a
			}
			b.WriteString(inner[last:cut])
			last = sp.b
		}
	}
	b.WriteString(inner[last:])
	return "(footprint" + strings.TrimPrefix(b.String(), "(x")
}

var (
	padRe    = regexp.MustCompile(`^\(pad "([^"]*)"`)
	padNetRe = regexp.MustCompile(`\(net (\d+) "((?:[^"\\]|\\.)*)"\)`)
)

// renameNets gives each of the board's nets the name the cut schematic's netlist gives its pads.
// It refuses a net whose pads the netlist puts on two different nets, since one number cannot
// carry two names.
func renameNets(board, netlist string) (string, int, error) {
	raw, err := os.ReadFile(netlist) // #nosec G304 -- reads the netlist its own -netlist flag names
	if err != nil {
		return "", 0, err
	}
	var doc struct {
		Nets []struct {
			Name  string `xml:"name,attr"`
			Nodes []struct {
				Ref string `xml:"ref,attr"`
				Pin string `xml:"pin,attr"`
			} `xml:"node"`
		} `xml:"nets>net"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return "", 0, err
	}
	pinNet := map[[2]string]string{}
	for _, n := range doc.Nets {
		for _, nd := range n.Nodes {
			pinNet[[2]string{nd.Ref, nd.Pin}] = n.Name
		}
	}
	oldName := map[string]string{} // net number -> the board's name
	newName := map[string]string{} // net number -> the schematic's name
	for _, sp := range topItems(board) {
		fp := board[sp.a:sp.b]
		if head(fp) != "footprint" {
			continue
		}
		r := refRe.FindStringSubmatch(fp)
		if r == nil {
			continue
		}
		inner := "(x" + strings.TrimPrefix(fp, "(footprint")
		for _, p := range topItems(inner) {
			pad := inner[p.a:p.b]
			pm := padRe.FindStringSubmatch(pad)
			nm := padNetRe.FindStringSubmatch(pad)
			if pm == nil || nm == nil {
				continue
			}
			want, ok := pinNet[[2]string{r[1], pm[1]}]
			if !ok {
				continue
			}
			want = strings.ReplaceAll(want, `"`, `\"`)
			if prev, seen := newName[nm[1]]; seen && prev != want {
				return "", 0, fmt.Errorf("board net %s (%s) holds pads on %q and %q in the cut", nm[1], nm[2], prev, want)
			}
			oldName[nm[1]], newName[nm[1]] = nm[2], want
		}
	}
	renamed := 0
	nums := make([]string, 0, len(newName))
	for n := range newName {
		nums = append(nums, n)
	}
	sort.Strings(nums)
	for _, n := range nums {
		if oldName[n] == newName[n] {
			continue
		}
		board = strings.ReplaceAll(board, fmt.Sprintf("(net %s \"%s\")", n, oldName[n]), fmt.Sprintf("(net %s \"%s\")", n, newName[n]))
		renamed++
	}
	return board, renamed, nil
}
