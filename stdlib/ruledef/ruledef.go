// Package ruledef reads and writes rule DEFINITIONS, the declarative source a rule compiles from
// (WS3-103). It is the one form shared by the three declarative sources, a check.Spec, a datalog
// query and an interface profile. See
// docsite/content/architecture/checks-contract.md#the-other-half-rule-definitions.
//
// It sits ABOVE all three because core/query imports core/check and stdlib/profiles imports both, so
// a converter for all three cannot live in core/check without a cycle. Each source owns the wire form
// of its OWN body (check.SpecProto, query.QueryProto, profiles.ProfileProto). This package holds only
// the join, meaning which body a definition carries and how to compile it back into rules.
//
// A new NODE TYPE is caught where a type switch covers the vocabulary, but a new FIELD on a
// body the converter copies by hand is silently not copied. Profile.HostClass was dropped that way
// until the round-trip guard in stdlib/profiles caught it, so each body's wire form owes a
// deep-equality round-trip test as well as a converter.
//
// check.Rule itself is NOT serializable, because its Eval closure has no wire form. A rule with a
// hand-written Go Eval and no declarative twin is outside this contract.
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

// Compile turns one definition back into the rules it declares. It returns a SLICE because a spec
// and a query each yield one rule and an interface profile yields one per requirement.
//
// Every failure mode is an error, never a panic and never a silent drop. A definition read from
// outside this build can name a fact, function, relation or requirement type that does not exist
// here, and each would otherwise produce a rule that never fires.
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
		r, err := query.RuleFromQuery(query.FindingQuery{
			Rule:        meta,
			Query:       q,
			Kind:        b.Query.GetKind(),
			SubjectVar:  b.Query.GetSubjectVar(),
			PinVar:      b.Query.GetPinVar(),
			Message:     b.Query.GetMessage(),
			ParamSymbol: b.Query.GetParamSymbol(),
			ContextVars: query.ContextVarsFromProto(b.Query.GetContextVars()),
		})
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", meta.Name, err)
		}
		return []*check.Rule{r}, nil

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
// rather than skipping it, so a deck never loads with one rule quietly missing.
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

// Source compiles a deck into a check.RuleSource, so definitions read from a document join a catalog
// the way a Go-registered suite does. It is the data-driven counterpart to check.RegisterSource.
func Source(deck *checkspb.RuleDeck) (check.RuleSource, error) {
	rules, err := CompileDeck(deck)
	if err != nil {
		return nil, err
	}
	return check.NewSource(deck.GetName(), rules), nil
}

// Marshal encodes a deck as indented protojson, since people author, review and diff rule decks.
func Marshal(deck *checkspb.RuleDeck) ([]byte, error) {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(deck)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Parse decodes a deck without compiling it, so a caller can inspect or re-emit a deck holding a
// definition this build cannot run.
func Parse(b []byte) (*checkspb.RuleDeck, error) {
	deck := &checkspb.RuleDeck{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, deck); err != nil {
		return nil, fmt.Errorf("rule deck: %w", err)
	}
	return deck, nil
}

// requirementsRegistered rejects a profile whose requirements this build cannot run, meaning an
// unknown requirement type or params the type's validator refuses. profiles.Compile panics on either,
// which suits a Go literal, but from a document it is an input error. It delegates to
// profiles.ValidateRequirements so a deck and a YAML profile agree on what is valid (WS3-047).
func requirementsRegistered(p profiles.Profile) error {
	return profiles.ValidateRequirements(p)
}

func deckName(deck *checkspb.RuleDeck) string {
	if n := deck.GetName(); n != "" {
		return "deck " + n
	}
	return "deck"
}
