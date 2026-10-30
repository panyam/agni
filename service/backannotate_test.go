package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/edif"
)

// A netlist names R1001 and U101, and its schematic companion draws them as R4 and as no designator,
// back-annotating both to the netlist's names (agni issue 1228). A query cell and a finding on either
// part then locate to the page the part is drawn on, rather than reading as drawn nowhere.
func TestBackAnnotatedPartsLocateOnTheirSheet(t *testing.T) {
	dir := filepath.Join("..", "cmd", "agni", "testdata", "github-issues", "backannotate-locate")
	open := func(name string) *os.File {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	d, err := edif.Read(open("demo_netlist.edn"), "demo_netlist.edn")
	if err != nil {
		t.Fatal(err)
	}
	g, err := edif.ReadSchematic(open("demo_schematic.eds"), "demo_schematic.eds")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"R1001": "P1", "U101": "P2"}

	svc := NewQueryService(fakeLoader{design: d, geom: g}, nil, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/demo_netlist.edn"),
		Query:    `entity(?name, ?kind), ?kind = "component"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetRows()) != len(want) {
		t.Fatalf("query answered %d components, want %d", len(resp.GetRows()), len(want))
	}
	for _, row := range resp.GetRows() {
		name := row.GetCells()[0]
		sheets, reason := CellLocation(resp, row, 0)
		if len(sheets) != 1 || sheets[0] != want[name] || reason != checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED {
			t.Errorf("query cell %s located to %v (%v), want [%s] UNSPECIFIED", name, sheets, reason, want[name])
		}
	}

	var findings []*checkspb.Finding
	for ref := range want {
		findings = append(findings, &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindComponent, Ref: ref}})
	}
	AnnotateSheets(findings, g, nil)
	for _, f := range findings {
		ref := f.GetSubject().GetRef()
		if got := f.GetSheets(); len(got) != 1 || got[0] != want[ref] {
			t.Errorf("finding on %s badged %v, want [%s]", ref, got, want[ref])
		}
	}
}
