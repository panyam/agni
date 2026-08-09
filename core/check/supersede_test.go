package check

import (
	"slices"
	"testing"
)

func ruleNames(rules []*Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.Name)
	}
	return out
}

// tagged builds a rule carrying an arbitrary tag set, the shape supersession selects on.
func tagged(name string, tags map[string]string) *Rule {
	return &Rule{Name: name, Tags: tags}
}

// Without is the exclusion complement of Filter: same Facets grammar, opposite sense. It must not
// mutate the receiver, since a catalog is shared by every surface composed from it.
func TestCatalogWithout(t *testing.T) {
	c, err := NewCatalog(NewSource("acme", []*Rule{
		tagged("a", map[string]string{"iface": "SPI"}),
		tagged("b", map[string]string{"iface": "CAN"}),
		tagged("c", map[string]string{"iface": "SPI"}),
	}))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}

	byTag := c.Without(Facets{Tags: map[string][]string{"iface": {"SPI"}}})
	if got, want := ruleNames(byTag.Rules()), []string{"acme/b"}; !slices.Equal(got, want) {
		t.Errorf("Without(iface=SPI) = %v, want %v", got, want)
	}
	if byTag.Lookup("acme/a") != nil {
		t.Error("Without left an excluded rule reachable through Lookup")
	}

	byName := c.Without(Facets{Names: []string{"acme/b"}})
	if got, want := ruleNames(byName.Rules()), []string{"acme/a", "acme/c"}; !slices.Equal(got, want) {
		t.Errorf("Without(name=acme/b) = %v, want %v", got, want)
	}

	if got := len(c.Rules()); got != 3 {
		t.Errorf("Without mutated the receiver: %d rules left, want 3", got)
	}
}

// An empty Facets selects every rule for Filter, so Without empties the catalog. Asserted because the
// opposite reading ("no constraint, so exclude nothing") is the natural one and would silently keep
// every rule a caller meant to drop.
func TestCatalogWithoutEmptyFacetsExcludesEverything(t *testing.T) {
	c, err := NewCatalog(NewSource("acme", []*Rule{tagged("a", nil), tagged("b", nil)}))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if got := len(c.Without(Facets{}).Rules()); got != 0 {
		t.Errorf("Without(Facets{}) kept %d rules, want 0", got)
	}
}

// A superseding source replaces the rules it names and records what it dropped.
func TestSupersedingSourceReplacesEarlierRules(t *testing.T) {
	core := NewSource("profile", []*Rule{
		tagged("spi-missing", map[string]string{"profile": "SPI"}),
		tagged("can-missing", map[string]string{"profile": "CAN"}),
	})
	overlay := NewSupersedingSource("profile-overlay", []*Rule{
		tagged("spi-missing", map[string]string{"profile": "SPI"}),
	}, Facets{Tags: map[string][]string{"profile": {"SPI"}, KeySource: {"profile"}}})

	c, err := NewCatalog(core, overlay)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	want := []string{"profile/can-missing", "profile-overlay/spi-missing"}
	if got := ruleNames(c.Rules()); !slices.Equal(got, want) {
		t.Errorf("composed catalog = %v, want %v", got, want)
	}

	sup := c.Superseded()
	if len(sup) != 1 || sup[0].By != "profile-overlay" ||
		!slices.Equal(sup[0].Rules, []string{"profile/spi-missing"}) {
		t.Errorf("Superseded() = %+v, want one entry by profile-overlay dropping profile/spi-missing", sup)
	}
}

// A superseded rule leaves the name index too, so Rules and Lookup cannot disagree about what the
// catalog contains. Nothing in the engine resolves a rule through Lookup today (--rule goes through
// Filter, over the rule slice), which is exactly why this needs pinning: the first consumer that does
// would otherwise reach a rule the catalog had dropped, and get it back.
func TestSupersededRuleLeavesTheNameIndex(t *testing.T) {
	core := NewSource("profile", []*Rule{tagged("spi-missing", map[string]string{"profile": "SPI"})})
	overlay := NewSupersedingSource("profile-overlay", []*Rule{
		tagged("spi-missing", map[string]string{"profile": "SPI"}),
	}, Facets{Tags: map[string][]string{"profile": {"SPI"}, KeySource: {"profile"}}})

	c, err := NewCatalog(core, overlay)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if got := c.Lookup("profile/spi-missing"); got != nil {
		t.Errorf("Lookup returned a superseded rule (%q); Rules and Lookup must agree", got.Name)
	}
	if c.Lookup("profile-overlay/spi-missing") == nil {
		t.Error("Lookup lost the superseding rule")
	}
}

// A declaration that matches nothing records nothing. Without this the catalog accumulates an empty
// supersession, and the surfaces that report one print "supersedes 0 rule(s):" on a run where no rule
// was replaced.
func TestDeclarationMatchingNothingRecordsNothing(t *testing.T) {
	c, err := NewCatalog(
		NewSource("profile", []*Rule{tagged("can-missing", map[string]string{"profile": "CAN"})}),
		NewSupersedingSource("profile-overlay", []*Rule{tagged("other", nil)},
			Facets{Tags: map[string][]string{"profile": {"NOSUCHINTERFACE"}}}),
	)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if got := c.Superseded(); len(got) != 0 {
		t.Errorf("Superseded() = %+v, want empty when the declaration matched no rule", got)
	}
	if len(c.Rules()) != 2 {
		t.Errorf("a declaration matching nothing changed the catalog: %v", ruleNames(c.Rules()))
	}
}

// The exemption that makes supersession safe. An overlay profile and the built-in it replaces tag
// their rules identically, so a declaration matching on tags alone would delete the REPLACEMENT too
// and leave the interface with no rules. That failure is invisible in a report: the findings simply
// stop, which reads as a clean pass.
func TestSupersedingSourceNeverDropsItsOwnRules(t *testing.T) {
	core := NewSource("profile", []*Rule{tagged("spi-missing", map[string]string{"profile": "SPI"})})
	// Deliberately NOT scoped by source, so only the self-exemption can save the overlay's own rule.
	overlay := NewSupersedingSource("profile-overlay", []*Rule{
		tagged("spi-missing", map[string]string{"profile": "SPI"}),
	}, Facets{Tags: map[string][]string{"profile": {"SPI"}}})

	c, err := NewCatalog(core, overlay)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if got, want := ruleNames(c.Rules()), []string{"profile-overlay/spi-missing"}; !slices.Equal(got, want) {
		t.Fatalf("composed catalog = %v, want %v (the overlay must survive its own declaration)", got, want)
	}
}

// Supersession applies through With too, which is the path serve and review extend a base catalog by.
// The record carries across, so a surface that only ever sees the extended catalog can still report it.
func TestSupersedingSourceThroughWith(t *testing.T) {
	base, err := NewCatalog(NewSource("profile", []*Rule{
		tagged("spi-missing", map[string]string{"profile": "SPI"}),
	}))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	ext, err := base.With(NewSupersedingSource("profile-overlay", []*Rule{
		tagged("spi-missing", map[string]string{"profile": "SPI"}),
	}, Facets{Tags: map[string][]string{"profile": {"SPI"}, KeySource: {"profile"}}}))
	if err != nil {
		t.Fatalf("With: %v", err)
	}
	if got, want := ruleNames(ext.Rules()), []string{"profile-overlay/spi-missing"}; !slices.Equal(got, want) {
		t.Errorf("extended catalog = %v, want %v", got, want)
	}
	if len(ext.Superseded()) != 1 {
		t.Errorf("Superseded() = %+v, want the supersession recorded on the extended catalog", ext.Superseded())
	}
	if got := len(base.Rules()); got != 1 {
		t.Errorf("With mutated the base catalog: %d rules, want 1", got)
	}
}

// A source declaring no supersession is an ordinary augmenting source. This is the invariant that
// keeps every existing overlay behaving exactly as it did.
func TestSourceWithoutDeclarationStillAugments(t *testing.T) {
	c, err := NewCatalog(
		NewSource("profile", []*Rule{tagged("spi-missing", map[string]string{"profile": "SPI"})}),
		NewSupersedingSource("profile-overlay", []*Rule{tagged("other", nil)}),
	)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	want := []string{"profile/spi-missing", "profile-overlay/other"}
	if got := ruleNames(c.Rules()); !slices.Equal(got, want) {
		t.Errorf("composed catalog = %v, want %v", got, want)
	}
	if len(c.Superseded()) != 0 {
		t.Errorf("Superseded() = %+v, want empty", c.Superseded())
	}
}
