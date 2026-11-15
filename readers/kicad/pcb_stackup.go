package kicad

import (
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// boardStackup reads the board's physical stack from (setup (stackup ...)) and the board's total
// thickness from (general (thickness t)), both in millimetres (agni 1592). It returns nil when the
// file gives neither. layer names the layer-table row a stack row is, and a dielectric has none.
//
// A dielectric KiCad builds from several plies writes the first ply's values, then the atom
// addsublayer before each further ply's. Each ply is a physical layer, so each becomes its own row
// under the dielectric's name, and the rows' thicknesses sum to the dielectric's.
func boardStackup(root *node, tableNames map[string]bool) *geom.Stackup {
	s := &geom.Stackup{}
	if t := root.Child("general").Child("thickness"); t != nil {
		v := mmToNm(atomOf(t.Arg(1)))
		s.Thickness = &v
	}
	for _, l := range root.Child("setup").Child("stackup").Children("layer") {
		name := atomOf(l.Arg(1))
		kind, dtype := stackKind(atomOf(l.Child("type").Arg(1)))
		row := &geom.StackLayer{Name: name, Kind: kind, DielectricType: dtype}
		if kind != "dielectric" && tableNames[name] {
			row.Layer = name
		}
		for _, k := range l.Kids[2:] {
			if !k.IsList {
				if atomOf(k) == "addsublayer" {
					s.Layers = append(s.Layers, row)
					row = &geom.StackLayer{Name: name, Kind: kind, DielectricType: dtype}
				}
				continue
			}
			switch k.Head() {
			case "thickness":
				v := mmToNm(atomOf(k.Arg(1)))
				row.Thickness = &v
			case "material":
				row.Material = atomOf(k.Arg(1))
			case "epsilon_r":
				row.EpsilonR = floatOf(atomOf(k.Arg(1)))
			case "loss_tangent":
				row.LossTangent = floatOf(atomOf(k.Arg(1)))
			}
		}
		s.Layers = append(s.Layers, row)
	}
	if s.Thickness == nil && len(s.Layers) == 0 {
		return nil
	}
	return s
}

// stackKind maps a stackup layer's type word to the contract's kind, and a dielectric's to its
// dielectric type. KiCad writes "copper", "core" and "prepreg" for the layers between the
// surfaces, and phrases such as "Top Solder Mask" for the surface layers.
func stackKind(typ string) (kind, dielectricType string) {
	t := strings.ToLower(typ)
	switch {
	case t == "copper":
		return "copper", ""
	case t == "core" || t == "prepreg":
		return "dielectric", t
	case strings.Contains(t, "mask"):
		return "mask", ""
	case strings.Contains(t, "paste"):
		return "paste", ""
	case strings.Contains(t, "silk"):
		return "silk", ""
	}
	return "other", ""
}

// floatOf parses a number the file gives, or returns nil for one it does not.
func floatOf(s string) *float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}
