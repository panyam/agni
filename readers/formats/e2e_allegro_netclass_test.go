package formats

import (
	"strconv"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestAllegroNetClassesHoldEveryRoutedTrack reads the BeagleBone AI's physical constraint sets as
// net classes (agni issue 1359). Neither extract lists the sets, so the board's own routing is the
// check. Every track Allegro routed is at least as wide as its net's set allows on that layer. A
// differential pair takes its set from its match group, and without that link six pairs read as
// routed under DEFAULT's width. Three segments on F.Cu sit at 4 mil, under DEFAULT's 4.5 mil there
// and at its 4 mil elsewhere, which reads as a neck-down near a part.
func TestAllegroNetClassesHoldEveryRoutedTrack(t *testing.T) {
	d, g, _ := readAllegroSample(t)
	classes := netClassesOf(t, d)
	if len(classes) != 8 {
		t.Fatalf("read %d net classes, want the BeagleBone AI's 8", len(classes))
	}
	if classes["DEFAULT"].GetParams()["is_default"] != "true" {
		t.Error("DEFAULT is not marked the default class")
	}
	if got := classes["DEFAULT"].GetParams()["track_width"]; got != "0.1016" {
		t.Errorf("DEFAULT track_width %q, want 0.1016 (4 mil, its inner layers)", got)
	}
	if got := classes["DEFAULT"].GetParams()["track_width@F.Cu"]; got != "0.1143" {
		t.Errorf("DEFAULT track_width@F.Cu %q, want 0.1143 (4.5 mil)", got)
	}
	members := map[string]int{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetNetClasses() {
			members[c]++
		}
	}
	want := map[string]int{"DDR": 62, "PWR": 33, "EMMC": 17, "100OHM_DIFF": 34, "90OHM_DIFF": 28, "DEFAULT": 1}
	for c, n := range want {
		if members[c] != n {
			t.Errorf("class %s has %d nets, want %d", c, members[c], n)
		}
	}
	held, necked := holdTracksToClasses(t, d, g, classes)
	if held < 9000 {
		t.Fatalf("held %d track segments to their class, want the BeagleBone AI's routed ones", held)
	}
	if necked != 3 {
		t.Errorf("%d segments sit under their layer's width, want the 3 necked on F.Cu", necked)
	}
}

// TestAllegro166NetClassesHoldEveryRoutedTrack does the same on a 16.6 board, whose set names come
// from each set's schematic reference and whose minimum width is the record's first word. On each
// signal layer that width equals the narrowest track parasdr routes there, 0.15 mm outside and
// 0.1 mm on In2 and In3.
func TestAllegro166NetClassesHoldEveryRoutedTrack(t *testing.T) {
	l := &Loader{}
	d, err := l.ReadDesign(parasdr + "parasdr_layout.brd")
	if err != nil {
		t.Fatal(err)
	}
	g, err := l.BoardGeometry(parasdr + "parasdr_layout.brd")
	if err != nil {
		t.Fatal(err)
	}
	classes := netClassesOf(t, d)
	if len(classes) != 2 || classes["DEFAULT"] == nil || classes["DIFFPAIR"] == nil {
		t.Fatalf("read classes %v, want DEFAULT and DIFFPAIR", classes)
	}
	for key, want := range map[string]string{"track_width": "0.1", "track_width@F.Cu": "0.15", "track_width@B.Cu": "0.15", "track_width@In2.Cu": "0.1", "track_width@In3.Cu": "0.1"} {
		if got := classes["DEFAULT"].GetParams()[key]; got != want {
			t.Errorf("DEFAULT %s %q, want %s", key, got, want)
		}
	}
	pairs := 0
	for _, n := range d.GetNets() {
		for _, c := range n.GetNetClasses() {
			if c == "DIFFPAIR" {
				pairs++
			}
		}
	}
	if pairs != 30 {
		t.Errorf("DIFFPAIR has %d nets, want 30", pairs)
	}
	held, necked := holdTracksToClasses(t, d, g, classes)
	if held < 1000 {
		t.Fatalf("held %d track segments to their class, want parasdr's routed ones", held)
	}
	if necked != 0 {
		t.Errorf("%d segments sit under their layer's width, want none", necked)
	}
}

func netClassesOf(t *testing.T, d *ir.Design) map[string]*ir.Constraint {
	t.Helper()
	out := map[string]*ir.Constraint{}
	for _, c := range d.GetConstraints() {
		if c.GetKind() == "netclass" {
			out[c.GetName()] = c
		}
	}
	return out
}

// holdTracksToClasses checks each routed segment against its net's class, or the default class. No
// segment is narrower than the class's track_width. It returns the number of segments held, and the
// number narrower than the width the class states for the segment's own layer.
func holdTracksToClasses(t *testing.T, d *ir.Design, g *geom.BoardGeometry, classes map[string]*ir.Constraint) (held, necked int) {
	t.Helper()
	def := ""
	for name, c := range classes {
		if c.GetParams()["is_default"] == "true" {
			def = name
		}
	}
	classOf := map[string]string{}
	for _, n := range d.GetNets() {
		classOf[n.GetName()] = def
		if cs := n.GetNetClasses(); len(cs) > 0 {
			classOf[n.GetName()] = cs[0]
		}
	}
	nm := func(cls *ir.Constraint, key string) float64 {
		v, err := strconv.ParseFloat(cls.GetParams()[key], 64)
		if err != nil {
			t.Fatalf("class %s: %s %q: %v", cls.GetName(), key, cls.GetParams()[key], err)
		}
		// The limit came through millimetres and the width is whole nanometres, so allow one.
		return v*1e6 - 1
	}
	for _, nc := range g.GetNets() {
		cls := classes[classOf[nc.GetNet()]]
		if cls == nil {
			continue
		}
		for _, s := range nc.GetSegments() {
			w := float64(s.GetWidth())
			if w < nm(cls, "track_width") {
				t.Errorf("net %s on %s: a track %d nm wide, under its class %s's %s mm", nc.GetNet(), s.GetLayer(), s.GetWidth(), cls.GetName(), cls.GetParams()["track_width"])
			}
			if key := "track_width@" + s.GetLayer(); cls.GetParams()[key] != "" && w < nm(cls, key) {
				necked++
			}
			held++
		}
	}
	return held, necked
}
