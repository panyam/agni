// Package ruledef reads and writes rule DEFINITIONS: the declarative source a rule compiles from
// (WS3-103).
//
// It exists because the engine has three declarative rule sources with three compilers — a
// check.Spec, a datalog query, an interface profile — and, until now, no common form. That made
// profile YAML, datalog text, a future DSL, and a foreign rule deck four parallel paths rather than
// four front-ends onto one target. This package is that target.
//
// It has to sit ABOVE all three sources rather than inside any one of them: core/query imports
// core/check, and stdlib/profiles imports both, so a package that converts all three cannot live in
// core/check without a cycle. Each source owns the wire form of its OWN body — check.SpecProto,
// query.QueryProto, profiles.ProfileProto — so a node added to a closed vocabulary and not given a
// wire form is a compile error next to the vocabulary. What lives here is only the join: which body a
// definition carries, and how to compile it back into rules.
//
// What is deliberately NOT serializable is check.Rule itself. A Rule carries an Eval closure, and a Go
// func has no wire form. The serializable artifact is the SOURCE, and compiling is exactly the step
// that produces the non-serializable part — so a rule with a hand-written Go Eval and no declarative
// twin is outside this contract by design rather than a gap in it.
package ruledef

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/stdlib/profiles"
	"google.golang.org/protobuf/encoding/protojson"
)

// SpecDef assembles a definition from a rule's metadata and its Spec body.
func SpecDef(meta check.Rule, spec check.Spec) *checkspb.RuleDef {
	return &checkspb.RuleDef{Body: &checkspb.RuleDef_Spec{Spec: &checkspb.SpecRule{
		Meta: check.RuleMetaProto(meta),
		Body: check.SpecProto(spec),
	}}}
}

// QueryDef assembles a definition from a datalog rule declaration.
func QueryDef(fq query.FindingQuery) *checkspb.RuleDef {
	qr := query.FindingQueryProto(fq)
	qr.Meta = check.RuleMetaProto(fq.Rule)
	return &checkspb.RuleDef{Body: &checkspb.RuleDef_Query{Query: qr}}
}

// ProfileDef assembles a definition from an interface profile.
func ProfileDef(p profiles.Profile) *checkspb.RuleDef {
	return &checkspb.RuleDef{Body: &checkspb.RuleDef_Profile{Profile: profiles.ProfileProto(p)}}
}

// Compile turns one definition back into the rules it declares.
//
// It returns a SLICE because a definition is not always one rule: a spec and a query each yield one,
// an interface profile yields one per requirement. That asymmetry is the profile mechanism working as
// intended — a single declaration standing in for a family of near-identical checks — so the signature
// admits it rather than forcing every caller to pretend otherwise.
//
// Every failure mode is an error, never a panic and never a silent drop. A definition read from
// outside this build can name a fact, a function, a relation, or a requirement type that does not
// exist here, and each of those would otherwise produce a rule that quietly never fires — which reads
// exactly like a design with nothing wrong with it.
func Compile(def *checkspb.RuleDef) ([]*check.Rule, error) {
	switch b := def.GetBody().(type) {
	case *checkspb.RuleDef_Spec:
		meta := check.RuleMetaFromProto(b.Spec.GetMeta())
		spec, err := check.SpecFromProto(b.Spec.GetBody())
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", meta.Name, err)
		}
		return []*check.Rule{spec.Rule(meta)}, nil

	case *checkspb.RuleDef_Query:
		meta := check.RuleMetaFromProto(b.Query.GetMeta())
		q, err := query.QueryFromProto(b.Query.GetQuery())
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", meta.Name, err)
		}
		return []*check.Rule{query.RuleFromQuery(query.FindingQuery{
			Rule:        meta,
			Query:       q,
			Kind:        b.Query.GetKind(),
			SubjectVar:  b.Query.GetSubjectVar(),
			PinVar:      b.Query.GetPinVar(),
			Message:     b.Query.GetMessage(),
			ParamSymbol: b.Query.GetParamSymbol(),
		})}, nil

	case *checkspb.RuleDef_Profile:
		p := profiles.ProfileFromProto(b.Profile)
		if err := profiles.Validate(p); err != nil {
			return nil, err
		}
		if err := requirementsRegistered(p); err != nil {
			return nil, err
		}
		return profiles.Compile(p), nil
	}
	return nil, fmt.Errorf("rule definition is empty (no body set)")
}

// CompileDeck compiles every definition in a deck, in order. It stops at the first bad definition
// rather than skipping it: a deck that loads with one rule quietly missing is a catalog that looks
// complete and is not.
func CompileDeck(deck *checkspb.RuleDeck) ([]*check.Rule, error) {
	var out []*check.Rule
	for i, def := range deck.GetRules() {
		rules, err := Compile(def)
		if err != nil {
			return nil, fmt.Errorf("%s: definition #%d: %w", deckName(deck), i+1, err)
		}
		out = append(out, rules...)
	}
	return out, nil
}

// Source compiles a deck into a check.RuleSource, so a set of definitions read from a document joins a
// catalog exactly the way a Go-registered suite does. This is the data seam beside check.RegisterSource's
// runtime seam: a rule source that today must be Go code linked into the binary can be a document.
func Source(deck *checkspb.RuleDeck) (check.RuleSource, error) {
	rules, err := CompileDeck(deck)
	if err != nil {
		return nil, err
	}
	return check.NewSource(deck.GetName(), rules), nil
}

// Marshal encodes a deck as indented protojson. A rule deck is authored, reviewed, and diffed by
// people, so a text encoding is worth more than a compact one.
func Marshal(deck *checkspb.RuleDeck) ([]byte, error) {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(deck)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Parse decodes a deck. It does not compile: reading and judging are separate so a caller can inspect
// or re-emit a deck holding a definition this build cannot run.
func Parse(b []byte) (*checkspb.RuleDeck, error) {
	deck := &checkspb.RuleDeck{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, deck); err != nil {
		return nil, fmt.Errorf("rule deck: %w", err)
	}
	return deck, nil
}

// requirementsRegistered rejects a profile declaring a requirement type this build has no compiler
// for, naming what IS available. profiles.Compile panics on one (it is a programming error for a Go
// literal); arriving from a document it is an input error, and an unknown type silently skipped would
// mean a declared check that never runs.
func requirementsRegistered(p profiles.Profile) error {
	known := profiles.RequirementTypes()
	for _, r := range p.Requirements {
		found := false
		for _, k := range known {
			if k == r.Type {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("profile %q: unknown requirement type %q (known: %v)", p.Name, r.Type, known)
		}
	}
	return nil
}

func deckName(deck *checkspb.RuleDeck) string {
	if n := deck.GetName(); n != "" {
		return "deck " + n
	}
	return "deck"
}
