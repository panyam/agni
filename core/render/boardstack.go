package render

import (
	"github.com/panyam/agni/core/stackup"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// BoardStack is the board's physical layer stack as the viewer's layer list shows it (agni 1603).
// It returns nil for a board whose file states no stack, so the list shows no stack section.
func BoardStack(b *geom.BoardGeometry) *geom.BoardStackView {
	s := b.GetStackup()
	if len(s.GetLayers()) == 0 {
		return nil
	}
	unit := unitNm(b)
	v := &geom.BoardStackView{}
	for _, l := range s.GetLayers() {
		row := &geom.BoardStackRow{Name: l.GetName(), Kind: l.GetKind(), Layer: l.GetLayer(),
			Thickness: thicknessText(l, unit), Material: l.GetMaterial()}
		if l.Thickness != nil {
			mm := float64(l.GetThickness()*unit) / 1e6
			row.ThicknessMm = &mm
		}
		v.Rows = append(v.Rows, row)
	}
	v.Summary, v.Disagrees = stackSummary(s, unit)
	return v
}

// stackSummary compares the stack with the board thickness the file states, in words.
func stackSummary(s *geom.Stackup, unit int64) (string, bool) {
	if c := stackup.Check(s, unit); c != "" {
		return c, true
	}
	sum, missing := stackup.Sum(s)
	stated := s.Thickness != nil && s.GetThickness() > 0
	switch {
	case stated && missing == 0:
		return stackup.MM(s.GetThickness(), unit) + " mm, as the file states", false
	case stated:
		return "the file states a board thickness of " + stackup.MM(s.GetThickness(), unit) +
			" mm, and some layers give no thickness to sum", false
	case missing == 0 && sum > 0:
		return "the layers sum to " + stackup.MM(sum, unit) + " mm, and the file states no board thickness", false
	}
	return "", false
}

// stackThickness gives each layer row the thickness of the stack row whose layer names it. A row
// no stack row names keeps "", so Edge, Through and Zones say nothing.
func stackThickness(b *geom.BoardGeometry, rows []*geom.BoardLayerRow) {
	unit := unitNm(b)
	byLayer := map[string]*geom.StackLayer{}
	for _, l := range b.GetStackup().GetLayers() {
		if l.GetLayer() != "" {
			if _, ok := byLayer[l.GetLayer()]; !ok {
				byLayer[l.GetLayer()] = l
			}
		}
	}
	for _, r := range rows {
		if l, ok := byLayer[r.GetName()]; ok {
			r.Thickness = thicknessText(l, unit)
		}
	}
}

func thicknessText(l *geom.StackLayer, unit int64) string {
	if l.Thickness == nil {
		return "thickness unknown"
	}
	return stackup.MM(l.GetThickness(), unit) + " mm"
}

// unitNm is the board's unit, with KiCad's plain nanometres when the board names none.
func unitNm(b *geom.BoardGeometry) int64 {
	if u := b.GetUnitNm(); u > 0 {
		return u
	}
	return 1
}
