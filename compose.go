package agni

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/stdlib/profiles"
)

// composeRules builds the catalog every rule-running surface shares, plus the profile index the
// review's absence gate reads. Both come out of one call so a caller cannot compose a catalog that
// silently omits a tier (WS3-109).
//
// It takes VALUES rather than paths. Reading a profile directory is the caller's business (C22), so
// an embedder can compose from profiles it built in Go and never wrote to disk. A design's intent is
// not composed here, because it is per design and has to reach the model as well as the catalog; it
// arrives on a request or in the design's own config (agni issue 831).
func composeRules(overlay []profiles.Profile, extra ...check.RuleSource) (*check.Catalog, map[string][]profiles.Profile, error) {
	var sources []check.RuleSource
	byName := map[string][]profiles.Profile{}
	for _, p := range profiles.Profiles {
		byName[p.Name] = append(byName[p.Name], p)
	}
	if len(overlay) > 0 {
		sources = append(sources, profiles.Source("profile-overlay", overlay))
		// An overlay profile REPLACES the same-named built-in here, tracking the catalog, whose overlay
		// source supersedes that built-in's rules (WS3-056). This map is the review's absence gate, and
		// reviewClosures reports an interface as evaluating if ANY profile under its name is in use.
		// Keeping the built-in here while the catalog drops it would let an item score a clean pass on
		// an interface nothing checked, silently (WS3-090).
		//
		// Cleared in a separate pass before any overlay profile is added, because clearing and
		// appending in one pass would make a later profile wipe an earlier one of the same name.
		// Catalog composition rejects that input today, so this guards a future relaxation.
		for _, p := range overlay {
			delete(byName, p.Name)
		}
		for _, p := range overlay {
			byName[p.Name] = append(byName[p.Name], p)
		}
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
