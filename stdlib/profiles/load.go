package profiles

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// profileDoc is the YAML wire shape of a Profile declaration (WS3-045). It is a DTO, separate from
// Profile, so the domain type carries no yaml tags and the file layout can nest (host: {attr, value})
// where the struct is flat. A customer authors one of these in their overlay; Compile turns it into
// check rules through the same requirement registry the built-ins use.
type profileDoc struct {
	Name         string          `yaml:"name"`
	Host         *hostDoc        `yaml:"host"`
	Signals      []signalDoc     `yaml:"signals"`
	Requirements []requirementDoc `yaml:"requirements"`
}

type hostDoc struct {
	Attr  string `yaml:"attr"`
	Value string `yaml:"value"`
}

type signalDoc struct {
	Name   string `yaml:"name"`
	Prefix string `yaml:"prefix"`
	Suffix string `yaml:"suffix"`
	Glob   string `yaml:"glob"`
	Regex  string `yaml:"regex"`
	PullUp bool   `yaml:"pullup"`
	Anchor bool   `yaml:"anchor"`
}

type requirementDoc struct {
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
}

// namingMapDoc is the YAML wire shape of a NAMING MAP (WS3-054): an overlay re-binds a core profile's
// signals to this project's net-name suffixes without re-authoring the profile. Override names the
// core profile (by Name); Suffixes maps a signal ROLE (Signal.Name, e.g. "TXD") to this project's
// suffix (e.g. "_TX"). Unmapped signals keep the core suffix; structure and requirements come from
// core unchanged. A file carrying "override:" is a naming map; a file carrying "name:" is a full
// profile. This is the structure-vs-naming split: the interface shape stays in core, the naming
// becomes overlay config.
type namingMapDoc struct {
	Override string            `yaml:"override"`
	Suffixes map[string]string `yaml:"suffixes"`
}

// Parse reads a YAML profile declaration into a Profile and validates its STRUCTURE (name present, at
// least one signal, at most one anchor) and every signal's MATCHER (exactly one form, compiling, not
// over-broad — see validateSignalMatcher). It does NOT check that requirement types are registered —
// that is Load's job, so Parse stays free of any registry dependency and WASM-clean (yaml only, no
// os). Built-in profiles load through Parse (their requirement types are covered by the test suite);
// external/customer profiles go through Load for the teaching type-check.
func Parse(b []byte) (Profile, error) {
	var doc profileDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return Profile{}, fmt.Errorf("profile: invalid YAML: %w", err)
	}
	if strings.TrimSpace(doc.Name) == "" {
		return Profile{}, fmt.Errorf("profile: missing required field \"name\"")
	}
	if len(doc.Signals) == 0 {
		return Profile{}, fmt.Errorf("profile %q: needs at least one signal", doc.Name)
	}
	p := Profile{Name: doc.Name}
	if doc.Host != nil {
		p.HostAttrKey, p.HostAttrVal = doc.Host.Attr, doc.Host.Value
	}
	anchors := 0
	for i, s := range doc.Signals {
		if strings.TrimSpace(s.Name) == "" {
			return Profile{}, fmt.Errorf("profile %q: signal #%d needs a \"name\"", doc.Name, i+1)
		}
		sig := Signal{Name: s.Name, Prefix: s.Prefix, Suffix: s.Suffix, Glob: s.Glob, Regex: s.Regex, PullUp: s.PullUp, Anchor: s.Anchor}
		// The matcher is validated HERE, at load, so a malformed regex or an over-broad pattern is a
		// teaching error naming the signal rather than a rule that silently matches every net.
		if err := validateSignalMatcher(sig); err != nil {
			return Profile{}, fmt.Errorf("profile %q: %w", doc.Name, err)
		}
		if s.Anchor {
			anchors++
		}
		p.Signals = append(p.Signals, sig)
	}
	if anchors > 1 {
		return Profile{}, fmt.Errorf("profile %q: at most one signal may be the anchor, got %d", doc.Name, anchors)
	}
	for _, r := range doc.Requirements {
		if strings.TrimSpace(r.Type) == "" {
			return Profile{}, fmt.Errorf("profile %q: a requirement is missing its \"type\"", doc.Name)
		}
		p.Requirements = append(p.Requirements, Requirement{Type: r.Type, Params: r.Params})
	}
	return p, nil
}

// Load reads a YAML profile from r, parses+structurally-validates it (Parse), and additionally
// checks that every declared requirement type is registered — reporting an unknown type with the
// list of known types so an overlay author sees what is available. This is the entry point for
// external/customer profiles (LoadDir, the CLI --profile-path flag).
func Load(r io.Reader) (Profile, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Profile{}, err
	}
	// A doc with "override:" is a naming map over a core profile, not a full declaration.
	var probe namingMapDoc
	if err := yaml.Unmarshal(b, &probe); err == nil && strings.TrimSpace(probe.Override) != "" {
		return loadNamingMap(probe)
	}
	p, err := Parse(b)
	if err != nil {
		return Profile{}, err
	}
	for _, req := range p.Requirements {
		if _, ok := requirementRegistry[req.Type]; !ok {
			return Profile{}, fmt.Errorf("profile %q: unknown requirement type %q (known: %s)",
				p.Name, req.Type, strings.Join(knownRequirementTypes(), ", "))
		}
	}
	return p, nil
}

// loadNamingMap resolves a naming map against a built-in profile and applies the suffix remap. Errors
// teach: an unknown override profile lists the known ones; a suffix keyed to a role the profile has no
// signal for names the valid roles.
func loadNamingMap(doc namingMapDoc) (Profile, error) {
	core, ok := ByName(doc.Override)
	if !ok {
		return Profile{}, fmt.Errorf("naming map: unknown profile %q (known: %s)",
			doc.Override, strings.Join(builtinProfileNames(), ", "))
	}
	roles := map[string]bool{}
	for _, s := range core.Signals {
		roles[s.Name] = true
	}
	for role := range doc.Suffixes {
		if !roles[role] {
			return Profile{}, fmt.Errorf("naming map for %q: no signal role %q (roles: %s)",
				doc.Override, role, strings.Join(profileRoles(core), ", "))
		}
	}
	return applyNamingMap(core, doc.Suffixes), nil
}

// applyNamingMap returns a copy of core with each signal whose role (Signal.Name) is in suffixes
// rebound to the mapped suffix; unmapped signals keep the core matcher. Host binding, anchor/pull-up
// flags, and requirements are inherited unchanged — only the naming moves.
//
// A remap REPLACES the signal's matcher rather than adding to it: the mapped suffix becomes the whole
// convention, clearing any prefix/glob/regex the core signal declared. A map that only overrode the
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

// mustParse loads a built-in profile from its embedded YAML, panicking on error (a malformed built-in
// is a programmer error caught at package init, same posture as ruleDoc).
func mustParse(b []byte) Profile {
	p, err := Parse(b)
	if err != nil {
		panic(fmt.Sprintf("profiles: malformed built-in profile: %v", err))
	}
	return p
}

// knownRequirementTypes returns the registered requirement type names, sorted, for teaching errors.
func knownRequirementTypes() []string {
	names := make([]string, 0, len(requirementRegistry))
	for n := range requirementRegistry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
