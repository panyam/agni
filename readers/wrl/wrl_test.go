package wrl

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/readers/step"
)

// A quad and a triangle sharing one material by DEF and USE, inside a Transform that moves them up
// by 10 and turns them 90° about Z. The comments, commas and a skipped Viewpoint are as KiCad's and
// other exporters write them.
const twoShapes = `#VRML V2.0 utf8
# a comment, with { braces } in it
Viewpoint { position 0 0 10 description "front" }
Transform {
  translation 0 0 10
  rotation 0 0 1 1.5707963267948966
  children [
    Shape { appearance Appearance {material DEF RED Material {
        diffuseColor 1 0 0
        transparency 0.25
        }}
      geometry IndexedFaceSet { solid FALSE creaseAngle 0.5
        coordIndex [0,1,2,3,-1]
        coord Coordinate { point [0 0 0, 2 0 0, 2 1 0, 0 1 0] }
      }
    }
    Shape { geometry IndexedFaceSet
      { coordIndex [0 1 2 -1]
        coord Coordinate { point [0 0 1 1 0 1 0 1 1] } }
      appearance Appearance{material USE RED }
    }
  ]
}
`

func TestReadSplitsFacesAndAppliesTransforms(t *testing.T) {
	m, err := Read([]byte(twoShapes))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(m.Parts))
	}
	quad, tri := m.Parts[0], m.Parts[1]
	if got := len(quad.Indices); got != 6 {
		t.Errorf("the quad gave %d indices, want two triangles' 6", got)
	}
	if got := len(tri.Indices); got != 3 {
		t.Errorf("the triangle gave %d indices, want 3", got)
	}
	want := [4]float64{1, 0, 0, 0.75}
	if quad.Color != want || tri.Color != want {
		t.Errorf("colours %v and %v, want %v from the DEF and its USE", quad.Color, tri.Color, want)
	}
	// (2, 0, 0) turned 90° about Z and moved up 10 is (0, 2, 10).
	p := quad.Positions[1]
	if math.Abs(p[0]) > 1e-9 || math.Abs(p[1]-2) > 1e-9 || math.Abs(p[2]-10) > 1e-9 {
		t.Errorf("point 1 is %v, want (0, 2, 10)", p)
	}
}

func TestReadRefusesBrokenFiles(t *testing.T) {
	for name, src := range map[string]string{
		"undefined USE": `Shape { appearance Appearance { material USE NOPE } }`,
		"unclosed list": `Shape { geometry IndexedFaceSet { coordIndex [0 1 2 `,
		"PROTO":         `PROTO Thing [] { Group {} }`,
	} {
		if _, err := Read([]byte(src)); !errors.Is(err, ErrSyntax) {
			t.Errorf("%s: got %v, want ErrSyntax", name, err)
		}
	}
}

// KiCad's library ships each model as a STEP and a .wrl in tenths of an inch, and agni takes heights
// from the STEP and draws the .wrl. This holds every pinned .wrl to its STEP's top within 0.15 mm.
// It compares tops alone, since a STEP's extent counts curve control points and so overshoots
// sideways, such as CP_Elec_6.3x5.9's ±4.3 mm for a 6.3 mm can. The .wrl files come from KiCad 9.0.0
// and the STEPs from 10.0.6, and SOT-143's body grew 0.1 mm between the two.
func TestKiCadWRLsReachTheirSTEPsTop(t *testing.T) {
	dir := filepath.Join("..", "..", "tools", "samples", "remote", "kicad-models")
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.wrl"))
	if len(files) < 66 {
		t.Fatalf("found %d .wrl models under %s, want 66; run `make samples-oracle`", len(files), dir)
	}
	checked := 0
	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 -- a test fixture path
		if err != nil {
			t.Fatal(err)
		}
		m, err := Read(data)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		m.Scale(2.54)
		_, hi, ok := m.Bounds()
		if !ok || m.Triangles() == 0 {
			t.Errorf("%s: no triangles", filepath.Base(f))
			continue
		}
		sdata, err := os.ReadFile(strings.TrimSuffix(f, ".wrl") + ".step") // #nosec G304 -- as above
		if err != nil {
			t.Fatal(err)
		}
		pts, err := step.Points(sdata)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		shi := pts[0]
		for _, q := range pts {
			shi[2] = math.Max(shi[2], q[2])
		}
		if math.Abs(hi[2]-shi[2]) > 0.15 {
			t.Errorf("%s: reaches %.3f mm, its STEP %.3f", filepath.Base(f), hi[2], shi[2])
		}
		checked++
	}
	if checked != len(files) {
		t.Errorf("checked %d of %d models", checked, len(files))
	}
}
