package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// setResolver answers a config naming query-set directories with one set per directory, named for
// it, and records each config it was asked to resolve.
type setResolver struct {
	asked []*webapi.AnalysisConfig
}

func (r *setResolver) ResolveConfig(_ context.Context, cfg *webapi.AnalysisConfig, _ string) (ResolvedConfig, error) {
	r.asked = append(r.asked, cfg)
	var out ResolvedConfig
	for _, uri := range cfg.GetQuerySetUris() {
		name := uri[strings.LastIndex(uri, "/")+1:]
		out.QuerySets = append(out.QuerySets, NamedQuerySet{Name: name, Source: uri + "/" + name + ".yaml", Set: query.QuerySet{Queries: []query.NamedQuery{{Name: "q", Query: "net.rail(?n) => ?n"}}}})
	}
	return out, nil
}

func setNames(r *webapi.ListQuerySetsResponse) string {
	var out []string
	for _, s := range r.GetQuerySets() {
		out = append(out, s.GetName())
	}
	return strings.Join(out, ",")
}

// A project's sets are listed with the ones it inherits through extends first, as its checklists
// are, and only the query-set tier is resolved, so a listing reads no profile, corpus or library.
func TestListQuerySetsIncludesInheritedOnesAndResolvesNothingElse(t *testing.T) {
	store := extendingStore{projects: map[string]*webapi.Project{
		"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{
			Extends:      "projects/shared",
			QuerySetUris: []string{"mount://m/team"},
			ProfileUris:  []string{"mount://m/profiles"},
		}},
		"projects/shared": {Name: "projects/shared", Config: &webapi.AnalysisConfig{
			QuerySetUris: []string{"mount://m/shared"},
			LibraryUris:  []string{"mount://m/lib"},
		}},
	}}
	res := &setResolver{}
	svc := NewQueryService(nil, nil, &ProjectResolver{Store: store, Config: res})
	got, err := svc.ListQuerySets(context.Background(), &webapi.ListQuerySetsRequest{DesignUri: "mount://m/d/board.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetProject() != "projects/team" || setNames(got) != "shared,team" {
		t.Errorf("got project %q sets %q, want projects/team with shared (inherited) then team", got.GetProject(), setNames(got))
	}
	if got.GetQuerySets()[0].GetSet().GetQueries()[0].GetQuery() != "net.rail(?n) => ?n" || got.GetQuerySets()[0].GetSource() != "mount://m/shared/shared.yaml" {
		t.Errorf("first set = %v, want its query and its file", got.GetQuerySets()[0])
	}
	if len(res.asked) != 1 || len(res.asked[0].GetProfileUris())+len(res.asked[0].GetLibraryUris()) != 0 {
		t.Errorf("resolved %v, want one config naming the query sets alone", res.asked)
	}
}

func TestListQuerySetsTellsNoProjectFromNoSets(t *testing.T) {
	res := &setResolver{}
	none, err := NewQueryService(nil, nil, &ProjectResolver{Store: declaringStore{design: &webapi.Design{Uri: "mount://m/d"}}, Config: res}).
		ListQuerySets(context.Background(), &webapi.ListQuerySetsRequest{DesignUri: "mount://m/d/board.edn"})
	if err != nil || none.GetProject() != "" || len(none.GetQuerySets()) != 0 {
		t.Errorf("a design in no project = %v, %v; want no project and no sets", none, err)
	}
	bare := extendingStore{projects: map[string]*webapi.Project{"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{}}}}
	got, err := NewQueryService(nil, nil, &ProjectResolver{Store: bare, Config: res}).
		ListQuerySets(context.Background(), &webapi.ListQuerySetsRequest{DesignUri: "mount://m/d/board.edn"})
	if err != nil || got.GetProject() != "projects/team" || len(got.GetQuerySets()) != 0 {
		t.Errorf("a project keeping none = %v, %v; want its name and no sets", got, err)
	}
	if len(res.asked) != 0 {
		t.Errorf("a project keeping no sets resolved %v, want nothing read", res.asked)
	}
}

// A host that cannot resolve project config refuses a project naming sets, rather than listing none.
func TestListQuerySetsRefusesWhatItCannotRead(t *testing.T) {
	store := extendingStore{projects: map[string]*webapi.Project{"projects/team": {Name: "projects/team", Config: &webapi.AnalysisConfig{QuerySetUris: []string{"mount://m/q"}}}}}
	_, err := NewQueryService(nil, nil, &ProjectResolver{Store: store}).ListQuerySets(context.Background(), &webapi.ListQuerySetsRequest{DesignUri: "mount://m/d/board.edn"})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("err = %v, want an invalid argument", err)
	}
	broken := extendingStore{err: errors.New("project.yaml: line 3: field qeuries not found")}
	if _, err := NewQueryService(nil, nil, &ProjectResolver{Store: broken}).ListQuerySets(context.Background(), &webapi.ListQuerySetsRequest{DesignUri: "mount://m/d/board.edn"}); err == nil {
		t.Error("a descriptor that does not parse must be an error, not an empty list")
	}
}

// ParseQuerySet checks a set's text as `agni query --set` does, so a file a reader picks in the
// viewer is refused for the same reasons.
func TestParseQuerySetChecksWhatTheCLIChecks(t *testing.T) {
	svc := NewQueryService(nil, nil, nil)
	got, err := svc.ParseQuerySet(context.Background(), &webapi.ParseQuerySetRequest{Text: "title: T\nqueries:\n  - name: Rails\n    query: net.rail(?n) => ?n\n    bind: {n: GND}\n"})
	if err != nil {
		t.Fatal(err)
	}
	q := got.GetSet().GetQueries()
	if got.GetSet().GetTitle() != "T" || len(q) != 1 || q[0].GetName() != "Rails" || q[0].GetBindings()["n"].GetText() != "GND" {
		t.Errorf("set = %v, want its title, its one query and its binding", got.GetSet())
	}
	for name, text := range map[string]string{
		"unknown key":   "queries:\n  - name: A\n    qeury: x\n",
		"no queries":    "title: T\n",
		"repeated name": "queries:\n  - {name: A, query: \"net.rail(?n) => ?n\"}\n  - {name: A, query: \"net.rail(?n) => ?n\"}\n",
	} {
		if _, err := svc.ParseQuerySet(context.Background(), &webapi.ParseQuerySetRequest{Text: text}); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want an invalid argument", name, err)
		}
	}
}
