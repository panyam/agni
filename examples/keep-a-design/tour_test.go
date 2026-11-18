package main

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

func TestTheTourKeepsTwoRevisionsAndRefusesTheMountedDesign(t *testing.T) {
	ctx := context.Background()
	tr, err := newTour(t.TempDir(), mounted)
	if err != nil {
		t.Fatal(err)
	}
	if _, asked, err := tr.keep(ctx, "mixer-rev-a", "Mixer rev A", "../common/designs/rev-a.edn"); err != nil || asked != 1 {
		t.Fatalf("keep rev A = asked %d, %v; want the netlist's contents asked for", asked, err)
	}
	if _, asked, err := tr.keep(ctx, "mixer-rev-b", "Mixer rev B", "../common/designs/rev-b.edn"); err != nil || asked != 1 {
		t.Fatalf("keep rev B = asked %d, %v; want its netlist's contents asked for", asked, err)
	}
	before, after, err := tr.keepPack(ctx)
	if err != nil || before.GetHash() == "" || before.GetHash() == after.GetHash() {
		t.Errorf("keepPack = %v, %v, %v; want a hash that moves when the rail pattern changes", before, after, err)
	}
	if _, err := tr.projects.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "lab", Project: &webapi.Project{Designs: []string{"designs/mixer", "designs/mixer-rev-a", "designs/mixer-rev-b"}}}); err != nil {
		t.Fatal(err)
	}
	list, err := tr.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{Filter: `project = "projects/lab"`})
	if err != nil || len(list.GetDesigns()) != 3 {
		t.Errorf("projects/lab = %v, %v; want three designs", list.GetDesigns(), err)
	}
	if err := tr.designs.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: "designs/mixer"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteDesign of the mounted mixer = %v, want ErrFailedPrecondition", err)
	}
	if err := tr.projects.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: "projects/lab"}); err != nil {
		t.Fatal(err)
	}
	if all, err := tr.designs.ListDesigns(ctx, &webapi.ListDesignsRequest{}); err != nil || len(all.GetDesigns()) != 3 {
		t.Errorf("designs after deleting the project = %v, %v; want all three", all.GetDesigns(), err)
	}
}
