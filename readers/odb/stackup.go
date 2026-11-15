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
// attrlist gives its values. .layer_dielectric is a thickness in the file's length unit. KiCad
// writes it on copper and mask layers too, so it wins where present, and a copper layer without it
// takes its thickness from .copper_weight in ounces. .board_thickness comes from the step's
// attrlist, or the job's misc/attrlist. The stack is nil when the matrix has no physical layer.
// An ODX file carries no attrlist, so its stack gives order and kinds only.
func (b *boardBuilder) stackup() (*geom.Stackup, error) {
	s := &geom.Stackup{}
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
		if t, ok := length(attrs[".layer_dielectric"], scale); ok {
			row.Thickness = &t
		} else if w, err := strconv.ParseFloat(attrs[".copper_weight"], 64); err == nil && kind == "copper" && w > 0 {
			t := int64(math.Round(w * nmPerOunce))
			row.Thickness = &t
		}
		if m := attrs[".material"]; m != "" && !strings.EqualFold(m, "not_specified") {
			row.Material = m
		}
		if v, err := strconv.ParseFloat(attrs[".dielectric_constant"], 64); err == nil {
			row.EpsilonR = &v
		}
		if v, err := strconv.ParseFloat(attrs[".loss_tangent"], 64); err == nil {
			row.LossTangent = &v
		}
		s.Layers = append(s.Layers, row)
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

// length parses a positive length in a file's unit as nanometres.
func length(s string, scale float64) (int64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return int64(math.Round(v * scale)), true
}
