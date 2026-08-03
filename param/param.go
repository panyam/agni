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
	for i, p := range spec.Parameters {
		id := p.Symbol
		if id == "" {
			id = fmt.Sprintf("parameters[%d]", i)
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
