package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// recordingResolver is a ConfigResolver that answers from a table instead of a filesystem, and
// records what it was asked. It stands in for the OS adapter so these tests assert the COMPOSITION
// rule rather than re-testing file loading.
type recordingResolver struct {
	// asked is (namespace, uri) for every profile set requested, in order.
	asked  []string
	specs  param.ParamProvider
	failOn string
}

func (r *recordingResolver) ResolveConfig(_ context.Context, cfg *webapi.AnalysisConfig, namespace string) (ResolvedConfig, error) {
	var out ResolvedConfig
	for _, uri := range cfg.GetProfileUris() {
		if uri == r.failOn {
			return ResolvedConfig{}, errFake
		}
		r.asked = append(r.asked, namespace+" "+uri)
		out.Sources = append(out.Sources, check.NewSource(namespace+"-profiles", nil))
		out.Profiles = true
	}
	for range cfg.GetParamUris() {
		out.Specs = r.specs
	}
	return out, nil
}

var errFake = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "fake resolve failure" }

// TestRequestConfigResolvesRefTiers covers the capability this schema exists for, which is that a
// caller that is not the CLI can hand the engine profiles, parameters and intent, not only a naming
// convention.
//
// Before this, OverlayConfig could carry exactly one of five config tiers, so a browser could be given
// a fifth of the configuration a command line could, and the gap was invisible, because a run against
// less config produces fewer findings and fewer findings is what a clean board looks like.
func TestRequestConfigResolvesRefTiers(t *testing.T) {
	res := &recordingResolver{specs: someSpecs()}
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{
		ProfileUris: []string{"mount://m/profiles"},
		ParamUris:   []string{"mount://m/params"},
		Intent:      &configpb.DesignIntent{Modules: []*configpb.IntentModule{{Name: "MCU", Class: "ic"}}},
	}}
	ov, err := OverlayFor(context.Background(), res, nil, nil, nil, req, Overlay{}, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	if ov.Specs == nil {
		t.Error("a request-supplied parameter corpus must reach the run")
	}
	if !ov.Profiles || !ov.Intent {
		t.Errorf("provenance must record the tiers the request supplied, got profiles=%v intent=%v", ov.Profiles, ov.Intent)
	}
	if got := sourceNames(ov); len(got) != 2 {
		t.Errorf("request profiles and intent should both compose, got %v", got)
	}
	// The namespace is what keeps a request's profiles from colliding with a project's, and it is
	// visibly not a resource name so a catalog snapshot says where the rule came from.
	if len(res.asked) != 1 || !strings.HasPrefix(res.asked[0], requestNamespace+" ") {
		t.Errorf("request config should resolve under the request namespace, got %v", res.asked)
	}
}

// TestRequestConfigLayersOverTheProject checks that a request's profiles ADD to the project's
// rather than replacing them. That is the opposite of how a request CONVENTION behaves, and
// deliberately so. A caller sending profiles is adding a check to the run, where a caller sending a
// convention is answering for the whole naming vocabulary and cannot stack two.
func TestRequestConfigLayersOverTheProject(t *testing.T) {
	res := &recordingResolver{}
	project := &webapi.Project{
		Name:   "projects/acme",
		Config: &webapi.AnalysisConfig{ProfileUris: []string{"mount://m/acme/profiles"}},
	}
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{ProfileUris: []string{"mount://m/mine"}}}
	ov, err := OverlayFor(context.Background(), res, nil, project, &webapi.Design{}, req, Overlay{}, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	got := sourceNames(ov)
	if len(got) != 2 {
		t.Fatalf("both the project's and the request's profiles should compose, got %v", got)
	}
	if got[0] == got[1] {
		t.Errorf("the two must land under DIFFERENT source names or the second collides with the first, got %v", got)
	}
	if len(res.asked) != 2 || !strings.HasPrefix(res.asked[0], "projects/acme ") || !strings.HasPrefix(res.asked[1], requestNamespace+" ") {
		t.Errorf("project first, then request; got %v", res.asked)
	}
}

// TestRequestCorpusWinsOverTheProject is the one place a corpus still replaces rather than layers: the
// caller named a corpus for this run, which is choosing it outright (DECISIONS.md, "A project's
// datasheet corpus is layered over a shared one per MPN").
func TestRequestCorpusWinsOverTheProject(t *testing.T) {
	res := &recordingResolver{specs: someSpecs()}
	project := &webapi.Project{Name: "projects/acme", Config: &webapi.AnalysisConfig{}}
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{ParamUris: []string{"mount://m/mine"}}}
	ov, err := OverlayFor(context.Background(), res, nil, project, &webapi.Design{}, req, Overlay{}, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	if ov.Specs == nil {
		t.Error("the request's corpus should be the one in effect")
	}
}

// TestHostWithoutResolverRefusesRefs is the refusing half of the capability, and the reason this is a
// deployment property rather than a schema one.
//
// A host with no resolver (the engine in WASM, a service constructed without one) can still honour a
// config carrying a RESOLVED convention, because that composes with no I/O, the property
// C22 protects and this change had to preserve. A config naming a directory is a different request,
// and answering it by silently dropping the tier would report a clean run against config that never
// loaded. So it errors, and the error says what it could not resolve.
func TestHostWithoutResolverRefusesRefs(t *testing.T) {
	ctx := context.Background()
	refs := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{ProfileUris: []string{"mount://m/profiles"}}}
	_, err := OverlayFor(ctx, nil, nil, nil, nil, refs, Overlay{}, "")
	if err == nil {
		t.Fatal("a host that cannot resolve refs must refuse rather than silently drop the tier")
	}
	for _, want := range []string{"cannot resolve", "profiles"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name what could not be resolved, missing %q: %v", want, err)
		}
	}

	// The no-I/O path is untouched, so a resolved convention still composes with no resolver at all.
	valueOnly := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conventionNaming("house", "^H_")}}
	ov, err := OverlayFor(ctx, nil, nil, nil, nil, valueOnly, Overlay{}, "")
	if err != nil {
		t.Fatalf("a value-shaped config must still compose with no resolver: %v", err)
	}
	if ov.Lexicon == nil {
		t.Error("the convention's lexicon should have composed")
	}
}

// TestProjectRefsAlsoNeedAResolver applies the same refusal to a PROJECT that declares refs on a host
// that cannot read them. Failing only on the request side would mean a deployment silently ran a
// project's designs against a fraction of that project's config.
func TestProjectRefsAlsoNeedAResolver(t *testing.T) {
	project := &webapi.Project{
		Name:   "projects/acme",
		Config: &webapi.AnalysisConfig{ProfileUris: []string{"mount://m/acme/profiles"}},
	}
	_, err := OverlayFor(context.Background(), nil, nil, project, &webapi.Design{}, nil, Overlay{}, "")
	if err == nil {
		t.Fatal("a project declaring refs on a resolver-less host must refuse")
	}
	if !strings.Contains(err.Error(), "projects/acme") {
		t.Errorf("the error should name the project, got %v", err)
	}
}

// TestMergeConfigLayersFieldWise covers a Design setting only intent while a Project sets the rest,
// so merging has to be per field. A whole-message replace would make a design that declares intent
// drop its project's profiles, which reads as the project having none.
func TestMergeConfigLayersFieldWise(t *testing.T) {
	di := &configpb.DesignIntent{Modules: []*configpb.IntentModule{{Name: "MCU", Class: "ic"}}}
	p := &webapi.AnalysisConfig{ProfileUris: []string{"p"}, ParamUris: []string{"q"},
		Checklists: []*webapi.NamedChecklist{{Name: "review"}}}
	d := &webapi.AnalysisConfig{Intent: di}
	got := mergeConfig(p, d)
	if len(got.GetProfileUris()) != 1 || len(got.GetParamUris()) != 1 {
		t.Errorf("the project's ref tiers must survive a design that declares intent, got %+v", got)
	}
	if got.GetIntent() != di || len(got.GetChecklists()) != 1 {
		t.Errorf("both scopes' fields should be present, got %+v", got)
	}
}

// A checklist inherited through extends is replaced IN PLACE by one of the same name, so the
// inherited default stays first, and a new name follows the inherited ones.
func TestMergeChecklistsByName(t *testing.T) {
	parent := []*webapi.NamedChecklist{{Name: "review", Manifest: &checkspb.ReviewManifest{Name: "shared"}}, {Name: "dft"}}
	child := []*webapi.NamedChecklist{{Name: "house"}, {Name: "review", Manifest: &checkspb.ReviewManifest{Name: "ours"}}}
	got := mergeChecklists(parent, child)
	var names []string
	for _, c := range got {
		names = append(names, c.GetName())
	}
	if strings.Join(names, ",") != "review,dft,house" {
		t.Errorf("order = %v, want review,dft,house", names)
	}
	if got[0].GetManifest().GetName() != "ours" {
		t.Errorf("the nearer project's review must replace the inherited one, got %q", got[0].GetManifest().GetName())
	}
	if len(parent) != 2 || parent[0].GetManifest().GetName() != "shared" {
		t.Error("merging must not modify the inherited list")
	}
}

// A design's intent arrives as a value and compiles in the service, with no resolver at all, so a
// host with no filesystem still runs the intent tier. An invalid declaration fails the run rather
// than dropping the tier.
func TestIntentValueComposesWithoutAResolver(t *testing.T) {
	d := &webapi.Design{Name: "projects/p/designs/board", Config: &webapi.AnalysisConfig{
		Intent: &configpb.DesignIntent{Modules: []*configpb.IntentModule{{Name: "MCU", Class: "ic"}}},
	}}
	ov, err := OverlayFor(context.Background(), nil, nil, &webapi.Project{Name: "projects/p"}, d, nil, Overlay{}, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	if !ov.Intent || len(sourceNames(ov)) != 1 || sourceNames(ov)[0] != "intent" {
		t.Errorf("the design's intent must compose as the intent source, got intent=%v sources=%v", ov.Intent, sourceNames(ov))
	}
	d.Config.Intent = &configpb.DesignIntent{Modules: []*configpb.IntentModule{{Name: "MCU"}}}
	_, err = OverlayFor(context.Background(), nil, nil, &webapi.Project{Name: "projects/p"}, d, nil, Overlay{}, "")
	if err == nil || !strings.Contains(err.Error(), `module "MCU" needs a "class"`) {
		t.Errorf("an invalid intent must fail the run and say why, got %v", err)
	}
}
