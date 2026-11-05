package copperraster

import (
	"strings"
	"testing"
)

const mil = 25400.0

// grid is a 1-mil bitmap over a 1-inch square with its corner at the origin.
func grid() *Bitmap { return New(0, 0, 1000*25400, 1000*25400, 25400) }

func film(body string) string {
	return "G04 Layer:  ETCH/TOP*\n%FSLAX25Y25*MOIN*%\n%IR0*IPPOS*OFA0.00000B0.00000*MIA0B0*SFA1.00000B1.00000*%\n" + body + "M02*\n"
}

func TestAFilmDrawsItsTracksAndFlashes(t *testing.T) {
	tracks, lands := grid(), grid()
	f, err := DrawGerber(strings.NewReader(film(
		"%ADD10C,.01*%\n%ADD11R,.04X.02*%\nG54D10*\nX10000Y10000D02*\nX50000Y10000D01*\nG54D11*\nX80000Y80000D03*\n")), tracks, lands)
	if err != nil {
		t.Fatal(err)
	}
	if f.Etch != "TOP" || f.Draws != 1 || f.Flashes != 1 {
		t.Errorf("film %+v, want ETCH TOP with one draw and one flash", f)
	}
	// A 0.4 in by 10 mil stroke with round ends, and a 40 by 20 mil flash, at 1 mil a pixel.
	if n := tracks.Count(); n < 4000 || n > 4200 {
		t.Errorf("track drew %d pixels, want about 4,080", n)
	}
	if n := lands.Count(); n < 780 || n > 820 {
		t.Errorf("flash drew %d pixels, want about 800", n)
	}
	if !lands.Get(800, 800) || lands.Get(830, 800) {
		t.Error("the flash is not centred on (0.8, 0.8) in")
	}
}

func TestRegionsAndClearCopperAreLeftOut(t *testing.T) {
	b := grid()
	f, err := DrawGerber(strings.NewReader(film(
		"%ADD10C,.01*%\nG54D10*\nG36*\nX0Y0D02*\nX50000Y0D01*\nX50000Y50000D01*\nG37*\n%LPC*%\nX10000Y10000D02*\nX40000Y10000D01*\n%LPD*%\n")), b, b)
	if err != nil {
		t.Fatal(err)
	}
	if b.Count() != 0 || f.Regions != 1 || f.Clear != 1 {
		t.Errorf("drew %d pixels and counted %+v, want nothing drawn, one region and one clear draw", b.Count(), f)
	}
}

func TestAnArcFollowsItsCentre(t *testing.T) {
	b := grid()
	// A counter-clockwise half circle of radius 0.2 in about (0.5, 0.5).
	_, err := DrawGerber(strings.NewReader(film(
		"%ADD10C,.01*%\nG54D10*\nG75*\nX70000Y50000D02*\nG03X30000Y50000I-20000J0D01*\n")), b, b)
	if err != nil {
		t.Fatal(err)
	}
	if !b.Get(500, 700) || b.Get(500, 300) {
		t.Error("the arc did not pass over the top of its circle, or passed under it")
	}
}

func TestMacrosDrawTheirShape(t *testing.T) {
	b := grid()
	_, err := DrawGerber(strings.NewReader(film(
		"%AMSQ*\n21,1,.04,.04,0.0,0.0,45.*%\n%ADD10SQ*%\n%AMTRI*\n4,1,3,\n0,0,\n.04,0,\n0,.04,\n0,0,\n0.0*%\n%ADD11TRI*%\n"+
			"G54D10*\nX20000Y20000D03*\nG54D11*\nX60000Y60000D03*\n")), b, b)
	if err != nil {
		t.Fatal(err)
	}
	// A 40 mil square turned 45 degrees reaches 28 mil along each axis from its centre.
	if !b.Get(200, 226) || b.Get(200, 232) {
		t.Error("the turned square's corner is not where 45 degrees puts it")
	}
	// The triangle covers its right-angle corner and not the far side of its hypotenuse.
	if !b.Get(605, 605) || b.Get(635, 635) {
		t.Error("the outline macro did not fill its triangle")
	}
}

func TestAFilmOutsideTheSubsetIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"step and repeat":  "%SRX2Y2I1J1*%\n",
		"single quadrant":  "G74*\n",
		"other macro":      "%AMX*\n1,1,.01,0,0*%\n",
		"offset image":     "%OFA1B0*%\n",
		"unknown aperture": "%ADD10P,.01X6*%\n",
		"unsupported word": "%ADD10C,.01*%\nG54D10*\nX0Y0K5D01*\n",
		"no end of file":   "",
	} {
		src := film(body)
		if name == "no end of file" {
			src = strings.TrimSuffix(src, "M02*\n")
		}
		if _, err := DrawGerber(strings.NewReader(src), grid(), grid()); err == nil {
			t.Errorf("%s: drew without an error", name)
		}
	}
}

func TestOverlapAndCovered(t *testing.T) {
	a, b := grid(), grid()
	a.Rect(200*mil, 200*mil, 100*mil, 100*mil, 0)
	b.Rect(250*mil, 200*mil, 100*mil, 100*mil, 0)
	if o := Overlap(a, a); o != 1 {
		t.Errorf("a bitmap overlaps itself %.3f, want 1", o)
	}
	if o := Overlap(a, b); o < 0.32 || o > 0.35 {
		t.Errorf("two squares sharing half their width overlap %.3f, want a third", o)
	}
	if c := Covered(a, Union(a, b)); c != 1 {
		t.Errorf("a union covers %.3f of its part, want all", c)
	}
}
