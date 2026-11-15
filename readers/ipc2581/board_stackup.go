package ipc2581

import (
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// gStackupEl is the board's <Stackup>, as the geometry read takes it. The design read keeps its own
// narrower view (stackupEl) for the IR's provisional Stackup.
type gStackupEl struct {
	OverallThickness string            `xml:"overallThickness,attr"`
	Layers           []gStackupLayerEl `xml:"StackupGroup>StackupLayer"`
}

type gStackupLayerEl struct {
	Ref       string `xml:"layerOrGroupRef,attr"`
	Thickness string `xml:"thickness,attr"`
	Material  string `xml:"materialType,attr"`
	Sequence  string `xml:"sequence,attr"`
	SpecRef   struct {
		ID string `xml:"id,attr"`
	} `xml:"SpecRef"`
}

// specEl is a <Spec> in the CadHeader. A stackup layer names one by its SpecRef for the material
// and the electrical values.
type specEl struct {
	Name       string `xml:"name,attr"`
	General    []specPropEl
	Dielectric []specPropEl
}

type specPropEl struct {
	Type     string `xml:"type,attr"`
	Comment  string `xml:"comment,attr"`
	Property struct {
		Text  string `xml:"text,attr"`
		Value string `xml:"value,attr"`
	} `xml:"Property"`
}

// stackKind maps a layerFunction to the Stackup's kind, and a dielectric's to its dielectric type.
func stackKind(function string) (kind, dielectricType string) {
	switch strings.ToUpper(function) {
	case "CONDUCTOR", "SIGNAL", "PLANE", "POWER_GROUND", "MIXED":
		return "copper", ""
	case "DIELCORE":
		return "dielectric", "core"
	case "DIELPREG":
		return "dielectric", "prepreg"
	case "DIELBASE", "DIELADHV", "DIELECTRIC":
		return "dielectric", ""
	case "SOLDERMASK":
		return "mask", ""
	case "PASTEMASK":
		return "paste", ""
	case "SILKSCREEN", "LEGEND":
		return "silk", ""
	}
	return "other", ""
}

// stackup reads the board's <Stackup> (agni 1592). Its rows give the order, top to bottom, and each
// row's thickness, in the CadHeader's unit or else the dictionaries'. The layer a row names gives
// its kind, and the Spec it names gives its material, epsilon_r and loss tangent. Allegro's export
// brackets the stack with zero-thickness air rows that name no layer, and those are left out. A
// file with no <Stackup> reads as nil.
func (f *ipcGeomFile) stackup(names layerNamer) *geom.Stackup {
	if f.Stackup == nil {
		return nil
	}
	unit := f.CadHeader.Units
	if unit == "" {
		unit = f.units()
	}
	nmPerUnit := unitToNm(unit)
	functions := map[string]string{}
	for _, l := range f.Layers {
		functions[l.Name] = l.Function
	}
	specs := map[string]*specEl{}
	for i := range f.CadHeader.Specs {
		specs[f.CadHeader.Specs[i].Name] = &f.CadHeader.Specs[i]
	}
	s := &geom.Stackup{}
	if t, ok := thicknessNm(f.Stackup.OverallThickness, nmPerUnit); ok && t > 0 {
		s.Thickness = &t
	}
	for _, sl := range f.Stackup.Layers {
		function, declared := functions[sl.Ref]
		t, hasT := thicknessNm(sl.Thickness, nmPerUnit)
		hasT = hasT && t > 0
		if !declared && !hasT {
			continue
		}
		kind, dtype := stackKind(function)
		if !declared && strings.EqualFold(sl.Material, "COPPER") {
			kind = "copper"
		}
		row := &geom.StackLayer{Name: names.of(sl.Ref), Kind: kind, DielectricType: dtype}
		if kind == "dielectric" {
			row.Name = sl.Ref
		} else if declared {
			row.Layer = row.Name
		}
		if hasT {
			row.Thickness = &t
		}
		if sp := specs[sl.SpecRef.ID]; sp != nil {
			for _, g := range sp.General {
				if strings.EqualFold(g.Type, "MATERIAL") {
					row.Material = g.Property.Text
				}
			}
			for _, d := range sp.Dielectric {
				v, err := strconv.ParseFloat(d.Property.Value, 64)
				if err != nil {
					continue
				}
				switch strings.ToUpper(d.Type) {
				case "DIELECTRIC_CONSTANT":
					row.EpsilonR = &v
				case "LOSS_TANGENT":
					row.LossTangent = &v
				}
			}
		}
		if row.Material == "" {
			row.Material = sl.Material
		}
		s.Layers = append(s.Layers, row)
	}
	return s
}
