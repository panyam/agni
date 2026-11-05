package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/render"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The viewer shows no layout picker, so a computed layout must say why it is one and what would
// draw the design instead.
func TestDrawingNoteNamesWhatWouldDrawTheDesign(t *testing.T) {
	cases := []struct {
		file string
		want []string // substrings; nil means no note
	}{
		{"x.edn", []string{"x.edn holds no schematic drawing", "Open the design's .eds"}},
		{"x.edf", []string{"Open the design's .eds"}},
		{"b.kicad_pcb", []string{"b.kicad_pcb holds no schematic drawing", "Open the design's .kicad_sch"}},
		{"j.tel", []string{"j.tel holds no schematic drawing", "from a .dsn, .eds,", ".kicad_sch or .sch file", "as a companion"}},
		{"board.PcbDoc", []string{"board.PcbDoc holds no schematic drawing", "as a companion"}},
		{"BeagleBone-AI.brd", []string{"as a companion"}},
		{"x.eds", nil},
		{"x.kicad_sch", nil},
		{"x.kicad_pro", nil},
		{"CLAYR6.DSN", nil},
	}
	for _, c := range cases {
		got := drawingNote("designs/" + c.file)
		if c.want == nil {
			if got != "" {
				t.Errorf("drawingNote(%s) = %q, want none for a file with its own drawing", c.file, got)
			}
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("drawingNote(%s) = %q, lacks %q", c.file, got, w)
			}
		}
	}
}

func TestGetDesignCarriesTheDrawingNote(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "R1"}}}
	svc := NewDesignService(fakeLoader{design: d, geom: twoSheetGeom()}, noNative{}, render.Style{}, nil)
	for file, want := range map[string]bool{"x.edn": true, "x.eds": false} {
		resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/"+file)})
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.GetDrawingNote() != ""; got != want {
			t.Errorf("%s: drawing_note = %q, want a note: %v", file, resp.GetDrawingNote(), want)
		}
	}
}
