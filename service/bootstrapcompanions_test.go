package service_test

import (
	"context"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// declaredNetlist adds to the board repo at version a folder whose design.yaml names an EDIF
// netlist its entry and declares its schematic export a companion. netlist is the entry's contents.
func declaredNetlist(t *testing.T, w *bootWorld, version, netlist string) {
	t.Helper()
	eds, err := os.ReadFile("../examples/common/boards/demo-schematic.eds")
	if err != nil {
		t.Fatal(err)
	}
	files := w.board.files[version]
	files["gw/design.yaml"] = "name: gw\nentry: gw.edn\ncompanions: [gw.eds]\n"
	files["gw/gw.edn"] = netlist
	files["gw/gw.eds"] = string(eds)
}

// twoResistors is an EDIF netlist from the examples.
func twoResistors(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../examples/common/boards/two-resistors.edn")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// gwPlan links the design gw by its entry alone, so the schematic arrives as a companion.
func gwPlan(t *testing.T, prune bool) *service.BootstrapPlan {
	t.Helper()
	p, err := service.ParseBootstrap([]byte("designs:\n  - id: gw\n    link:\n      netlist: board/gw/gw.edn@main\n"))
	if err != nil {
		t.Fatal(err)
	}
	p.Prune = prune
	return p
}

// artifactsOf lists a design's artifact ids, sorted, and the companion_of of each by id.
func artifactsOf(t *testing.T, w *bootWorld, design string) ([]string, map[string]string) {
	t.Helper()
	arts, err := w.catalog.Artifacts(as("alice"), design)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	of := map[string]string{}
	for _, a := range arts {
		_, id, _ := service.ArtifactID(a.GetName())
		ids = append(ids, id)
		of[id] = a.GetSource().GetCompanionOf()
	}
	slices.Sort(ids)
	return ids, of
}

// unstamp writes the artifact id of design again without companion_of. A link by v0.11.5 left each
// companion it followed so.
func unstamp(t *testing.T, w *bootWorld, design, id string) {
	t.Helper()
	ctx := as("alice")
	a, err := w.designs.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: design + "/artifacts/" + id})
	if err != nil {
		t.Fatal(err)
	}
	src := proto.Clone(a.GetSource()).(*webapi.ArtifactSource)
	src.CompanionOf = ""
	if _, err := w.catalog.WriteArtifacts(ctx, design, []service.ArtifactWrite{{ID: id, Files: a.GetFiles(), Source: src}}, w.blobs.source); err != nil {
		t.Fatal(err)
	}
	if _, of := artifactsOf(t, w, design); of[id] != "" {
		t.Fatalf("%s/artifacts/%s after unstamping names %q as its entry; want none", design, id, of[id])
	}
}

// A bootstrap entry that links a declared entry alone gets its companion too. The companion counts
// as named, so a prune keeps it on every apply (agni issue 1890).
func TestBootstrapCountsTheCompanionsALinkFollowedAsNamed(t *testing.T) {
	w := bootstrapWorld(t)
	declaredNetlist(t, w, "c1", twoResistors(t))
	ctx := context.Background()
	for i := range 2 {
		r := w.boot.Apply(ctx, gwPlan(t, true))
		if !r.OK() || len(r.Unnamed) != 0 || len(r.Pruned) != 0 {
			t.Fatalf("apply %d = unnamed %v, pruned %v, failed %v; want nothing unnamed", i+1, r.Unnamed, r.Pruned, failures(r))
		}
		ids, of := artifactsOf(t, w, "designs/gw")
		if !slices.Equal(ids, []string{"netlist", "schematic"}) || of["schematic"] != "netlist" || of["netlist"] != "" {
			t.Fatalf("designs/gw after apply %d holds %v, companion_of %v; want the netlist and the schematic it declares", i+1, ids, of)
		}
		if i == 1 && (len(r.Applied) != 0 || !slices.Equal(r.Unchanged, []string{"designs/gw"})) {
			t.Errorf("second apply = applied %v, unchanged %v; want designs/gw unchanged", r.Applied, r.Unchanged)
		}
	}
}

// An artifact linked by hand beside a followed companion is still reported. A prune deletes it and
// keeps the companion.
func TestBootstrapStillPrunesAnArtifactNoLinkFollowed(t *testing.T) {
	w := bootstrapWorld(t)
	declaredNetlist(t, w, "c1", twoResistors(t))
	ctx := context.Background()
	if r := w.boot.Apply(ctx, gwPlan(t, false)); !r.OK() {
		t.Fatal(failures(r))
	}
	stray := &webapi.LinkArtifactRequest{Name: "designs/gw/artifacts/notes", Source: &webapi.ArtifactSource{Mount: "board", Path: "README.md", Ref: "main"}, AllowMissing: true}
	if _, err := w.designs.LinkArtifact(as("alice"), stray); err != nil {
		t.Fatal(err)
	}
	r := w.boot.Apply(ctx, gwPlan(t, false))
	if !r.OK() || !slices.Equal(r.Unnamed, []string{"designs/gw/artifacts/notes"}) || len(r.Pruned) != 0 {
		t.Fatalf("apply = unnamed %v, pruned %v, failed %v; want the notes reported alone and kept", r.Unnamed, r.Pruned, failures(r))
	}
	r = w.boot.Apply(ctx, gwPlan(t, true))
	if !r.OK() || !slices.Equal(r.Pruned, []string{"designs/gw/artifacts/notes"}) {
		t.Fatalf("apply with prune = pruned %v, failed %v; want the notes alone", r.Pruned, failures(r))
	}
	if ids, _ := artifactsOf(t, w, "designs/gw"); !slices.Equal(ids, []string{"netlist", "schematic"}) {
		t.Errorf("designs/gw after the prune holds %v; want the netlist and its companion", ids)
	}
}

// A companion that a v0.11.5 link followed has no companion_of. The next bootstrap stamps it and
// leaves the entry alone, so a prune keeps it. The apply after that reads nothing.
func TestBootstrapStampsACompanionAnOlderLinkFollowed(t *testing.T) {
	w := bootstrapWorld(t)
	declaredNetlist(t, w, "c1", twoResistors(t))
	ctx := context.Background()
	if r := w.boot.Apply(ctx, gwPlan(t, false)); !r.OK() {
		t.Fatal(failures(r))
	}
	unstamp(t, w, "designs/gw", "schematic")
	r := w.boot.Apply(ctx, gwPlan(t, true))
	if !r.OK() || len(r.Unnamed) != 0 || len(r.Pruned) != 0 || !slices.Equal(r.Applied, []string{"designs/gw"}) {
		t.Fatalf("apply = applied %v, unnamed %v, pruned %v, failed %v; want designs/gw stamped and nothing unnamed", r.Applied, r.Unnamed, r.Pruned, failures(r))
	}
	if ids, of := artifactsOf(t, w, "designs/gw"); !slices.Equal(ids, []string{"netlist", "schematic"}) || of["schematic"] != "netlist" {
		t.Fatalf("designs/gw after the apply holds %v, companion_of %v; want the schematic stamped with the netlist", ids, of)
	}
	fetched := len(w.board.fetches)
	if r := w.boot.Apply(ctx, gwPlan(t, true)); !r.OK() || len(r.Applied) != 0 || len(r.Unnamed) != 0 {
		t.Errorf("apply after the stamp = applied %v, unnamed %v, failed %v; want nothing", r.Applied, r.Unnamed, failures(r))
	}
	if len(w.board.fetches) != fetched {
		t.Errorf("apply after the stamp fetched %v; want no file read again", w.board.fetches[fetched:])
	}
}

// A link or a sync of the entry stamps a companion that an older link followed.
func TestALinkOrSyncOfTheEntryStampsAnOlderCompanion(t *testing.T) {
	w := bootstrapWorld(t)
	declaredNetlist(t, w, "c1", twoResistors(t))
	ctx := context.Background()
	if r := w.boot.Apply(ctx, gwPlan(t, false)); !r.OK() {
		t.Fatal(failures(r))
	}
	unstamp(t, w, "designs/gw", "schematic")
	entry := &webapi.LinkArtifactRequest{Name: "designs/gw/artifacts/netlist", Source: &webapi.ArtifactSource{Mount: "board", Path: "gw/gw.edn", Ref: "main"}}
	if _, err := w.designs.LinkArtifact(as("alice"), entry); err != nil {
		t.Fatal(err)
	}
	if _, of := artifactsOf(t, w, "designs/gw"); of["schematic"] != "netlist" {
		t.Errorf("schematic after a link of the entry names %q as its entry; want netlist", of["schematic"])
	}

	unstamp(t, w, "designs/gw", "schematic")
	w.board.files["c2"] = maps.Clone(w.board.files["c1"])
	declaredNetlist(t, w, "c2", strings.Replace(twoResistors(t), "R1", "R9", 1))
	w.board.refs["main"] = "c2"
	resp, err := w.designs.SyncDesign(as("alice"), &webapi.SyncDesignRequest{Name: "designs/gw"})
	if err != nil {
		t.Fatal(err)
	}
	moved := slices.ContainsFunc(resp.GetArtifacts(), func(a *webapi.ArtifactSync) bool {
		return strings.HasSuffix(a.GetArtifact(), "/netlist") && a.GetChanged()
	})
	if !moved {
		t.Fatalf("sync = %v; want the netlist changed, or the sync never read the entry's design.yaml", resp.GetArtifacts())
	}
	ids, of := artifactsOf(t, w, "designs/gw")
	if !slices.Equal(ids, []string{"netlist", "schematic"}) || of["schematic"] != "netlist" {
		t.Errorf("designs/gw after a sync holds %v, companion_of %v; want the schematic stamped with the netlist", ids, of)
	}
	if r := w.boot.Apply(ctx, gwPlan(t, true)); !r.OK() || len(r.Unnamed) != 0 || len(r.Pruned) != 0 {
		t.Errorf("apply after the sync = unnamed %v, pruned %v, failed %v; want nothing unnamed", r.Unnamed, r.Pruned, failures(r))
	}
}
