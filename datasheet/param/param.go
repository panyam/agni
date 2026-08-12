// Package param loads and validates parameter-IR PartSpecs (agni.v1.param), the
// datasheet-parameter contract described in docs/20-parameter-ir.md. The schema lives
// in protos/agni/v1/param/param.proto; this package holds the invariants the proto
// language cannot express (required provenance, resolvable doc refs, non-empty
// ranges) and the under-specification predicate consumers must respect.
package param

import (
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/prototext"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Load parses one PartSpec in textproto form (the fixture and hand-encoding format).
// It only parses; call Validate for the semantic invariants.
func Load(r io.Reader) (*parampb.PartSpec, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	spec := &parampb.PartSpec{}
	if err := prototext.Unmarshal(data, spec); err != nil {
		return nil, fmt.Errorf("param: parse PartSpec: %w", err)
	}
	return spec, nil
}

// Validate checks the invariants every PartSpec must hold before any consumer (the
// WS10-003 join, a store) may trust it: a join key (mpn), and on every parameter a
// classified limit kind, a value with at least one bound and min <= typ <= max where
// present, and provenance that resolves to a declared source doc with a confidence
// someone stands behind (in (0, 1]). All violations are reported, joined into one
// error; nil means the spec is trustworthy as data (not that its values are true --
// that is what the provenance is for).
//
// When a spec carries pin data it must also be COHERENT: unique package and pin ids,
// a name on every pin, numbers that resolve to a declared package with no two pins
// claiming one number within it, provenance on every pin, and every Parameter.pin_refs
// resolving to a declared pin. A dangling binding is the failure worth catching at load,
// because downstream it does not look like an error -- the parameter simply stops
// applying to anything and the rule that wanted it reports nothing.
//
// Pin data is entirely OPTIONAL. A spec with no pins or packages validates exactly as it
// did before pin binding existed, so an older corpus is unaffected (CONSTRAINTS C9).
func Validate(spec *parampb.PartSpec) error {
	var errs []error
	if spec.Mpn == "" {
		errs = append(errs, errors.New("part spec has no mpn (the join key to the design IR)"))
	}
	docs := make(map[string]bool, len(spec.Docs))
	for i, d := range spec.Docs {
		if d.Id == "" {
			errs = append(errs, fmt.Errorf("docs[%d] has no id", i))
		}
		docs[d.Id] = true
	}
	packages := make(map[string]bool, len(spec.Packages))
	for i, pkg := range spec.Packages {
		if pkg.Id == "" {
			errs = append(errs, fmt.Errorf("packages[%d] has no id", i))
			continue
		}
		if packages[pkg.Id] {
			errs = append(errs, fmt.Errorf("packages[%d]: duplicate package id %q", i, pkg.Id))
		}
		packages[pkg.Id] = true
	}

	pins := make(map[string]bool, len(spec.Pins))
	// A pin NUMBER belongs to one pin within one package. Names may repeat (that is the
	// multi-supply case pin binding exists for) but a package cannot send one terminal to
	// two pins, and a corpus that says otherwise would make the tie-breaking channel
	// unusable.
	claimed := map[string]string{}
	for i, p := range spec.Pins {
		id := p.Id
		if id == "" {
			id = fmt.Sprintf("pins[%d]", i)
			errs = append(errs, fmt.Errorf("%s has no id; a parameter cannot bind to it", id))
		} else {
			if pins[p.Id] {
				errs = append(errs, fmt.Errorf("%s: duplicate pin id", id))
			}
			pins[p.Id] = true
		}
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("%s: no name; the name is the channel that survives repackaging", id))
		}
		for j, n := range p.Numbers {
			if n.Number == "" {
				errs = append(errs, fmt.Errorf("%s: numbers[%d] has no number", id, j))
			}
			if !packages[n.PackageRef] {
				errs = append(errs, fmt.Errorf("%s: numbers[%d] package_ref %q does not resolve to a declared package", id, j, n.PackageRef))
				continue
			}
			key := n.PackageRef + "\x00" + normalizePinNumber(n.Number)
			if prev, dup := claimed[key]; dup {
				errs = append(errs, fmt.Errorf("%s: number %q in package %q is already claimed by pin %q", id, n.Number, n.PackageRef, prev))
			}
			claimed[key] = id
		}
		switch {
		case p.Prov == nil:
			errs = append(errs, fmt.Errorf("%s: no prov; a pin function is an extracted claim like any other", id))
		case !docs[p.Prov.DocRef]:
			errs = append(errs, fmt.Errorf("%s: prov.doc_ref %q does not resolve to a declared source doc", id, p.Prov.DocRef))
		case p.Prov.Confidence <= 0 || p.Prov.Confidence > 1:
			errs = append(errs, fmt.Errorf("%s: prov.confidence %v outside (0, 1]", id, p.Prov.Confidence))
		}
	}

	for i, p := range spec.Parameters {
		id := p.Symbol
		if id == "" {
			id = fmt.Sprintf("parameters[%d]", i)
		}
		for _, ref := range p.PinRefs {
			if !pins[ref] {
				errs = append(errs, fmt.Errorf("%s: pin_refs %q does not resolve to a declared pin", id, ref))
			}
		}
		if p.LimitKind == parampb.LimitKind_LIMIT_KIND_UNSPECIFIED {
			errs = append(errs, fmt.Errorf("%s: limit_kind is unspecified; classify or drop", id))
		}
		if p.Value == nil || (p.Value.Min == nil && p.Value.Typ == nil && p.Value.Max == nil) {
			errs = append(errs, fmt.Errorf("%s: value has no min, typ, or max", id))
		} else if p.Value.Min != nil && p.Value.Max != nil && p.Value.GetMin() > p.Value.GetMax() {
			errs = append(errs, fmt.Errorf("%s: value min %v above max %v", id, p.Value.GetMin(), p.Value.GetMax()))
		}
		switch {
		case p.Prov == nil:
			errs = append(errs, fmt.Errorf("%s: no prov; every parameter carries provenance", id))
		case !docs[p.Prov.DocRef]:
			errs = append(errs, fmt.Errorf("%s: prov.doc_ref %q does not resolve to a declared source doc", id, p.Prov.DocRef))
		case p.Prov.Confidence <= 0 || p.Prov.Confidence > 1:
			errs = append(errs, fmt.Errorf("%s: prov.confidence %v outside (0, 1]", id, p.Prov.Confidence))
		}
	}
	return errors.Join(errs...)
}

// UnderSpecified reports whether a parameter's value must not be treated as a plain
// comparable limit: its condition list is not asserted complete (coverage is unknown
// or known-partial). A no-condition parameter is fine only when the source genuinely
// states none (CONDITION_COVERAGE_UNCONDITIONAL). Consumers skip or flag
// under-specified parameters; they never compare against them (docs/20).
func UnderSpecified(p *parampb.Parameter) bool {
	switch p.ConditionCoverage {
	case parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
		parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL:
		return false
	}
	return true
}

// MachineComparable reports whether a consumer may compare this parameter's value
// against an operating point automatically: the condition list is asserted complete
// (not UnderSpecified) AND every condition is structured (carries eq or min/max).
// A condition captured only as text (raw) is honestly retained but cannot be
// evaluated against anything, so the row is the middle trust state: fit to show a
// human next to its provenance, never fit for an automatic comparison. The three
// states a consumer must distinguish (docs/20):
//
//	UnderSpecified            -> skip; the conditions themselves are not trustworthy
//	!UnderSpecified && !MachineComparable -> surface to a human; no auto-compare
//	MachineComparable         -> safe to compare, under its stated conditions
func MachineComparable(p *parampb.Parameter) bool {
	if UnderSpecified(p) {
		return false
	}
	for _, c := range p.Conditions {
		if c.Eq == nil && c.Min == nil && c.Max == nil {
			return false
		}
	}
	return true
}
