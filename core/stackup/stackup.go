// Package stackup reads a board's physical layer stack (geom.Stackup) for a person or a check:
// it describes a row in words, sums the rows' thicknesses, and says when that sum disagrees with
// the board thickness the file states (agni 1592). Readers fill the Stackup with what the file
// says and nothing more. Totals and the agreement check happen here, once, for every format.
package stackup

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Tolerance is how far, as a fraction of the stated board thickness, the sum of the layers may
// differ from it before Check reports the difference. Fabricators commonly quote a finished
// thickness to plus or minus 10%.
const Tolerance = 0.10

// MM formats a length in unit_nm units as millimetres, trimmed of trailing zeros, such as
// "0.035" or "1.6".
func MM(v, unitNm int64) string {
	return strconv.FormatFloat(float64(v*unitNm)/1e6, 'f', -1, 64)
}

// Describe is one row in words, in the order name, kind, the layer-table name, thickness, material,
// dielectric type, epsilon_r and loss tangent, leaving out what the file does not give. A row with
// no thickness says "thickness unknown" rather than leaving the gap silent. For example
// "dielectric 1 dielectric 0.1 mm PR1080 prepreg er=3.96 tand=0.02".
func Describe(l *geom.StackLayer, unitNm int64) string {
	parts := []string{l.GetName(), l.GetKind()}
	if l.GetLayer() != "" && l.GetLayer() != l.GetName() {
		parts = append(parts, "layer="+l.GetLayer())
	}
	if l.Thickness != nil {
		parts = append(parts, MM(l.GetThickness(), unitNm)+" mm")
	} else {
		parts = append(parts, "thickness unknown")
	}
	for _, s := range []string{l.GetMaterial(), l.GetDielectricType()} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	if l.EpsilonR != nil {
		parts = append(parts, "er="+strconv.FormatFloat(l.GetEpsilonR(), 'f', -1, 64))
	}
	if l.LossTangent != nil {
		parts = append(parts, "tand="+strconv.FormatFloat(l.GetLossTangent(), 'f', -1, 64))
	}
	return strings.Join(parts, " ")
}

// Sum adds the thicknesses of the rows that give one. missing counts the copper and dielectric
// rows that give none, since those make up the board's bulk and a sum without them is short. A
// surface row with no thickness, such as silkscreen or paste, is left out of both.
func Sum(s *geom.Stackup) (sum int64, missing int) {
	for _, l := range s.GetLayers() {
		switch {
		case l.Thickness != nil:
			sum += l.GetThickness()
		case l.GetKind() == "copper" || l.GetKind() == "dielectric":
			missing++
		}
	}
	return sum, missing
}

// Check compares the sum of the layers with the board thickness the file states, and returns a
// sentence when they differ by more than Tolerance of the stated thickness. It returns "" when
// they agree, and also when the comparison cannot be made: the file states no thickness, or a
// copper or dielectric row has none.
func Check(s *geom.Stackup, unitNm int64) string {
	if s == nil || s.Thickness == nil || s.GetThickness() <= 0 {
		return ""
	}
	sum, missing := Sum(s)
	if missing > 0 || len(s.GetLayers()) == 0 {
		return ""
	}
	stated := s.GetThickness()
	if math.Abs(float64(sum-stated)) <= Tolerance*float64(stated) {
		return ""
	}
	return fmt.Sprintf("the layers sum to %s mm, and the file states a board thickness of %s mm",
		MM(sum, unitNm), MM(stated, unitNm))
}
