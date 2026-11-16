package altium

import (
	"math"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// stackup reads the board's physical stack from Board6 (agni 1601). The copper layers run in the
// order copperStack follows. Each copper layer's LAYER<n>COPTHICK is its thickness, and the
// dielectric below it, down to the next copper layer, is LAYER<n>DIEL*. A solder mask covers each
// face when TOPHEIGHT or BOTTOMHEIGHT states one. Altium states no board thickness apart from the
// layers, so the stack states none. copper names each copper layer in the contract's vocabulary.
func stackup(b props, stack []int, copper map[int]string) *geom.Stackup {
	s := &geom.Stackup{}
	mask := func(side, layer string) {
		t, ok := thickness(b[side+"HEIGHT"])
		if !ok || t == 0 {
			return
		}
		row := &geom.StackLayer{Name: layer, Kind: "mask", Layer: layer, Thickness: &t, Material: b[side+"MATERIAL"]}
		row.EpsilonR = constant(b[side+"CONST"])
		s.Layers = append(s.Layers, row)
	}
	mask("TOP", "F.Mask")
	for i, id := range stack {
		key := "LAYER" + strconv.Itoa(id)
		row := &geom.StackLayer{Name: b[key+"NAME"], Kind: "copper", Layer: copper[id]}
		if row.Name == "" {
			row.Name = copper[id]
		}
		if t, ok := thickness(b[key+"COPTHICK"]); ok {
			row.Thickness = &t
		}
		s.Layers = append(s.Layers, row)
		if i == len(stack)-1 {
			break
		}
		d := &geom.StackLayer{Name: "Dielectric " + strconv.Itoa(i+1), Kind: "dielectric", Material: b[key+"DIELMATERIAL"]}
		if t, ok := thickness(b[key+"DIELHEIGHT"]); ok {
			d.Thickness = &t
		}
		d.DielectricType = dielectricTypes[b[key+"DIELTYPE"]]
		d.EpsilonR = constant(b[key+"DIELCONST"])
		s.Layers = append(s.Layers, d)
	}
	mask("BOTTOM", "B.Mask")
	return s
}

// dielectricTypes names Board6's DIELTYPE codes. 3 is a surface material, which Altium uses for the
// solder masks.
var dielectricTypes = map[string]string{"1": "core", "2": "prepreg"}

// thickness reads a Board6 length such as "1.4mil" or "0.035mm" into nanometres. A bare number is
// in mils, as Board6 writes its lengths.
func thickness(s string) (int64, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	scale := 25_400.0
	switch {
	case strings.HasSuffix(s, "mil"):
		s = strings.TrimSuffix(s, "mil")
	case strings.HasSuffix(s, "mm"):
		s, scale = strings.TrimSuffix(s, "mm"), 1e6
	case strings.HasSuffix(s, "um"):
		s, scale = strings.TrimSuffix(s, "um"), 1e3
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return int64(math.Round(v * scale)), true
}

// constant reads a dielectric constant, or nil when Board6 states none.
func constant(s string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}
