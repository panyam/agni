// Package composetest is the green half of the engine facade's acceptance. It exists as its own
// package because the two halves need OPPOSITE binaries: the facade's own tests assert what New
// refuses when a registration seam is empty, which is only observable in a binary that installed
// none, while these assert what it composes when every seam is filled. Registration is
// process-global, so one test binary cannot be in both states.
//
// The blank imports below ARE the subject. Dropping stdlib/rules/builtin, stdlib/rules/datalog or
// stdlib/reviewquery is the red-check for the assertion that names it, and each was confirmed red
// for its own reason.
//
// stdlib/relations is the exception and is listed for intent rather than for effect: stdlib/rules/datalog
// imports it, so dropping the blank import here changes nothing and the seam cannot be red-checked
// in this binary. The facade's own TestNewRefusesWhenNoRelationsAreInstalled is where that refusal
// is proven, in a binary that genuinely installs none. Keep the import anyway, because a program
// that drops the datalog suite would otherwise lose the fact base with it and silently.
package composetest

import (
	"context"
	"testing"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/profiles"

	_ "github.com/panyam/agni/stdlib/relations"     // facts.RegisterRelation
	_ "github.com/panyam/agni/stdlib/reviewquery"   // review.RegisterQueryCompiler
	_ "github.com/panyam/agni/stdlib/rules/builtin" // check.RegisterBuiltins
	_ "github.com/panyam/agni/stdlib/rules/datalog" // check.RegisterSource, source "dl"
)

func TestNewComposesEverySeamAndWarnsAboutNothing(t *testing.T) {
	e, err := agni.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !e.Registry().Installed() {
		t.Error("fact base is empty, so every datalog rule would match nothing")
	}
	if len(check.BuiltinRules()) == 0 {
		t.Error("built-in rules are not installed")
	}
	if w := e.Warnings(); len(w) != 0 {
		t.Errorf("a fully composed engine warned about %d things, want none: %v", len(w), w)
	}
}

func TestComposedCatalogCarriesEverySource(t *testing.T) {
	e, err := agni.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var bare, dl, profile int
	for _, r := range e.Catalog().Rules() {
		switch {
		case hasPrefix(r.Name, "dl/"):
			dl++
		case hasPrefix(r.Name, "profile/"):
			profile++
		default:
			bare++
		}
	}
	if bare == 0 {
		t.Error("no built-in rule reached the catalog")
	}
	if dl == 0 {
		t.Error("no datalog-authored rule reached the catalog")
	}
	if profile == 0 {
		t.Error("no interface-profile rule reached the catalog")
	}
}

// The two rule-running services must see ONE catalog. RuleServices does not take a catalog and
// returns both services together for exactly this reason, so the assertion is that what the check
// surface lists is what the engine composed, rule for rule.
func TestRuleServicesRunTheEnginesCatalog(t *testing.T) {
	e, err := agni.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	checkSvc, reviewSvc := e.RuleServices(agni.RuleServiceDeps{ReviewStore: service.NewMemReviewStore()})
	if reviewSvc == nil {
		t.Fatal("RuleServices returned no review service")
	}
	resp, err := checkSvc.ListRules(context.Background(), &webapi.ListRulesRequest{})
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	listed := map[string]bool{}
	for _, r := range resp.GetRules() {
		listed[r.GetName()] = true
	}
	for _, r := range e.Catalog().Rules() {
		if !listed[r.Name] {
			t.Errorf("rule %q is in the engine's catalog but not in what the check surface lists", r.Name)
		}
	}
}

// An overlay profile REPLACES the same-named built-in in the catalog, and the profile index the
// review's absence gate reads has to track that. An index still naming the built-in would let the
// gate clear on an interface whose rules are no longer in the run, so an item scoped by it would
// score a clean pass on an interface nothing checked.
func TestProfileIndexTracksAnOverlaySupersession(t *testing.T) {
	overlay := profiles.Profile{
		Name:    "CAN",
		Signals: []profiles.Signal{{Name: "CANH", Suffix: "_CANH", Anchor: true}},
	}
	e, err := agni.New(agni.WithProfiles([]profiles.Profile{overlay}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := e.ProfileIndex()["CAN"]
	if len(got) != 1 {
		t.Fatalf("profile index holds %d CAN profiles, want only the overlay's", len(got))
	}
	if len(got[0].Signals) != 1 {
		t.Errorf("index kept a CAN profile with %d signals; the built-in was not replaced", len(got[0].Signals))
	}
}

func TestWithoutDatalogRulesIsAcceptedWhenTheSuiteIsPresent(t *testing.T) {
	e, err := agni.New(agni.WithoutDatalogRules())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(e.Warnings()) != 0 {
		t.Errorf("warnings: %v", e.Warnings())
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
