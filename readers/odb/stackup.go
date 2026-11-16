package odb

import (
	"math"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// nmPerOunce is the thickness of one ounce of copper per square foot, 1.37 mil.
const nmPerOunce = 34_798

// stackKinds maps a matrix layer type to the Stackup's kind. A board layer of another type, such as
// a drill, a rout or a document layer, is no physical layer of the stack.
var stackKinds = map[string]string{
	"SIGNAL": "copper", "POWER_GROUND": "copper", "MIXED": "copper",
	"DIELECTRIC":   "dielectric",
	"SOLDER_MASK":  "mask",
	"SOLDER_PASTE": "paste",
	"SILK_SCREEN":  "silk",
}

// stackup reads the board's physical stack (agni 1592). The matrix lists the board's layers in
// physical order, top to bottom, so its rows give the stack's order and kinds. Each layer's
// attrlist gives its values.
//
// Tools write .layer_dielectric two ways (agni 1660). KiCad lists each dielectric in the matrix and
// writes every layer's own thickness there, so it wins where present. Altium and some Allegro
// exports list no dielectric in the matrix, and write each copper layer's .layer_dielectric as the
// dielectric below it. For those the reader adds that dielectric under the copper, gives the copper
// its .copper_weight, and reads the last copper's value as no dielectric at all, since Altium writes
// the bottom mask there. .copper_weight is in ounces, except that Allegro writes it in micrometres,
// so a weight of copperWeightMicrons or more reads as micrometres.
//
// .board_thickness comes from the step's attrlist, or the job's misc/attrlist. The stack is nil when
// the matrix has no physical layer. An ODX file carries no attrlist, so its stack gives order and
// kinds, and the total its CAD_DATA_HEADER states.
func (b *boardBuilder) stackup() (*geom.Stackup, error) {
	s := &geom.Stackup{}
	listsDielectric, coppers := false, 0
	for _, l := range b.p.matrix.layers {
		if l.context == "BOARD" && stackKinds[l.typ] == "dielectric" {
			listsDielectric = true
		}
		if l.context == "BOARD" && stackKinds[l.typ] == "copper" {
			coppers++
		}
	}
	copper, added := 0, 0
	for _, l := range b.p.matrix.layers {
		kind, ok := stackKinds[l.typ]
		if l.context != "BOARD" || !ok {
			continue
		}
		name := b.names[strings.ToUpper(l.name)]
		row := &geom.StackLayer{Name: name, Kind: kind}
		if name == "" {
			row.Name = l.name
		}
		if kind == "dielectric" {
			row.DielectricType = strings.ToLower(l.dielType)
		} else {
			row.Layer = name
		}
		attrs, scale, err := b.attrlist(b.p.layerFile(l.name, "attrlist"))
		if err != nil {
			return nil, err
		}
		under, hasUnder := length(attrs[".layer_dielectric"], scale)
		weight, hasWeight := copperWeight(attrs[".copper_weight"])
		switch {
		case kind == "copper" && !listsDielectric:
			if hasWeight {
				row.Thickness = &weight
			}
		case hasUnder:
			row.Thickness = &under
		case kind == "copper" && hasWeight:
			row.Thickness = &weight
		}
		if m := attrs[".material"]; m != "" && !strings.EqualFold(m, "not_specified") {
			row.Material = m
		}
		er, hasEr := positive(attrs[".dielectric_constant"])
		if hasEr {
			row.EpsilonR = &er
		}
		if v, err := strconv.ParseFloat(attrs[".loss_tangent"], 64); err == nil {
			row.LossTangent = &v
		}
		s.Layers = append(s.Layers, row)
		if kind != "copper" {
			continue
		}
		copper++
		if !listsDielectric && hasUnder && copper < coppers {
			added++
			d := &geom.StackLayer{Name: "Dielectric " + strconv.Itoa(added), Kind: "dielectric", Thickness: &under}
			if hasEr {
				d.EpsilonR = &er
			}
			s.Layers = append(s.Layers, d)
		}
	}
	for _, f := range []string{b.p.stepFile("attrlist"), "misc/attrlist"} {
		attrs, scale, err := b.attrlist(f)
		if err != nil {
			return nil, err
		}
		if t, ok := length(attrs[".board_thickness"], scale); ok {
			s.Thickness = &t
			break
		}
	}
	if b.p.odx != nil && b.p.odx.thickness > 0 {
		t := b.p.odx.thickness
		s.Thickness = &t
	}
	if len(s.Layers) == 0 && s.Thickness == nil {
		return nil, nil
	}
	return s, nil
}

// attrlist reads an attrlist file's name=value lines, and the factor from its lengths to
// nanometres. The file's own UNITS line sets the unit. Without one it takes the job's, from
// misc/info, and inches when the job states none, as the spec says. A missing file reads as empty.
func (b *boardBuilder) attrlist(name string) (map[string]string, float64, error) {
	out := map[string]string{}
	scale := unitsInch.coord
	if strings.EqualFold(b.p.info["UNITS"], "MM") {
		scale = unitsMM.coord
	}
	if b.p.odx != nil {
		return out, scale, nil
	}
	raw, err := b.p.readOptional(name)
	if err != nil || raw == nil {
		return out, scale, err
	}
	_ = lines(raw, func(_ int, line string) error {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if strings.EqualFold(k, "UNITS") {
			scale = unitsInch.coord
			if strings.EqualFold(v, "MM") {
				scale = unitsMM.coord
			}
			return nil
		}
		out[strings.ToLower(k)] = v
		return nil
	})
	return out, scale, nil
}

// copperWeightMicrons is the least .copper_weight read as micrometres. Allegro writes 18 and 35
// for half- and one-ounce copper, and no copper is 10 oz thick.
const copperWeightMicrons = 10

// copperWeight reads a .copper_weight as nanometres, in ounces below copperWeightMicrons and in
// micrometres from it.
func copperWeight(s string) (int64, bool) {
	w, ok := positive(s)
	if !ok {
		return 0, false
	}
	if w >= copperWeightMicrons {
		return int64(math.Round(w * 1000)), true
	}
	return int64(math.Round(w * nmPerOunce)), true
}

// positive parses a number above zero.
func positive(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil && v > 0
}

// length parses a positive length in a file's unit as nanometres.
func length(s string, scale float64) (int64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return int64(math.Round(v * scale)), true
}
