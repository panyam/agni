package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// extendingStore resolves every design to projects/team, which extends projects/shared.
type extendingStore struct {
	ProjectStore
	projects map[string]*webapi.Project
	err      error
}

func (s extendingStore) ResolveDesign(context.Context, mounts.URI) (*webapi.Design, *webapi.Project, error) {
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
	return NewReviewService(dirReviewLoader{}, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "", &ProjectResolver{Store: store})
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
	svc := listService(store)
	got, err := svc.ListChecklists(context.Background(), &webapi.ListChecklistsRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/d/board.edn")})
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
	svc := listService(declaringStore{design: &webapi.Design{Uri: "mount://m/d"}})
	none, err := svc.ListChecklists(context.Background(), &webapi.ListChecklistsRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/d/board.edn")})
	if err != nil || none.GetProject() != "" || len(none.GetChecklists()) != 0 {
		t.Errorf("a design in no project = %v, %v; want no project and no checklists", none, err)
	}
	bare := extendingStore{projects: map[string]*webapi.Project{"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{}}}}
	svc = listService(bare)
	got, err := svc.ListChecklists(context.Background(), &webapi.ListChecklistsRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/d/board.edn")})
	if err != nil || got.GetProject() != "projects/team" || len(got.GetChecklists()) != 0 {
		t.Errorf("a project declaring none = %v, %v; want its name and no checklists", got, err)
	}
}

func TestListChecklistsReportsABrokenDescriptor(t *testing.T) {
	broken := extendingStore{err: errors.New("design.yaml: line 3: field chekclists not found")}
	ingest := func(uri string) error {
		_, err := listService(broken).Snapshots().Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: uri}})
		return err
	}
	if err := ingest("mount://m/d/board.edn"); err == nil {
		t.Error("a descriptor that does not parse must be an error, not an empty list")
	}
	if err := ingest("not a uri"); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a malformed uri = %v, want an invalid argument", err)
	}
}

// fixedPacks is a PackLookup over packs held by name.
type fixedPacks map[string]*webapi.Pack

func (p fixedPacks) GetPack(_ context.Context, req *webapi.GetPackRequest) (*webapi.Pack, error) {
	if pk, ok := p[req.GetName()]; ok {
		return pk, nil
	}
	return nil, ErrNotFound
}

// A page reads a design under packs, as /designs/jetson/view?pack=house does. Its Review panel lists
// the packs' checklists first, with a later pack replacing an earlier one's checklist of the same
// name, as `agni review --pack` picks them. The project's own follow, less any a pack replaces
// (agni issue 1851).
func TestListChecklistsOffersThePacksTheSnapshotReadUnder(t *testing.T) {
	store := extendingStore{projects: map[string]*webapi.Project{
		"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{
			Checklists: []*webapi.NamedChecklist{{Name: "bringup"}, {Name: "own"}},
		}},
	}}
	packs := fixedPacks{
		"packs/house": {Name: "packs/house", Config: &webapi.AnalysisConfig{Checklists: []*webapi.NamedChecklist{
			{Name: "release", Manifest: &checkspb.ReviewManifest{Name: "house release"}}, {Name: "bringup"},
		}}},
		"packs/extra": {Name: "packs/extra", Config: &webapi.AnalysisConfig{Checklists: []*webapi.NamedChecklist{
			{Name: "release", Manifest: &checkspb.ReviewManifest{Name: "extra release"}}, {Name: "emc"},
		}}},
	}
	svc := NewReviewService(dirReviewLoader{}, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "", &ProjectResolver{Store: store, Packs: packs})
	snap := ingestWith(t, svc.Snapshots(), &webapi.Snapshot{Uri: "mount://m/d/board.edn", Packs: []string{"packs/house", "packs/extra"}})
	got, err := svc.ListChecklists(context.Background(), &webapi.ListChecklistsRequest{Snapshot: snap})
	if err != nil {
		t.Fatal(err)
	}
	if names, want := checklistNames(got), []string{"release", "bringup", "emc", "own"}; !slices.Equal(names, want) {
		t.Errorf("checklists = %v, want %v", names, want)
	}
	if m := got.GetChecklists()[0].GetManifest().GetName(); m != "extra release" {
		t.Errorf("the default checklist is %q, want the later pack's release", m)
	}

	plain, err := svc.ListChecklists(context.Background(), &webapi.ListChecklistsRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/d/board.edn")})
	if err != nil {
		t.Fatal(err)
	}
	if names, want := checklistNames(plain), []string{"bringup", "own"}; !slices.Equal(names, want) {
		t.Errorf("with no packs, checklists = %v, want the project's own %v", names, want)
	}
}
