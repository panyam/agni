package main

import (
	"context"
	"github.com/panyam/agni/artifact"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/graph"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/readers/formats"
)

// TestOsLoaderCompanionGeometry checks that serving a netlist with a sibling .eds draws on the .eds
// schematic (WS1-047), so GetDesign/GetSheet/HighlightSheet (all funneling through Geometry) show
// the design's own drawing; a netlist without a sibling falls back to the auto-layout graph.
func TestOsLoaderCompanionGeometry(t *testing.T) {
	l := &osLoader{mounts: []mounts.Mount{{Name: "m", Root: filepath.Join("testdata", "review")}}, loader: &formats.Loader{}}

	// With a sibling .eds -> the companion schematic (its page sheet "P1", its named wire SIGA).
	g, err := l.Geometry(context.Background(), mustURI("m", "companion-demo.edn"), graph.DefaultStrategy, false)
	if err != nil {
		t.Fatalf("companion geometry: %v", err)
	}
	if len(g.GetSheets()) == 0 || g.GetSheets()[0].GetId() != "P1" {
		t.Fatalf("want the companion .eds sheet P1, got %v", sheetIDs(g))
	}
	if !hasWireNet(g, "SIGA") {
		t.Error("companion geometry is missing the SIGA wire (the .eds was not used)")
	}

	// Without a sibling -> the auto-layout graph, as before.
	g2, err := l.Geometry(context.Background(), mustURI("m", "can-broken.edn"), graph.DefaultStrategy, false)
	if err != nil {
		t.Fatalf("auto-layout geometry: %v", err)
	}
	if len(g2.GetSheets()) == 0 || g2.GetSheets()[0].GetId() != "graph" {
		t.Errorf("a netlist with no companion should render the auto-layout graph, got %v", sheetIDs(g2))
	}
}

func sheetIDs(g *geom.SchematicGeometry) []string {
	var ids []string
	for _, s := range g.GetSheets() {
		ids = append(ids, s.GetId())
	}
	return ids
}

func hasWireNet(g *geom.SchematicGeometry, net string) bool {
	for _, s := range g.GetSheets() {
		for _, w := range s.GetWires() {
			if w.GetNet() == net {
				return true
			}
		}
	}
	return false
}

// mustURI builds an artifact URI from a (mount, path) pair the test itself declared. It panics
// rather than returning an error, since a fixture URI that will not parse is a broken test.
func mustURI(mount, p string) artifact.URI {
	u, err := artifact.New(mount, p)
	if err != nil {
		panic(err)
	}
	return u
}
