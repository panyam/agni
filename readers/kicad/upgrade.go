package kicad

import "github.com/panyam/agni/internal/sexpr"

// upgrade respells what KiCad 6 and 7 wrote into the form KiCad 8 writes, once after parsing, so
// every reader below sees one spelling. Before it, a KiCad 6 root sheet named no sub-sheets and read
// as an empty design, and a KiCad 6 or 7 board had no footprint with a reference, so a .kicad_pro read
// took its nets from an empty board (agni issue 1233). A file already in the KiCad 8 form passes
// through unchanged.
//
//   - A sheet's (property "Sheet name" ...) and (property "Sheet file" ...) become "Sheetname" and
//     "Sheetfile", KiCad 7's spelling.
//   - A footprint's (fp_text reference "R1" ...) and (fp_text value ...) become (property
//     "Reference" "R1" ...) and (property "Value" ...), with a bare hide atom becoming (hide yes).
//     A footprint that already carries the property keeps it.
//
// Parsed atoms are shared between nodes (internal/sexpr), so a respelled node gets new atoms rather
// than having one edited in place.
func upgrade(root *node) {
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil || !n.IsList {
			return
		}
		switch n.Head() {
		case "sheet":
			for _, p := range n.Children("property") {
				if to, ok := sheetPropRenames[atomOf(p.Arg(1))]; ok {
					p.Kids[1] = quoted(to)
				}
			}
		case "footprint":
			upgradeFootprintText(n)
			return // nothing inside a footprint nests another sheet or footprint
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(root)
}

var sheetPropRenames = map[string]string{"Sheet name": "Sheetname", "Sheet file": "Sheetfile"}

// fpTextProps maps the fp_text kinds that KiCad 8 made properties onto those properties' names.
var fpTextProps = map[string]string{"reference": "Reference", "value": "Value"}

func upgradeFootprintText(fp *node) {
	for i, k := range fp.Kids {
		if k.Head() != "fp_text" {
			continue
		}
		name, ok := fpTextProps[atomOf(k.Arg(1))]
		if !ok || propNode(fp, name) != nil || len(k.Kids) < 3 {
			continue
		}
		kids := []*node{{Atom: "property"}, quoted(name), k.Kids[2]}
		for _, rest := range k.Kids[3:] {
			if !rest.IsList && rest.Atom == "hide" {
				rest = &node{IsList: true, Kids: []*node{{Atom: "hide"}, {Atom: "yes"}}}
			}
			kids = append(kids, rest)
		}
		fp.Kids[i] = &node{IsList: true, Kids: kids}
	}
}

func quoted(s string) *node { return &sexpr.Node{Atom: s, Quoted: true} }
