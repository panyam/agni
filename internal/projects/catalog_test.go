package projects

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/service"
)

// tutorial is a mounted folder holding a project and one design that extends it.
func tutorial() Tree {
	return Tree{Mount: "tut", FS: fstest.MapFS{
		"project.yaml":          {Data: []byte("name: tut\n")},
		"designs/g/design.yaml": {Data: []byte("name: g\nextends: ../../project.yaml\nentry: g.edn\n")},
		"designs/g/g.edn":       {Data: []byte("(edif g)")},
	}}
}

func TestACatalogListsStoredAndMountedDesignsByOneName(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	cat := service.NewDesignCatalog(ds, NewMountedDesigns(tutorial()))
	if _, err := cat.CreateDesign(ctx, "mul", service.DesignRecord{}); err != nil {
		t.Fatal(err)
	}
	if _, err := put(ctx, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}}); err != nil {
		t.Fatal(err)
	}
	all, unreachable, err := cat.Designs(ctx)
	if err != nil || len(unreachable) != 0 || len(all) != 2 || all[0].GetName() != "designs/g" || all[1].GetName() != "designs/mul" {
		t.Fatalf("catalog designs = %v, %v; want designs/g then designs/mul", all, err)
	}
	if all[0].GetConfig().GetExtends() != "projects/tut" || all[0].GetEntryUri() != "mount://tut/designs/g/g.edn" {
		t.Errorf("mounted design g = extends %q at %s; want projects/tut at mount://tut/designs/g/g.edn", all[0].GetConfig().GetExtends(), all[0].GetEntryUri())
	}
	if d, err := cat.Design(ctx, "designs/mul"); err != nil || d.GetEntryUri() != inView(t, d, "netlist/a.edn") {
		t.Errorf("catalog design mul = %v, %v; want it read from the store", d, err)
	}
	if !all[0].GetReadOnly() || all[1].GetReadOnly() {
		t.Errorf("read_only = g %v, mul %v; want the mounted g read-only and the stored mul not (agni issue 1697)", all[0].GetReadOnly(), all[1].GetReadOnly())
	}
	if g, err := cat.Design(ctx, "designs/g"); err != nil || !g.GetReadOnly() {
		t.Errorf("catalog design g = %v, %v; want it read-only", g, err)
	}
	if _, err := cat.Design(ctx, "projects/tut/designs/g"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a nested name = %v, want ErrInvalidArgument", err)
	}
}

func TestACatalogRefusesWritesToAMountedDesign(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	cat := service.NewDesignCatalog(ds, NewMountedDesigns(tutorial()))
	u := uploadOf(map[string]string{"a.edn": "(edif a)"})
	if _, err := cat.WriteArtifacts(ctx, "designs/g", []service.ArtifactWrite{{ID: "netlist", Files: u.files, AllowMissing: true}}, u.source); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("WriteArtifacts on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := cat.UpdateDesign(ctx, "designs/g", func(*service.DesignRecord) {}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("UpdateDesign on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if err := cat.DeleteArtifact(ctx, "designs/g/artifacts/netlist"); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteArtifact on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if as, err := cat.Artifacts(ctx, "designs/g"); err != nil || len(as) != 0 {
		t.Errorf("Artifacts of a mounted design = %v, %v; want none", as, err)
	}
	if err := cat.DeleteDesign(ctx, "designs/g"); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := cat.CreateDesign(ctx, "g", service.DesignRecord{}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("CreateDesign over a mounted design's id = %v, want ErrAlreadyExists", err)
	}
	if err := cat.DeleteDesign(ctx, "designs/none"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("DeleteDesign of a missing design = %v, want ErrNotFound", err)
	}
}

// An id two places hold is unreachable, and the rest of the listing still answers (agni issue 1694).
func TestACatalogLeavesOutAnIdTwoStoresHold(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	mustDesign(t, ds, "g", arts{"netlist": {"a.edn": "(edif a)"}})
	mustDesign(t, ds, "mul", arts{"netlist": {"a.edn": "(edif a)"}})
	cat := service.NewDesignCatalog(ds, NewMountedDesigns(tutorial()))
	if _, err := cat.Design(ctx, "designs/g"); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("Design of an id two stores hold = %v, want ErrFailedPrecondition", err)
	}
	all, unreachable, err := cat.Designs(ctx)
	if err != nil || len(all) != 1 || all[0].GetName() != "designs/mul" || len(unreachable) != 1 || unreachable[0] != "designs/g" {
		t.Errorf("Designs = %v, unreachable %v, %v; want designs/mul listed and designs/g unreachable", all, unreachable, err)
	}
}

// Two mounted folders declaring one id leave it unreachable, and Design names both folders.
func TestTwoMountedFoldersDeclaringOneIdLeaveItUnreachable(t *testing.T) {
	ctx := context.Background()
	other := Tree{Mount: "copy", FS: fstest.MapFS{
		"g/design.yaml": {Data: []byte("name: g\nentry: g.edn\n")},
		"g/g.edn":       {Data: []byte("(edif g)")},
		"h/design.yaml": {Data: []byte("name: h\nentry: h.edn\n")},
		"h/h.edn":       {Data: []byte("(edif h)")},
	}}
	cat := service.NewDesignCatalog(nil, NewMountedDesigns(tutorial(), other))
	_, err := cat.Design(ctx, "designs/g")
	if !errors.Is(err, service.ErrFailedPrecondition) || !strings.Contains(err.Error(), "mount://tut/designs/g") || !strings.Contains(err.Error(), "mount://copy/g") {
		t.Errorf("Design of a duplicated id = %v, want ErrFailedPrecondition naming both folders", err)
	}
	all, unreachable, err := cat.Designs(ctx)
	if err != nil || len(all) != 1 || all[0].GetName() != "designs/h" || len(unreachable) != 1 || unreachable[0] != "designs/g" {
		t.Errorf("Designs = %v, unreachable %v, %v; want designs/h listed and designs/g unreachable", all, unreachable, err)
	}
}

func TestACatalogWithNoStoreRefusesWrites(t *testing.T) {
	ctx := context.Background()
	cat := service.NewDesignCatalog(nil, NewMountedDesigns(tutorial()))
	if _, err := cat.CreateDesign(ctx, "mul", service.DesignRecord{}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("CreateDesign with no store = %v, want ErrFailedPrecondition", err)
	}
	if all, _, err := cat.Designs(ctx); err != nil || len(all) != 1 {
		t.Errorf("Designs with no store = %v, %v; want the mounted design", all, err)
	}
}
