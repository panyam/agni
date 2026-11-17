package projects

import (
	"context"
	"errors"
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
	u := uploadOf(map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	if _, err := cat.CreateDesign(ctx, "mul", u.files, u.source); err != nil {
		t.Fatal(err)
	}
	all, err := cat.Designs(ctx)
	if err != nil || len(all) != 2 || all[0].GetName() != "designs/g" || all[1].GetName() != "designs/mul" {
		t.Fatalf("catalog designs = %v, %v; want designs/g then designs/mul", all, err)
	}
	if all[0].GetConfig().GetExtends() != "projects/tut" || all[0].GetEntryUri() != "mount://tut/designs/g/g.edn" {
		t.Errorf("mounted design g = extends %q at %s; want projects/tut at mount://tut/designs/g/g.edn", all[0].GetConfig().GetExtends(), all[0].GetEntryUri())
	}
	if d, err := cat.Design(ctx, "designs/mul"); err != nil || d.GetEntryUri() != "mount://store/mul/a.edn" {
		t.Errorf("catalog design mul = %v, %v; want it read from the store", d, err)
	}
	if _, err := cat.Design(ctx, "projects/tut/designs/g"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a nested name = %v, want ErrInvalidArgument", err)
	}
}

func TestACatalogRefusesWritesToAMountedDesign(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	cat := service.NewDesignCatalog(ds, NewMountedDesigns(tutorial()))
	u := uploadOf(map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	if _, err := cat.ReplaceDesign(ctx, "designs/g", u.files, u.source); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("ReplaceDesign on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if err := cat.DeleteDesign(ctx, "designs/g"); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign on a mounted design = %v, want ErrFailedPrecondition", err)
	}
	if _, err := cat.CreateDesign(ctx, "g", u.files, u.source); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("CreateDesign over a mounted design's id = %v, want ErrAlreadyExists", err)
	}
	if err := cat.DeleteDesign(ctx, "designs/none"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("DeleteDesign of a missing design = %v, want ErrNotFound", err)
	}
}

func TestACatalogRefusesAnIdTwoStoresHold(t *testing.T) {
	ctx := context.Background()
	ds, _ := dirStore(t)
	mustDesign(t, ds, "g", map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	cat := service.NewDesignCatalog(ds, NewMountedDesigns(tutorial()))
	if _, err := cat.Design(ctx, "designs/g"); err == nil {
		t.Error("Design of an id two stores hold answered, want an error")
	}
	if _, err := cat.Designs(ctx); err == nil {
		t.Error("Designs with an id two stores hold answered, want an error")
	}
}

func TestACatalogWithNoStoreRefusesWrites(t *testing.T) {
	ctx := context.Background()
	cat := service.NewDesignCatalog(nil, NewMountedDesigns(tutorial()))
	u := uploadOf(map[string]string{"design.yaml": "entry: a.edn\n", "a.edn": "(edif a)"})
	if _, err := cat.CreateDesign(ctx, "mul", u.files, u.source); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("CreateDesign with no store = %v, want ErrFailedPrecondition", err)
	}
	if all, err := cat.Designs(ctx); err != nil || len(all) != 1 {
		t.Errorf("Designs with no store = %v, %v; want the mounted design", all, err)
	}
}
