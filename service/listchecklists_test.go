package service

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// extendingStore resolves every design to projects/team, which extends projects/shared.
type extendingStore struct {
	ProjectStore
	projects map[string]*webapi.Project
	err      error
}

func (s extendingStore) ResolveDesign(context.Context, artifact.URI) (*webapi.Design, *webapi.Project, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return &webapi.Design{Uri: "mount://m/d"}, s.projects["projects/team"], nil
}

func (s extendingStore) Project(_ context.Context, name string) (*webapi.Project, error) {
	if p, ok := s.projects[name]; ok {
		return p, nil
	}
	return nil, ErrNotFound
}

func checklistNames(r *webapi.ListChecklistsResponse) []string {
	var out []string
	for _, c := range r.GetChecklists() {
		out = append(out, c.GetName())
	}
	return out
}

func listService(store ProjectStore) *ReviewService {
	return NewReviewService(nil, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "", &ProjectResolver{Store: store})
}

// The viewer's picker read the project as its own descriptor declares it and missed what it inherits
// through extends (agni issue 829). ListChecklists answers from the composed config, so an inherited
// checklist is listed, ahead of the project's own as ResolveExtends orders them.
func TestListChecklistsIncludesInheritedOnes(t *testing.T) {
	store := extendingStore{projects: map[string]*webapi.Project{
		"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{
			Extends:    "projects/shared",
			Checklists: []*webapi.NamedChecklist{{Name: "review"}},
		}},
		"projects/shared": {Name: "projects/shared", Config: &webapi.AnalysisConfig{
			Checklists: []*webapi.NamedChecklist{{Name: "house"}},
		}},
	}}
	got, err := listService(store).ListChecklists(context.Background(), &webapi.ListChecklistsRequest{DesignUri: "mount://m/d/board.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetProject() != "projects/team" {
		t.Errorf("project = %q, want projects/team", got.GetProject())
	}
	if names := checklistNames(got); len(names) != 2 || names[0] != "house" || names[1] != "review" {
		t.Errorf("checklists = %v, want house (inherited) then review", names)
	}
}

func TestListChecklistsTellsNoProjectFromNoChecklists(t *testing.T) {
	none, err := listService(declaringStore{design: &webapi.Design{Uri: "mount://m/d"}}).ListChecklists(context.Background(), &webapi.ListChecklistsRequest{DesignUri: "mount://m/d/board.edn"})
	if err != nil || none.GetProject() != "" || len(none.GetChecklists()) != 0 {
		t.Errorf("a design in no project = %v, %v; want no project and no checklists", none, err)
	}
	bare := extendingStore{projects: map[string]*webapi.Project{"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{}}}}
	got, err := listService(bare).ListChecklists(context.Background(), &webapi.ListChecklistsRequest{DesignUri: "mount://m/d/board.edn"})
	if err != nil || got.GetProject() != "projects/team" || len(got.GetChecklists()) != 0 {
		t.Errorf("a project declaring none = %v, %v; want its name and no checklists", got, err)
	}
}

func TestListChecklistsReportsABrokenDescriptor(t *testing.T) {
	broken := extendingStore{err: errors.New("design.yaml: line 3: field chekclists not found")}
	if _, err := listService(broken).ListChecklists(context.Background(), &webapi.ListChecklistsRequest{DesignUri: "mount://m/d/board.edn"}); err == nil {
		t.Error("a descriptor that does not parse must be an error, not an empty list")
	}
	if _, err := listService(broken).ListChecklists(context.Background(), &webapi.ListChecklistsRequest{DesignUri: "not a uri"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a malformed uri = %v, want an invalid argument", err)
	}
}
