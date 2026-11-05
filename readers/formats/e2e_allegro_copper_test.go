package formats

import (
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/panyam/agni/internal/copperraster"
)

// TestAllegroCopperMatchesItsFilms holds each copper layer of the BeagleBone AI, as agni reads it, to
// the Gerber film Allegro 17.2-S049 plotted for that layer from the same .brd (agni issue 1376). Both
// sides are drawn at 1 mil a pixel. A film plots a pour as a region and cuts its voids with clear
// flashes and regions, and agni draws the ETCH shapes it reads with their holes (agni 1357). The films
// also plot the board outline, so agni's outline is drawn too, at the films' 5 mil default width.
//
// Tracks must agree on every layer, and agni must draw nearly all of each film's copper, planes
// included. The whole layer must agree on the outer layers. On an inner layer the films were
// plotted with "Suppress unconnected internal pads: DYNAMIC" (photoplot.log), so Allegro leaves out
// the land of a via or pin that nothing on that layer connects to. That is an artwork setting, and
// agni draws every land the padstack defines, so an inner layer is held to a lower floor.
func TestAllegroCopperMatchesItsFilms(t *testing.T) {
	d, g, _ := readAllegroSample(t)
	films, _ := filepath.Glob(filepath.Join(allegroBoards, "beaglebone-ai", "MFG", "*.art"))
	if len(films) != 12 {
		t.Fatalf("found %d copper films, want the BeagleBone AI's 12; run `make samples-oracle`", len(films))
	}
	byEtch := map[string]string{} // Allegro's layer name -> the contract's
	var order []string
	for _, l := range d.GetLayers() {
		byEtch[l.GetAttributes()["allegro_layer"]] = l.GetName()
		order = append(order, l.GetName())
	}
	var x0, y0, x1, y1 int64 = 1 << 62, 1 << 62, -1 << 62, -1 << 62
	for _, p := range g.GetOutline().GetPaths() {
		for _, q := range p.GetPoints() {
			x0, y0, x1, y1 = min(x0, q.GetX()), min(y0, q.GetY()), max(x1, q.GetX()), max(y1, q.GetY())
		}
	}
	const mil, margin = 25400, 2_000_000
	grid := func() *copperraster.Bitmap { return copperraster.New(x0-margin, y0-margin, x1+margin, y1+margin, mil) }

	seen := map[string]bool{}
	sort.Strings(films)
	for _, f := range films {
		ft, fl := grid(), grid()
		r, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		film, err := copperraster.DrawGerber(r, ft, fl)
		r.Close()
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		layer := byEtch[film.Etch]
		if layer == "" || seen[layer] {
			t.Fatalf("%s plots ETCH/%q, which names no copper layer or one already compared", filepath.Base(f), film.Etch)
		}
		seen[layer] = true
		if ft.Count() == 0 || fl.Count() == 0 {
			t.Fatalf("%s drew %d track and %d pad pixels, want copper of both", filepath.Base(f), ft.Count(), fl.Count())
		}

		at, al := grid(), grid()
		copperraster.DrawBoardLayer(g, layer, order, at, al)
		copperraster.DrawBoardOutline(g, 5*mil, at)
		want, got := copperraster.Union(ft, fl), copperraster.Union(at, al)

		tracks := copperraster.Overlap(ft, at)
		covered := copperraster.Covered(want, got)
		whole := copperraster.Overlap(want, got)
		t.Logf("%-8s %-7s tracks %.3f, film copper drawn %.3f, pads %.3f, whole %.3f", film.Etch, layer, tracks, covered, copperraster.Overlap(fl, al), whole)

		outer := layer == "F.Cu" || layer == "B.Cu"
		if tracks < 0.99 || covered < 0.99 {
			t.Errorf("%s (%s): tracks agree %.3f and agni draws %.3f of the film's copper, want 0.99 each; %s",
				layer, film.Etch, tracks, covered, writeCopperDiff(t, film.Etch, want, got))
		}
		if outer && whole < 0.98 {
			t.Errorf("%s (%s): the whole layer agrees %.3f, want 0.98 on an outer layer; %s",
				layer, film.Etch, whole, writeCopperDiff(t, film.Etch, want, got))
		}
		if !outer && whole < 0.80 {
			t.Errorf("%s (%s): the whole layer agrees %.3f, want 0.80 on an inner layer; %s",
				layer, film.Etch, whole, writeCopperDiff(t, film.Etch, want, got))
		}
	}
	if len(seen) != 12 {
		t.Errorf("compared %d copper layers, want 12", len(seen))
	}
}

// writeCopperDiff saves a layer's difference image outside the tree and says where. Red is copper
// only the film has, blue copper only agni has.
func writeCopperDiff(t *testing.T, name string, want, got *copperraster.Bitmap) string {
	dir, err := os.MkdirTemp("", "allegro-copper-")
	if err != nil {
		return "no difference image: " + err.Error()
	}
	p := filepath.Join(dir, name+".png")
	f, err := os.Create(p)
	if err != nil {
		return "no difference image: " + err.Error()
	}
	defer f.Close()
	if err := png.Encode(f, copperraster.Diff(want, got)); err != nil {
		return "no difference image: " + err.Error()
	}
	return "difference image " + p
}
