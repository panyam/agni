// Package param loads and validates parameter-IR PartSpecs (agni.v1.param), the
// datasheet-parameter contract in
// docsite/content/architecture/datasheet-layer.md#the-parameter-contract. The schema
// is protos/agni/v1/param/param.proto. This package holds the invariants proto cannot
// express (required provenance, resolvable doc refs, non-empty ranges) and the
// under-specification predicate consumers must respect.
package param

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Load parses one PartSpec in textproto form, the fixture and hand-encoding format.
// It only parses, so call Validate for the semantic invariants.
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
// WS10-003 join, a store) may trust it, which is every Problem joined into one error.
// Nil means the spec is trustworthy as data, not that its values are true; the
// provenance is for that.
//
// When a spec carries pin data it must also be COHERENT (see structuralProblems). A
// dangling pin binding is worth catching at load because downstream it does not look
// like an error. The parameter stops applying to anything and the rule that wanted it
// reports nothing. Pin data is OPTIONAL, so a spec with no pins or packages is
// unaffected (CONSTRAINTS C9).
func Validate(spec *parampb.PartSpec) error {
	var errs []error
	for _, p := range Problems(spec) {
		errs = append(errs, errors.New(p.Message))
	}
	return errors.Join(errs...)
}

// ProblemKind separates the two questions Problems answers, since a consumer usually wants only one.
// STRUCTURAL means the spec contradicts itself and is wrong at any stage of authoring. COMPLETENESS
// means it is unfinished, which is true of every spec under transcription.
type ProblemKind string

const (
	ProblemStructural   ProblemKind = "structural"
	ProblemCompleteness ProblemKind = "completeness"
)

// Problem is one validation finding, unjoined so a caller can render each one and act on the kinds
// differently.
type Problem struct {
	Kind    ProblemKind
	Message string
}

// Problems reports everything wrong with a spec, classified. The workbench consumes it, showing
// structural problems as things to fix now and completeness ones as what stands between a draft and a
// corpus. Validate is this, joined. An empty result means the spec would load into a corpus today.
func Problems(spec *parampb.PartSpec) []Problem {
	var out []Problem
	for _, e := range structuralProblems(spec) {
		out = append(out, Problem{Kind: ProblemStructural, Message: e.Error()})
	}
	for _, e := range completenessProblems(spec) {
		out = append(out, Problem{Kind: ProblemCompleteness, Message: e.Error()})
	}
	return out
}

// completenessProblems reports what a spec still lacks to be trusted as corpus data. That is a join
// key (mpn), a name and resolvable provenance on every pin, and on every parameter and relation a
// classified kind, a non-empty bound with min <= max, and provenance resolving to a declared doc with
// confidence in (0, 1]. Each is a legitimate state mid-transcription, unlike structuralProblems.
func completenessProblems(spec *parampb.PartSpec) []error {
	var errs []error
	if spec.Mpn == "" {
		errs = append(errs, errors.New("part spec has no mpn (the join key to the design IR)"))
	}
	docs := make(map[string]bool, len(spec.Docs))
	for i, d := range spec.Docs {
		if d.Id == "" {
			errs = append(errs, fmt.Errorf("docs[%d] has no id", i))
		}
		// The title is the citation an engineer opens, so it names the DOCUMENT, number and
		// revision as printed.
		//
		// ABSENT is not a problem here. A first-pass derivation cannot state the identity yet, and
		// that refusal goes in the run manifest's gap list. Reporting it here too would make every
		// derived spec fail Validate, which is how derive tells a bug from a data gap. The citation
		// says "revision unrecorded" at the point of use.
		//
		// EQUAL TO THE MPN is a problem, because it is a wrong assertion rather than an absence. A
		// producer copying a doc-IR title writes it, and it is the same string before and after a
		// reissue, so a reader learns nothing about which revision to open (agni issue 290).
		//
		// Equality, not a guess at what a part number looks like, because a heuristic would reject
		// legitimate titles from vendors whose document numbering nobody has seen.
		if d.Title != "" && spec.Mpn != "" && strings.EqualFold(strings.TrimSpace(d.Title), strings.TrimSpace(spec.Mpn)) {
			errs = append(errs, fmt.Errorf(
				"docs[%d] (%s) is titled %q, which is the part, not the document; the title is the citation an engineer opens, so it wants the vendor's document number and revision as printed",
				i, d.Id, d.Title))
		}
		docs[d.Id] = true
	}
	for i, p := range spec.Pins {
		id := p.Id
		if id == "" {
			id = fmt.Sprintf("pins[%d]", i)
		}
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("%s: no name; the name is the channel that survives repackaging", id))
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
	for i, r := range spec.Relations {
		id := fmt.Sprintf("relations[%d]", i)
		if r.Kind == parampb.PinRelationKind_PIN_RELATION_KIND_UNSPECIFIED {
			errs = append(errs, fmt.Errorf("%s: kind is unspecified; classify or drop", id))
		}
		// A relation asserting no bound says nothing, like a parameter with no min, typ or max.
		if r.Difference == nil || (r.Difference.Min == nil && r.Difference.Max == nil) {
			errs = append(errs, fmt.Errorf("%s: difference has no min or max", id))
		} else if r.Difference.Min != nil && r.Difference.Max != nil && r.Difference.GetMin() > r.Difference.GetMax() {
			errs = append(errs, fmt.Errorf("%s: difference min %v above max %v", id, r.Difference.GetMin(), r.Difference.GetMax()))
		}
		switch {
		case r.Prov == nil:
			errs = append(errs, fmt.Errorf("%s: no prov; every relation carries provenance", id))
		case !docs[r.Prov.DocRef]:
			errs = append(errs, fmt.Errorf("%s: prov.doc_ref %q does not resolve to a declared source doc", id, r.Prov.DocRef))
		case r.Prov.Confidence <= 0 || r.Prov.Confidence > 1:
			errs = append(errs, fmt.Errorf("%s: prov.confidence %v outside (0, 1]", id, r.Prov.Confidence))
		}
	}
	return errs
}

// UnderSpecified reports whether a parameter's value must not be treated as a plain
// comparable limit, because its condition list is not asserted complete (coverage is
// unknown or known-partial). A no-condition parameter is fine only when the source
// states none (CONDITION_COVERAGE_UNCONDITIONAL). Consumers skip or flag
// under-specified parameters and never compare against them
// (docsite/content/architecture/datasheet-layer.md#comparison-semantics).
func UnderSpecified(p *parampb.Parameter) bool {
	switch p.ConditionCoverage {
	case parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
		parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL:
		return false
	}
	return true
}

// MachineComparable reports whether a consumer may compare this parameter's value
// against an operating point automatically. The condition list must be asserted
// complete (not UnderSpecified) AND every condition structured (eq or min/max). A
// condition captured only as raw text makes the row the middle trust state, shown to
// a human next to its provenance and never auto-compared. The three states
// (docsite/content/architecture/datasheet-layer.md#comparison-semantics):
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

// structuralProblems checks the STRUCTURAL coherence of a spec's pin data: unique package and pin ids,
// numbers that resolve to a declared package with no two pins claiming one number within it, every
// Parameter.pin_refs resolving to a declared pin, and every relation's two ends resolving to two
// distinct pins. These can never be a valid work-in-progress state, unlike completenessProblems.
//
// The workbench's draft save is not gated on it, and param.LoadSet reads only *.textproto, so a draft
// cannot reach the corpus by sitting on disk. The editor mirrors these rules in TS for live feedback,
// so a change here wants the same change there.
func structuralProblems(spec *parampb.PartSpec) []error {
	var errs []error
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
	// A pin NUMBER belongs to one pin within one package. Names may repeat (the
	// multi-supply case pin binding exists for), but one terminal on two pins would make
	// the number unusable as the tie-breaking channel.
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
	}

	for _, p := range spec.Parameters {
		id := p.Symbol
		if id == "" {
			id = "a parameter"
		}
		for _, ref := range p.PinRefs {
			if !pins[ref] {
				errs = append(errs, fmt.Errorf("%s: pin_refs %q does not resolve to a declared pin", id, ref))
			}
		}
	}

	// A relation's two ends are the only part of it that can be wrong rather than unfinished.
	// The bound, kind and provenance live in completenessProblems.
	for i, r := range spec.Relations {
		if !pins[r.SubjectPinRef] {
			errs = append(errs, fmt.Errorf("relations[%d]: subject_pin_ref %q does not resolve to a declared pin", i, r.SubjectPinRef))
		}
		if !pins[r.ReferencePinRef] {
			errs = append(errs, fmt.Errorf("relations[%d]: reference_pin_ref %q does not resolve to a declared pin", i, r.ReferencePinRef))
		}
		if r.SubjectPinRef != "" && r.SubjectPinRef == r.ReferencePinRef {
			errs = append(errs, fmt.Errorf("relations[%d]: subject and reference are both %q; a pin cannot track itself", i, r.SubjectPinRef))
		}
	}
	return errs
}
