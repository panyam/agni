package geda

import "strings"

// Symbol (.sym) geometry: the pins of a gEDA symbol, needed to place a component's terminals on
// the schematic grid for netlisting. A gEDA pin is a P object:
//
//	P x1 y1 x2 y2 color pintype whichend
//	{ T ... pinnumber=N ... }
//
// whichend (the last field, 0 or 1) selects which endpoint connects to nets: 0 -> (x1,y1),
// 1 -> (x2,y2). The pinnumber attribute in the attached block is the pin designator.

type symbolPin struct {
	x, y    float64
	number  string
	name    string // pinlabel
	pintype string // gEDA pintype attribute (in/out/io/pas/pwr/...); "" when absent
	seq     int    // gEDA pinseq: the 1-based slot-mapping order; 0 when absent (unslotted)
}

// symbolPins extracts the pins from a parsed gEDA .sym file (its lines).
func symbolPins(lines []string) []symbolPin {
	var pins []symbolPin
	i := 0
	for i < len(lines) {
		f := strings.Fields(lines[i])
		if len(f) == 0 || f[0] != "P" {
			i++
			continue
		}
		x1, _ := atof(field(f, 1))
		y1, _ := atof(field(f, 2))
		x2, _ := atof(field(f, 3))
		y2, _ := atof(field(f, 4))
		whichend := 0
		if len(f) >= 8 {
			whichend = atoiInt(f[7])
		}
		cx, cy := x1, y1
		if whichend == 1 {
			cx, cy = x2, y2
		}
		attrs, _, next := readAttrBlock(lines, i+1)
		i = next
		pins = append(pins, symbolPin{x: cx, y: cy, number: attrs["pinnumber"], name: attrs["pinlabel"], pintype: attrs["pintype"], seq: atoiInt(attrs["pinseq"])})
	}
	return pins
}

// symbolSlots reads a gEDA symbol's slot table: the slotdef=SLOT:pin,pin,... lines that map,
// for each slot, the symbol's drawn pins (in pinseq order) onto the physical package pins. A
// multi-gate package (numslots>1) draws one gate and carries a slotdef row per gate, so slot
// K's pin with pinseq i takes the K-th row's i-th number. Returns slot number -> ordered
// physical pin numbers; empty for an unslotted symbol (no slotdef), which the caller reads
// with the drawn pin numbers unchanged.
func symbolSlots(lines []string) map[string][]string {
	slots := map[string][]string{}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "slotdef=") {
			continue
		}
		spec := strings.TrimPrefix(ln, "slotdef=")
		colon := strings.IndexByte(spec, ':')
		if colon < 0 {
			continue
		}
		slot := strings.TrimSpace(spec[:colon])
		var nums []string
		for _, p := range strings.Split(spec[colon+1:], ",") {
			if p = strings.TrimSpace(p); p != "" {
				nums = append(nums, p)
			}
		}
		if slot != "" && len(nums) > 0 {
			slots[slot] = nums
		}
	}
	return slots
}

func field(f []string, i int) string {
	if i < len(f) {
		return f[i]
	}
	return ""
}

// symbolNet returns the net name a power/ground symbol names via its symbol-level net=
// attribute ("net=GND:1" -> "GND"), or "" if the symbol has none. The attribute is a text
// content line; pin attribute blocks never carry net=, so a plain scan is safe.
func symbolNet(lines []string) string {
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "net=") {
			return netFromNetAttr(strings.TrimPrefix(ln, "net="))
		}
	}
	return ""
}
