package profiles

import (
	"fmt"
	"io"
	"sort"
	"strings"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/internal/yamlpb"
	"gopkg.in/yaml.v3"
)

// Parse reads a YAML profile declaration into a Profile and validates its STRUCTURE (name present, at
// least one signal, at most one anchor) and every signal's MATCHER (exactly one form, compiling, not
// over-broad; see validateSignalMatcher). It does NOT check that requirement types are registered.
// Load does that, so Parse stays free of any registry dependency and WASM-clean (yaml only, no os).
// Built-in profiles load through mustParse, which adds ValidateRequirements; external/customer
// profiles go through Load for the teaching type-check.
func Parse(b []byte) (Profile, error) {
	n, err := profileNode(b)
	if err != nil {
		return Profile{}, err
	}
	var def checkspb.ProfileDef
	if err := yamlpb.Decode(n, &def); err != nil {
		return Profile{}, fmt.Errorf("profile: %w", err)
	}
	p := ProfileFromProto(&def)
	if err := Validate(p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// profileNode parses a profile file's YAML. A file is bound to checkspb.ProfileDef, or to
// checkspb.ProfileNamingMap when its top level carries override, so the generated messages are the
// file's schema and a key neither has is an error naming its line (C26, agni issue 827).
func profileNode(b []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("profile: invalid YAML: %w", err)
	}
	return &doc, nil
}

// isNamingMap reports whether a profile file's top level carries override, which makes it a naming
// map rather than a full declaration.
func isNamingMap(n *yaml.Node) bool {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "override" {
			return true
		}
	}
	return false
}

// Validate reports why a profile cannot do what it says: a missing name, no signals, an unnamed
// signal, a requirement with no type, more than one anchor, an unsound signal matcher, or a
// completeness requirement with no anchor to hang on.
//
// It is exported and separate from Parse because a profile arrives by more than one route (YAML, and
// a serialized rule definition since WS3-103), and a second authoring route with its own idea of
// validity is how an unsound profile eventually gets in. The two failure classes it covers produce a
// rule which cannot fire rather than a rule that errors. An over-broad matcher silently claims foreign
// nets, and a completeness requirement with no anchor compiles to nothing at all, so an item bound to
// it scores a clean pass while the declared check never existed (WS3-099).
func Validate(p Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile: missing required field \"name\"")
	}
	if len(p.Signals) == 0 {
		return fmt.Errorf("profile %q: needs at least one signal", p.Name)
	}
	anchors := 0
	for i, s := range p.Signals {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("profile %q: signal #%d needs a \"name\"", p.Name, i+1)
		}
		if err := validateSignalMatcher(s); err != nil {
			return fmt.Errorf("profile %q: %w", p.Name, err)
		}
		if s.Anchor {
			anchors++
		}
	}
	if anchors > 1 {
		return fmt.Errorf("profile %q: at most one signal may be the anchor, got %d", p.Name, anchors)
	}
	for _, r := range p.Requirements {
		if strings.TrimSpace(r.Type) == "" {
			return fmt.Errorf("profile %q: a requirement is missing its \"type\"", p.Name)
		}
	}
	if err := validateAnchorDeclared(p); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	return nil
}

// Load reads a YAML profile from r, parses+structurally-validates it (Parse), and additionally
// checks that every declared requirement type is registered, reporting an unknown type with the
// list of known types so an overlay author sees what is available. This is the entry point for
// external/customer profiles (LoadDir, the CLI --profile-path flag).
func Load(r io.Reader) (Profile, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Profile{}, err
	}
	// A doc with "override:" is a naming map over a core profile, not a full declaration.
	n, err := profileNode(b)
	if err != nil {
		return Profile{}, err
	}
	if isNamingMap(n) {
		var nm checkspb.ProfileNamingMap
		if err := yamlpb.Decode(n, &nm); err != nil {
			return Profile{}, fmt.Errorf("naming map: %w", err)
		}
		return loadNamingMap(&nm)
	}
	p, err := Parse(b)
	if err != nil {
		return Profile{}, err
	}
	if err := ValidateRequirements(p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// ValidateRequirements checks each declared requirement against the registry, that its type has a
// compiler in this build and that its params satisfy that type's validator (WS3-047). Both failures
// are author errors in the declaration, and both produce a check that cannot run (an unknown type
// silently skipped, or a compiler handed params it cannot use).
//
// Separate from Validate because this half needs the requirement REGISTRY, which Validate (and so
// Parse) does not depend on. Parse stays structure-only and WASM-clean, and a built-in Go literal
// reaches its params gate through Compile instead. Exported for the same reason as Validate and
// RequirementTypes (WS3-103).
func ValidateRequirements(p Profile) error {
	for _, req := range p.Requirements {
		entry, ok := requirementRegistry[req.Type]
		if !ok {
			return fmt.Errorf("profile %q: unknown requirement type %q (known: %s)",
				p.Name, req.Type, strings.Join(knownRequirementTypes(), ", "))
		}
		if entry.validate == nil {
			continue
		}
		if err := entry.validate(req.Params); err != nil {
			return fmt.Errorf("profile %q: requirement %q: %w", p.Name, req.Type, err)
		}
	}
	return nil
}

// loadNamingMap resolves a naming map against a built-in profile and applies the suffix remap. Errors
// teach. An unknown override profile lists the known ones, and a suffix keyed to a role the profile has
// no signal for names the valid roles.
func loadNamingMap(doc *checkspb.ProfileNamingMap) (Profile, error) {
	core, ok := ByName(doc.GetOverride())
	if !ok {
		return Profile{}, fmt.Errorf("naming map: unknown profile %q (known: %s)",
			doc.GetOverride(), strings.Join(builtinProfileNames(), ", "))
	}
	roles := map[string]bool{}
	for _, s := range core.Signals {
		roles[s.Name] = true
	}
	for role := range doc.GetSuffixes() {
		if !roles[role] {
			return Profile{}, fmt.Errorf("naming map for %q: no signal role %q (roles: %s)",
				doc.GetOverride(), role, strings.Join(profileRoles(core), ", "))
		}
	}
	return applyNamingMap(core, doc.GetSuffixes()), nil
}

// applyNamingMap returns a copy of core with each signal whose role (Signal.Name) is in suffixes
// rebound to the mapped suffix; unmapped signals keep the core matcher. Host binding, anchor/pull-up
// flags, and requirements are inherited unchanged.
//
// A remap REPLACES the signal's matcher rather than adding to it, so the mapped suffix becomes the
// whole convention and clears any prefix/glob/regex the core signal declared. A map that only overrode the
// suffix of a glob-matched signal would leave two forms declared, which is not a matcher at all.
func applyNamingMap(core Profile, suffixes map[string]string) Profile {
	p := core
	p.Signals = make([]Signal, len(core.Signals))
	for i, s := range core.Signals {
		if sfx, ok := suffixes[s.Name]; ok {
			s.Prefix, s.Glob, s.Regex = "", "", ""
			s.Suffix = sfx
		}
		p.Signals[i] = s
	}
	return p
}

// builtinProfileNames returns the built-in profile names, sorted, for a teaching error.
func builtinProfileNames() []string {
	names := make([]string, 0, len(Profiles))
	for _, p := range Profiles {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// profileRoles returns a profile's signal roles, sorted, for a teaching error.
func profileRoles(p Profile) []string {
	roles := make([]string, 0, len(p.Signals))
	for _, s := range p.Signals {
		roles = append(roles, s.Name)
	}
	sort.Strings(roles)
	return roles
}

// mustParse builds a built-in profile from its embedded YAML (the authoritative declaration since
// WS3-049), panicking on error, so a malformed built-in fails at package init rather than shipping a
// profile that cannot do what it says (same posture as ruleDoc).
//
// It runs BOTH validation halves, so a built-in is held to exactly what a customer's overlay profile
// is. Parse covers structure and matchers, and ValidateRequirements covers requirement types and their
// params (WS3-047).
//
// It does not call Load, which is the other route that pairs those two halves. Load also resolves
// NAMING MAPS, which look their target up in Profiles, and Profiles is the list these very vars
// populate, so a built-in initialized through Load is an initialization cycle the compiler
// rejects (Profiles -> SPINOR -> Load -> loadNamingMap -> builtinProfileNames -> Profiles). Calling
// the two validators directly gets identical coverage with no cycle, since neither reaches Profiles.
func mustParse(b []byte) Profile {
	p, err := Parse(b)
	if err != nil {
		panic(fmt.Sprintf("profiles: malformed built-in profile: %v", err))
	}
	if err := ValidateRequirements(p); err != nil {
		panic(fmt.Sprintf("profiles: malformed built-in profile: %v", err))
	}
	return p
}

// RequirementTypes returns the registered requirement type names, sorted. Exported because a profile
// arrives from more than one place (YAML through Load, a serialized rule definition through the deck
// reader, WS3-103), and each has to reject an unknown type with the same teaching error rather
// than skipping it into a declared check that never runs.
func RequirementTypes() []string { return knownRequirementTypes() }

// knownRequirementTypes returns the registered requirement type names, sorted, for teaching errors.
func knownRequirementTypes() []string {
	names := make([]string, 0, len(requirementRegistry))
	for n := range requirementRegistry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
