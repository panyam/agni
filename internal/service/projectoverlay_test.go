package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/artifact"
)

// twoProjects is a deployment serving two teams' boards, each with a naming convention that names
// the OTHER team's rails. It is the shape the misfire was reported on: both configs are correct in
// isolation and aimed at the wrong design.
type twoProjects struct {
	byURI map[string]*webapi.Project
}

func (t *twoProjects) Project(context.Context, string) (*webapi.Project, error) { return nil, nil }
func (t *twoProjects) Projects(context.Context) ([]*webapi.Project, error)      { return nil, nil }
func (t *twoProjects) Design(context.Context, string) (*webapi.Design, error)   { return nil, nil }
func (t *twoProjects) Designs(context.Context, string) ([]*webapi.Design, error) {
	return nil, nil
}

func (t *twoProjects) ResolveDesign(_ context.Context, uri artifact.URI) (*webapi.Design, *webapi.Project, error) {
	p, ok := t.byURI[uri.String()]
	if !ok {
		return nil, nil, nil
	}
	return &webapi.Design{Name: "projects/x/designs/y"}, p, nil
}

func conventionNaming(name, pattern string) *webapi.NamingConvention {
	return &webapi.NamingConvention{
		Name:    name,
		Lexicon: &webapi.NamingLexicon{Rail: &webapi.VocabPatterns{Patterns: []string{pattern}}},
		Rules:   []*webapi.NamingRule{{Name: "net-naming", Severity: "warning", Allow: []string{"^" + name}}},
	}
}

func resolverFor(byURI map[string]*webapi.Project) *ProjectResolver {
	return &ProjectResolver{Store: &twoProjects{byURI: byURI}}
}

// TestConfigDoesNotCrossProjects is the bug this change exists to close. Two designs on one server,
// each in its own project: neither may be composed against the other's rules or vocabulary.
//
// It asserts on the RULE SOURCE names because that is what a finding is attributed to. A rule that
// runs under the wrong project's name is a finding a team cannot act on, and the failure is silent:
// nothing in a findings list says which config produced it.
func TestConfigDoesNotCrossProjects(t *testing.T) {
	acme := &webapi.Project{Name: "projects/acme", Conventions: conventionNaming("acme", "^ACME_")}
	globex := &webapi.Project{Name: "projects/globex", Conventions: conventionNaming("globex", "^GBX_")}
	r := resolverFor(map[string]*webapi.Project{
		"mount://m/acme/board.edn":   acme,
		"mount://m/globex/board.edn": globex,
	})
	ctx := context.Background()

	for _, c := range []struct{ uri, want, absent string }{
		{"mount://m/acme/board.edn", "acme", "globex"},
		{"mount://m/globex/board.edn", "globex", "acme"},
	} {
		u, err := artifact.Parse(c.uri)
		if err != nil {
			t.Fatal(err)
		}
		ov, err := r.Overlay(ctx, u, nil, Overlay{}, "")
		if err != nil {
			t.Fatalf("%s: %v", c.uri, err)
		}
		names := sourceNames(ov)
		if !contains(names, c.want) {
			t.Errorf("%s composed %v, want its own project %q", c.uri, names, c.want)
		}
		if contains(names, c.absent) {
			t.Errorf("%s composed %v, which includes another project's rules (%q)", c.uri, names, c.absent)
		}
	}
}

// TestNoProjectGetsNoProjectConfig is the structural half of the guarantee: a design that resolves
// to nothing cannot be checked against anyone's rules, because there is no project to take them
// from. It is a property of the shape rather than a flag someone remembers to leave off.
func TestNoProjectGetsNoProjectConfig(t *testing.T) {
	r := resolverFor(map[string]*webapi.Project{
		"mount://m/acme/board.edn": {Name: "projects/acme", Conventions: conventionNaming("acme", "^ACME_")},
	})
	u, err := artifact.Parse("mount://m/loose/board.edn")
	if err != nil {
		t.Fatal(err)
	}
	ov, err := r.Overlay(context.Background(), u, nil, Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if names := sourceNames(ov); len(names) != 0 {
		t.Fatalf("a design in no project composed %v, want nothing", names)
	}
	if ov.Lexicon != nil {
		t.Error("a design in no project must not inherit a vocabulary")
	}
}

// TestFallbackAppliesOnlyWithoutAProject keeps every existing single-project deployment working: the
// serve flags stay the default, and a design that resolves to a project stops using them.
func TestFallbackAppliesOnlyWithoutAProject(t *testing.T) {
	fallbackOv, err := ComposeOverlay(&webapi.OverlayConfig{Conventions: conventionNaming("deployment", "^DEP_")}, "")
	if err != nil {
		t.Fatal(err)
	}
	r := resolverFor(map[string]*webapi.Project{
		"mount://m/acme/board.edn": {Name: "projects/acme", Conventions: conventionNaming("acme", "^ACME_")},
	})
	ctx := context.Background()

	loose, _ := artifact.Parse("mount://m/loose/board.edn")
	ov, err := r.Overlay(ctx, loose, nil, fallbackOv, "")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(sourceNames(ov), "deployment") {
		t.Errorf("a design in no project = %v, want the deployment default", sourceNames(ov))
	}

	owned, _ := artifact.Parse("mount://m/acme/board.edn")
	ov, err = r.Overlay(ctx, owned, nil, fallbackOv, "")
	if err != nil {
		t.Fatal(err)
	}
	if contains(sourceNames(ov), "deployment") {
		t.Errorf("a design in a project = %v, want its project's config rather than the deployment default", sourceNames(ov))
	}
}

// TestRequestOverridesTheProject: a caller that named its own conventions is answering for itself,
// and the project is the default it is overriding (WS3-124's rule, one layer out).
func TestRequestOverridesTheProject(t *testing.T) {
	r := resolverFor(map[string]*webapi.Project{
		"mount://m/acme/board.edn": {Name: "projects/acme", Conventions: conventionNaming("acme", "^ACME_")},
	})
	u, _ := artifact.Parse("mount://m/acme/board.edn")
	ov, err := r.Overlay(context.Background(), u,
		&webapi.OverlayConfig{Conventions: conventionNaming("mine", "^MINE_")}, Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(sourceNames(ov), "mine") {
		t.Errorf("composed %v, want the request's own convention", sourceNames(ov))
	}
	// The request's LEXICON replaces rather than merges, which is what makes switching visible: a
	// vocabulary that half-applied would answer under neither team's names.
	if ov.Lexicon == nil {
		t.Error("the request's lexicon should be the one in effect")
	}
}

// TestNilResolverFallsBack: a deployment with no descriptors behaves exactly as it did before
// projects existed, which is what lets this land without a migration.
func TestNilResolverFallsBack(t *testing.T) {
	fallbackOv, _ := ComposeOverlay(&webapi.OverlayConfig{Conventions: conventionNaming("deployment", "^DEP_")}, "")
	var r *ProjectResolver
	u, _ := artifact.Parse("mount://m/any/board.edn")
	ov, err := r.Overlay(context.Background(), u, nil, fallbackOv, "")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(sourceNames(ov), "deployment") {
		t.Fatalf("nil resolver composed %v, want the deployment default", sourceNames(ov))
	}
}

func sourceNames(o Overlay) []string {
	out := make([]string, 0, len(o.Sources))
	for _, s := range o.Sources {
		out = append(out, s.Name())
	}
	return out
}

func contains(hay []string, want string) bool {
	for _, h := range hay {
		if h == want {
			return true
		}
	}
	return false
}

var _ = check.Rule{}

// TestIgnoreProjectYieldsTheBuiltInCatalog: a reviewer asking "is this finding the engine's opinion
// or my project's" answers it by subtraction, so the opt-out has to produce EXACTLY what a design in
// no project produces — not a filtered approximation of it.
func TestIgnoreProjectYieldsTheBuiltInCatalog(t *testing.T) {
	r := resolverFor(map[string]*webapi.Project{
		"mount://m/acme/board.edn": {Name: "projects/acme", Conventions: conventionNaming("acme", "^ACME_")},
	})
	owned, _ := artifact.Parse("mount://m/acme/board.edn")
	ctx := context.Background()

	plain, err := r.Overlay(ctx, owned, &webapi.OverlayConfig{IgnoreProject: true}, Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if names := sourceNames(plain); len(names) != 0 {
		t.Errorf("ignore_project composed %v, want the built-in catalog only", names)
	}
	if plain.Lexicon != nil {
		t.Error("ignore_project must not leave the project's vocabulary in effect")
	}

	// It is the same answer a design in no project gets, which is what makes the comparison honest.
	loose, _ := artifact.Parse("mount://m/loose/board.edn")
	unowned, err := r.Overlay(ctx, loose, nil, Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sourceNames(plain)) != len(sourceNames(unowned)) {
		t.Errorf("ignore_project = %v but an unparented design = %v; they must agree", sourceNames(plain), sourceNames(unowned))
	}
}

// TestIgnoreProjectKeepsTheRequestsOwnConvention: "ignore what my project declares" and "use this
// instead" are different acts, and a caller doing both means both.
func TestIgnoreProjectKeepsTheRequestsOwnConvention(t *testing.T) {
	r := resolverFor(map[string]*webapi.Project{
		"mount://m/acme/board.edn": {Name: "projects/acme", Conventions: conventionNaming("acme", "^ACME_")},
	})
	u, _ := artifact.Parse("mount://m/acme/board.edn")
	ov, err := r.Overlay(context.Background(), u,
		&webapi.OverlayConfig{IgnoreProject: true, Conventions: conventionNaming("mine", "^MINE_")}, Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	names := sourceNames(ov)
	if !contains(names, "mine") {
		t.Errorf("composed %v, want the request's own convention", names)
	}
	if contains(names, "acme") {
		t.Errorf("composed %v, which still includes the project's", names)
	}
}
