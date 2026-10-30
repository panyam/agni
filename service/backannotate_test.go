package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/netgraph"
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

// The same fixture through a check run. Its one finding, i2c-pull-up on I2C_SCL, carries the net's
// id, which the loader stamps and the EDIF geometry's wires never carry, so it was badged with no
// sheet while a query located the net on both pages (agni issue 1230).
func TestACheckRunLocatesAnEDIFNetFinding(t *testing.T) {
	dir := filepath.Join("..", "cmd", "agni", "testdata", "github-issues", "backannotate-locate")
	read := func(name string) *os.File {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	d, err := edif.Read(read("demo_netlist.edn"), "demo_netlist.edn")
	if err != nil {
		t.Fatal(err)
	}
	netgraph.StampNetIDs(d) // as formats.Loader does
	g, err := edif.ReadSchematic(read("demo_schematic.eds"), "demo_schematic.eds")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewCheckService(fakeLoader{design: d, geom: g}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/demo_netlist.edn"),
		Rules:    []string{"i2c-pull-up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetFindings()) != 1 {
		t.Fatalf("got %d findings, want the one on I2C_SCL", len(resp.GetFindings()))
	}
	f := resp.GetFindings()[0]
	if f.GetSubject().GetNetId() == "" {
		t.Fatal("the finding carries no net id, so this does not exercise the id lookup")
	}
	if got := f.GetSheets(); len(got) != 2 || got[0] != "P1" || got[1] != "P2" {
		t.Errorf("I2C_SCL finding badged %v (%v), want [P1 P2]", got, f.GetLocateReason())
	}
}

// A schematic that cannot be read used to leave every subject reading as drawn nowhere, the answer
// for a part the schematic genuinely leaves out, with nothing saying the file had failed (agni issue
// 1230). A run says the schematic was unreadable instead, and still runs.
func TestAnUnreadableSchematicIsNotDrawnNowhere(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "cmd", "agni", "testdata", "github-issues", "backannotate-locate", "demo_netlist.edn"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := edif.Read(f, "demo_netlist.edn")
	if err != nil {
		t.Fatal(err)
	}
	netgraph.StampNetIDs(d)
	l := fakeLoader{design: d, geomErr: errors.New("sexpr: unexpected EOF inside list")}
	unreadable := checkspb.LocateReason_LOCATE_REASON_SCHEMATIC_UNREADABLE

	checks := NewCheckService(l, check.DefaultCatalog(), nil, "", nil, nil)
	run, err := checks.CheckDesign(context.Background(), &webapi.CheckDesignRequest{
		Snapshot: snapOf(t, checks.Snapshots(), "mount://m/demo_netlist.edn"),
		Rules:    []string{"i2c-pull-up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.GetFindings()) != 1 {
		t.Fatalf("got %d findings, want the one on I2C_SCL", len(run.GetFindings()))
	}
	if got := run.GetFindings()[0].GetLocateReason(); got != unreadable {
		t.Errorf("finding reason = %v, want %v", got, unreadable)
	}

	queries := NewQueryService(l, nil, nil)
	resp, err := queries.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Snapshot: snapOf(t, queries.Snapshots(), "mount://m/demo_netlist.edn"),
		Query:    `entity(?name, ?kind), ?kind = "component"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetRows()) != 2 {
		t.Fatalf("query answered %d components, want 2", len(resp.GetRows()))
	}
	for _, row := range resp.GetRows() {
		if _, reason := CellLocation(resp, row, 0); reason != unreadable {
			t.Errorf("cell %s reason = %v, want %v", row.GetCells()[0], reason, unreadable)
		}
	}
}
