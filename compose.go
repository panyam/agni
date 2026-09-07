package agni

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// composeRules builds the catalog every rule-running surface shares, plus the profile index the
// review's absence gate reads. Both come out of one call so a caller cannot compose a catalog that
// silently omits a tier: serve used to REBUILD its review catalog for the naming-convention case,
// which dropped the profile and intent sources whenever an operator passed --conventions together
// with --profile-path or --intent-path.
//
// It takes VALUES rather than paths. Reading a profile directory or an intent file is the caller's
// business (C22), which is also what lets an embedder compose from profiles it built in Go and never
// wrote to disk.
func composeRules(overlay []profiles.Profile, decl *intent.Declaration, extra ...check.RuleSource) (*check.Catalog, map[string][]profiles.Profile, error) {
	var sources []check.RuleSource
	byName := map[string][]profiles.Profile{}
	for _, p := range profiles.Profiles {
		byName[p.Name] = append(byName[p.Name], p)
	}
	if len(overlay) > 0 {
		sources = append(sources, profiles.Source("profile-overlay", overlay))
		// An overlay profile REPLACES the same-named built-in here, tracking the catalog, whose overlay
		// source supersedes that built-in's rules (WS3-056). This map is the review's absence gate:
		// reviewClosures reports an interface as evaluating if ANY profile under its name is in use, and
		// unions every one of their nets for scoping. Keeping the built-in here while the catalog drops it
		// would let the gate clear on a profile whose rules are no longer in the run, and an item scoped by
		// it would score a clean pass on an interface nothing checked. That is the WS3-090 twin
		// disagreement, which is silent by construction.
		//
		// Cleared in a separate pass before any overlay profile is added. Clearing and appending in one
		// pass would make a later profile wipe an earlier one of the same name. That specific input is
		// rejected upstream (identical rule names fail catalog composition), so the two-pass form is not
		// load-bearing today, but it costs nothing and the one-pass form is wrong for a reason unrelated
		// to why it currently cannot happen.
		for _, p := range overlay {
			delete(byName, p.Name)
		}
		for _, p := range overlay {
			byName[p.Name] = append(byName[p.Name], p)
		}
	}
	if decl != nil {
		sources = append(sources, intent.Source("intent", *decl))
	}
	sources = append(sources, extra...)
	catalog := check.DefaultCatalog()
	if len(sources) > 0 {
		catalog = check.CatalogWith(sources...)
	}
	return catalog, byName, nil
}

// reviewQueryCompilerInstalled reports whether an engine has registered itself to compile a review
// manifest's inline query bindings. A binary that composes none still loads every manifest that
// binds no inline query, so this is a warning rather than a refusal.
func reviewQueryCompilerInstalled() bool { return review.QueryCompilerInstalled() }
