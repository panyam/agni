package service

import (
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// This file is the review manifest's wire <-> value conversion, the manifest half of what overlay.go
// does for a naming convention (WS3-102, generalized by WS9-050). It lives here, not in core/review,
// because the core must not depend on the wire types; this package already sits between the two.
//
// The conversions are total and lossless in both directions, and TestManifestProtoRoundTrip pins
// that: a Binding field added to the Go struct without a matching proto field fails the round trip
// rather than quietly dropping off the wire. A dropped binding field is the worst kind of bug here
// because the item still SCORES — it just scores against a check nobody asked for, or against no
// check at all while reading not-automated.

// ManifestFromProto converts a wire manifest to the engine's value. A nil message yields the zero
// Manifest rather than panicking: an absent manifest is a validation error the caller reports with a
// message, not a crash.
//
// It does NOT validate. Conversion and validation are separate so a caller decides when to pay for
// the query compilation that validation performs, and so a validation failure names the manifest
// rather than the conversion. Every service path that converts an inbound manifest validates it.
func ManifestFromProto(p *checkspb.ReviewManifest) review.Manifest {
	if p == nil {
		return review.Manifest{}
	}
	m := review.Manifest{Name: p.GetName()}
	for _, a := range p.GetAreas() {
		area := review.Area{Name: a.GetName()}
		for _, it := range a.GetItems() {
			area.Items = append(area.Items, review.Item{
				ID:          it.GetId(),
				Title:       it.GetTitle(),
				Description: it.GetDescription(),
				Note:        it.GetNote(),
				Binding:     bindingFromProto(it.GetBinding()),
			})
		}
		m.Areas = append(m.Areas, area)
	}
	return m
}

// ManifestProto converts an engine manifest to its wire form, for a caller that obtained one from its
// own source (the CLI reads the YAML file the user named with --checklist) and now has to send it.
func ManifestProto(m review.Manifest) *checkspb.ReviewManifest {
	p := &checkspb.ReviewManifest{Name: m.Name}
	for _, a := range m.Areas {
		area := &checkspb.ManifestArea{Name: a.Name}
		for _, it := range a.Items {
			area.Items = append(area.Items, &checkspb.ManifestItem{
				Id:          it.ID,
				Title:       it.Title,
				Description: it.Description,
				Note:        it.Note,
				Binding:     bindingProto(it.Binding),
			})
		}
		p.Areas = append(p.Areas, area)
	}
	return p
}

func bindingFromProto(p *checkspb.ItemBinding) review.Binding {
	b := review.Binding{
		Rule:           p.GetRule(),
		Tag:            p.GetTag(),
		Profile:        p.GetProfile(),
		Requirement:    p.GetRequirement(),
		AppliesToClass: p.GetAppliesToClass(),
	}
	if q := p.GetQuery(); q != nil {
		b.Query = &review.QueryBinding{
			Match: q.GetMatch(), Subject: q.GetSubject(), Kind: q.GetKind(),
			Message: q.GetMessage(), Severity: q.GetSeverity(), ParamSymbol: q.GetParamSymbol(),
		}
	}
	if pr := p.GetPresent(); pr != nil {
		b.Present = &review.PresentBinding{Class: pr.GetClass()}
	}
	if s := p.GetScope(); s != nil {
		b.Scope = review.ScopeBinding{Profiles: s.GetProfiles()}
	}
	return b
}

// bindingProto mirrors bindingFromProto. The three sub-messages are emitted only when the binding
// actually carries them, because presence is MEANINGFUL for two of them: a non-nil Present is a
// present binding (an empty one fails validation for its missing class), and a non-nil Query counts
// toward the mutually-exclusive binding limit. Materializing an empty message would turn every plain
// rule item into an item with three extra bindings.
func bindingProto(b review.Binding) *checkspb.ItemBinding {
	p := &checkspb.ItemBinding{
		Rule:           b.Rule,
		Tag:            b.Tag,
		Profile:        b.Profile,
		Requirement:    b.Requirement,
		AppliesToClass: b.AppliesToClass,
	}
	if b.Query != nil {
		p.Query = &checkspb.ManifestQuery{
			Match: b.Query.Match, Subject: b.Query.Subject, Kind: b.Query.Kind,
			Message: b.Query.Message, Severity: b.Query.Severity, ParamSymbol: b.Query.ParamSymbol,
		}
	}
	if b.Present != nil {
		p.Present = &checkspb.ManifestPresent{Class: b.Present.Class}
	}
	if len(b.Scope.Profiles) > 0 {
		p.Scope = &checkspb.ManifestScope{Profiles: b.Scope.Profiles}
	}
	return p
}
