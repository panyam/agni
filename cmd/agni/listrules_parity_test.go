package main

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/agni/mounts"
)

// The viewer selects the rules ListRules calls available and runs those (agni issue 950). A listing
// that calls a rule unavailable when a run would run it hides that rule's findings, which is how the
// gateway showed 25 findings where CheckDesign returns 28 (agni issue 940). A listing that calls a
// rule available when every run skips it offers a checkbox that answers nothing. So for each fixture,
// on the server and on the browser engine alike, ListRules' available set must be the set of rules a
// whole CheckDesign run did not report in `skipped`.
func TestListRulesAvailableIsWhatARunRuns(t *testing.T) {
	ms := []struct{ name, dir string }{
		{"tut", filepath.Join("..", "..", "examples", "tutorial-project")},
		{"rev", filepath.Join("testdata", "review")},
		{"com", filepath.Join("..", "..", "examples", "common", "designs")},
	}
	var hostMounts []mounts.Mount
	var memMounts []fshost.Mount
	for _, m := range ms {
		root, err := filepath.Abs(m.dir)
		if err != nil {
			t.Fatal(err)
		}
		hostMounts = append(hostMounts, mounts.Mount{Name: m.name, Root: root})
		memMounts = append(memMounts, fshost.Mount{Name: m.name, FS: memCopy(t, root)})
	}
	eng, err := wasmengine.New(memMounts...)
	if err != nil {
		t.Fatal(err)
	}
	hosts := []struct {
		name string
		c    parityClients
	}{
		{"server", newParityClients(t, servedParityMux(t, hostMounts))},
		{"browser engine", newParityClients(t, eng.Handler)},
	}
	// runs and skips name a rule each fixture's run must run and one it must skip, so both directions
	// of the comparison are exercised on every fixture. The gateway's supply-exceeds-abs-max is a
	// datasheet-tier rule its project's params/ ungates, the rule shape agni issue 940 hid, and
	// track-width is gated on the board tier.
	designs := []struct{ name, uri, runs, skips string }{
		{"tutorial gateway in its project", "mount://tut/designs/gateway", "supply-exceeds-abs-max", "unconnected-pin"},
		{"bare KiCad schematic", "mount://com/dangling-wire.kicad_sch", "wire-no-junction", "track-width"},
		{"bare KiCad board", "mount://com/demo-board.kicad_pcb", "track-width", "wire-no-junction"},
		{"EDIF netlist", "mount://rev/companion-demo.edn", "single-pin-net", "track-width"},
	}
	for _, h := range hosts {
		for _, d := range designs {
			t.Run(h.name+"/"+d.name, func(t *testing.T) {
				listedAgreesWithRun(t, h.c, d.uri, d.runs, d.skips)
			})
		}
	}
}

// listedAgreesWithRun ingests design, lists its rules, runs the whole catalog, and requires the
// listing's available set to be exactly the run's runnable set. runs and skips are a rule the run must
// run and one it must skip.
func listedAgreesWithRun(t *testing.T, c parityClients, design, runs, skips string) {
	t.Helper()
	ctx := context.Background()
	s, err := c.snap.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: design}}))
	if err != nil {
		t.Fatalf("Ingest %s: %v", design, err)
	}
	snap := s.Msg.GetName()
	listed, err := c.check.ListRules(ctx, connect.NewRequest(&webapi.ListRulesRequest{Snapshot: snap}))
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	run, err := c.check.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snap}))
	if err != nil {
		t.Fatalf("CheckDesign: %v", err)
	}
	skipped := map[string]string{}
	for _, sk := range run.Msg.GetSkipped() {
		skipped[sk.GetName()] = sk.GetReason()
	}
	if _, skip := skipped[runs]; skip {
		t.Errorf("the run skipped %s, which this fixture exists to run", runs)
	}
	if _, skip := skipped[skips]; !skip {
		t.Errorf("the run ran %s, which this fixture exists to skip", skips)
	}
	var hidden, promised []string
	ran, seen := 0, 0
	for _, r := range listed.Msg.GetRules() {
		if r.GetName() == runs || r.GetName() == skips {
			seen++
		}
		why, skip := skipped[r.GetName()]
		if !skip {
			ran++
		}
		switch {
		case !r.GetAvailable() && !skip:
			hidden = append(hidden, r.GetName()+" (listed: "+r.GetUnavailableReason()+")")
		case r.GetAvailable() && skip:
			promised = append(promised, r.GetName()+" (run: "+why+")")
		}
	}
	// A listing of nothing, or a run that ran nothing, agrees with anything.
	if len(listed.Msg.GetRules()) < 20 || ran < 10 {
		t.Fatalf("ListRules listed %d rules and the run ran %d, too few for the comparison to mean anything", len(listed.Msg.GetRules()), ran)
	}
	if seen != 2 {
		t.Fatalf("ListRules lists %d of %s and %s, so the comparison does not reach the rules this fixture is for", seen, runs, skips)
	}
	sort.Strings(hidden)
	sort.Strings(promised)
	for _, h := range hidden {
		t.Errorf("ListRules calls %s unavailable, and a run of the whole catalog runs it, so a viewer selecting the available rules hides its findings", h)
	}
	for _, p := range promised {
		t.Errorf("ListRules calls %s available, and a run of the whole catalog skips it", p)
	}
}
