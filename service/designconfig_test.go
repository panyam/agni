package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// layeredStore resolves every design to one design with config of its own, in projects/team.
type layeredStore struct {
	ProjectStore
	design  *webapi.Design
	project *webapi.Project
}

func (s layeredStore) ResolveDesign(context.Context, artifact.URI) (*webapi.Design, *webapi.Project, error) {
	return s.design, s.project, nil
}

func namedChecklist(name, title string) *webapi.NamedChecklist {
	return &webapi.NamedChecklist{Name: name, Manifest: &checkspb.ReviewManifest{Name: title}}
}

// A design declares any section its project can (agni issue 1136), and its own layer goes on top: its
// convention replaces its project's, as a nearer link of an extends chain does, and a checklist of a
// name the project declares replaces that one in place while a new name is added after.
func TestADesignLayersItsOwnConfigOverItsProject(t *testing.T) {
	store := layeredStore{
		project: &webapi.Project{Name: "projects/team", Config: &webapi.AnalysisConfig{
			Conventions: &configpb.NamingConvention{Name: "team"},
			Checklists:  []*webapi.NamedChecklist{namedChecklist("review", "team review"), namedChecklist("house", "house")},
		}},
		design: &webapi.Design{Name: "projects/team/designs/legacy", Uri: "mount://m/d", Config: &webapi.AnalysisConfig{
			Conventions: &configpb.NamingConvention{Name: "legacy"},
			Checklists:  []*webapi.NamedChecklist{namedChecklist("review", "legacy review"), namedChecklist("extra", "extra")},
		}},
	}
	r := &ProjectResolver{Store: store}
	ctx := context.Background()
	lists, project, err := r.Checklists(ctx, artifact.URI{Mount: "m", Path: "d"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range lists {
		got = append(got, c.GetName()+"="+c.GetManifest().GetName())
	}
	if project != "projects/team" || len(got) != 3 || got[0] != "review=legacy review" || got[1] != "house=house" || got[2] != "extra=extra" {
		t.Errorf("checklists = %v in %q, want the design's review in place, the project's house, then the design's extra", got, project)
	}
	ov, err := OverlayFor(ctx, nil, store, store.project, store.design, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if ov.conventionName != "legacy" {
		t.Errorf("convention = %q, want the design's own, legacy", ov.conventionName)
	}
	noOwn := layeredStore{project: store.project, design: &webapi.Design{Uri: "mount://m/d", Config: &webapi.AnalysisConfig{}}}
	if ov, err := OverlayFor(ctx, nil, noOwn, noOwn.project, noOwn.design, nil, ""); err != nil || ov.conventionName != "team" {
		t.Errorf("positive control: a design declaring none takes its project's convention, got %q, %v", ov.conventionName, err)
	}
}

// A design in no project still has the checklists it declares itself.
func TestADesignInNoProjectListsItsOwnChecklists(t *testing.T) {
	r := &ProjectResolver{Store: declaringStore{design: &webapi.Design{Uri: "mount://m/d", Config: &webapi.AnalysisConfig{
		Checklists: []*webapi.NamedChecklist{namedChecklist("bench", "bench")},
	}}}}
	lists, project, err := r.Checklists(context.Background(), artifact.URI{Mount: "m", Path: "d"})
	if err != nil || project != "" || len(lists) != 1 || lists[0].GetName() != "bench" {
		t.Errorf("checklists = %v in %q, %v; want the design's bench and no project", lists, project, err)
	}
}
