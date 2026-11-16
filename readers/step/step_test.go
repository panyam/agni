package step

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// stepFile wraps DATA-section lines in a STEP file's header and trailer.
func stepFile(unit string, data ...string) []byte {
	if unit == "" {
		unit = "#900=( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );"
	}
	return []byte("ISO-10303-21;\nHEADER;\nFILE_NAME('t.step','',(''),(''),'','','');\nENDSEC;\nDATA;\n" +
		unit + "\n" + strings.Join(data, "\n") + "\nENDSEC;\nEND-ISO-10303-21;\n")
}

// box is a 1 x 2 x 3 mm block's eight corners as VERTEX_POINTs, plus a LINE whose origin sits 7 mm
// up. A line's origin is not on the part, so it must not count.
var box = []string{
	"#1=CARTESIAN_POINT('',(0.,0.,0.));", "#2=CARTESIAN_POINT('',(1.,0.,0.));",
	"#3=CARTESIAN_POINT('',(0.,2.,0.));", "#4=CARTESIAN_POINT('',(1.,2.,0.));",
	"#5=CARTESIAN_POINT('',(0.,0.,3.));", "#6=CARTESIAN_POINT('',(1.,0.,3.));",
	"#7=CARTESIAN_POINT('',(0.,2.,3.));", "#8=CARTESIAN_POINT('',(1.,2.,3.E+000));",
	"#11=VERTEX_POINT('',#1);", "#12=VERTEX_POINT('',#2);", "#13=VERTEX_POINT('',#3);",
	"#14=VERTEX_POINT('',#4);", "#15=VERTEX_POINT('',#5);", "#16=VERTEX_POINT('',#6);",
	"#17=VERTEX_POINT('',#7);", "#18=VERTEX_POINT('',#8);",
	"#20=CARTESIAN_POINT('a line origin; (not on the part)',(0.5,1.,7.));",
	"#21=DIRECTION('',(0.,1.,0.));", "#22=VECTOR('',#21,1.);", "#23=LINE('',#20,#22);",
}

func extent(t *testing.T, pts [][3]float64) (lo, hi [3]float64) {
	t.Helper()
	if len(pts) == 0 {
		t.Fatal("no points")
	}
	lo, hi = pts[0], pts[0]
	for _, p := range pts {
		for k := range p {
			lo[k], hi[k] = math.Min(lo[k], p[k]), math.Max(hi[k], p[k])
		}
	}
	return lo, hi
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPointsTakeVerticesAndSkipALinesOrigin(t *testing.T) {
	pts, err := Points(stepFile("", box...))
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := extent(t, pts)
	if lo != [3]float64{0, 0, 0} || !near(hi[0], 1) || !near(hi[1], 2) || !near(hi[2], 3) {
		t.Errorf("extent %v to %v, want 0,0,0 to 1,2,3", lo, hi)
	}
}

func TestPointsReadInches(t *testing.T) {
	inch := "#900=( CONVERSION_BASED_UNIT('INCH',#901) LENGTH_UNIT() NAMED_UNIT(#902) );"
	pts, err := Points(stepFile(inch, box...))
	if err != nil {
		t.Fatal(err)
	}
	if _, hi := extent(t, pts); !near(hi[2], 3*25.4) {
		t.Errorf("top %g mm, want %g", hi[2], 3*25.4)
	}
}

func TestPointsReadMetres(t *testing.T) {
	m := "#900=( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT($,.METRE.) );"
	pts, err := Points(stepFile(m, box...))
	if err != nil {
		t.Fatal(err)
	}
	if _, hi := extent(t, pts); !near(hi[2], 3000) {
		t.Errorf("top %g mm, want 3000", hi[2])
	}
}

// A B-spline surface bulging above the box's top face reaches its control points, which a vertex
// alone misses.
func TestPointsTakeBSplineControlPoints(t *testing.T) {
	data := append(append([]string{}, box...),
		"#30=CARTESIAN_POINT('',(0.,0.,3.));", "#31=CARTESIAN_POINT('',(0.5,1.,4.5));",
		"#32=CARTESIAN_POINT('',(1.,2.,3.));",
		"#33=( BOUNDED_SURFACE() B_SPLINE_SURFACE(1,1,((#30,#31),(#31,#32)),.UNSPECIFIED.,.F.,.F.,.F.) "+
			"B_SPLINE_SURFACE_WITH_KNOTS((2,2),(2,2),(0.,1.),(0.,1.),.UNSPECIFIED.) GEOMETRIC_REPRESENTATION_ITEM() "+
			"RATIONAL_B_SPLINE_SURFACE(((1.,1.),(1.,1.))) REPRESENTATION_ITEM('') SURFACE() );")
	pts, err := Points(stepFile("", data...))
	if err != nil {
		t.Fatal(err)
	}
	if _, hi := extent(t, pts); !near(hi[2], 4.5) {
		t.Errorf("top %g, want 4.5", hi[2])
	}
}

// A cylinder lying along X, radius 1 about z = 3, has its seam vertices at its sides. Its circle
// reaches z = 4.
func TestPointsTakeACirclesReach(t *testing.T) {
	data := append(append([]string{}, box...),
		"#40=CARTESIAN_POINT('',(0.,1.,3.));", "#41=DIRECTION('',(1.,0.,0.));", "#42=DIRECTION('',(0.,1.,0.));",
		"#43=AXIS2_PLACEMENT_3D('',#40,#41,#42);", "#44=CIRCLE('',#43,1.);")
	pts, err := Points(stepFile("", data...))
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := extent(t, pts)
	if !near(hi[2], 4) || !near(lo[0], 0) || !near(hi[0], 1) {
		t.Errorf("extent %v to %v, want z up to 4 and x still 0 to 1", lo, hi)
	}
}

func TestPointsRefuseAPlacedSubPart(t *testing.T) {
	data := append(append([]string{}, box...),
		"#50=CARTESIAN_POINT('',(0.,0.,0.));", "#51=CARTESIAN_POINT('',(5.,0.,0.));",
		"#52=AXIS2_PLACEMENT_3D('',#50,$,$);", "#53=AXIS2_PLACEMENT_3D('',#51,$,$);",
		"#54=ITEM_DEFINED_TRANSFORMATION('','',#52,#53);")
	if _, err := Points(stepFile("", data...)); !errors.Is(err, ErrAssembly) {
		t.Errorf("err = %v, want ErrAssembly", err)
	}
}

// KiCad's library writes an identity transform for a part with one body. It moves nothing, so the
// part reads.
func TestPointsAcceptAnIdentityTransform(t *testing.T) {
	data := append(append([]string{}, box...),
		"#50=CARTESIAN_POINT('',(0.,0.,0.));", "#51=CARTESIAN_POINT('',(0.,0.,0.));",
		"#52=AXIS2_PLACEMENT_3D('',#50,$,$);", "#53=AXIS2_PLACEMENT_3D('',#51,$,$);",
		"#54=ITEM_DEFINED_TRANSFORMATION('','',#52,#53);")
	if _, err := Points(stepFile("", data...)); err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

func TestPointsRefuseWhatIsNotAShape(t *testing.T) {
	for name, data := range map[string][]byte{
		"not STEP":    []byte("solid x\nendsolid x\n"),
		"no vertices": stepFile("", "#1=CARTESIAN_POINT('',(0.,0.,9.));"),
		"two units": stepFile("#900=( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );\n"+
			"#901=( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT($,.METRE.) );", box...),
	} {
		if _, err := Points(data); err == nil {
			t.Errorf("%s: read with no error", name)
		}
	}
}
