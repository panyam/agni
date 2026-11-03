package profiles

import (
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// This file is the interface-profile half of the rule-definition contract (WS3-103). Since agni issue
// 827 ProfileDef is also the schema of a profile YAML file, and Parse binds that file through yamlpb.
// A Profile is already a pure declaration (name, signals, an optional declared host, and a list of
// requirement references), so this is a field mapping.
//
// Validation is not duplicated here, since a decoded profile goes through the same exported Validate
// a YAML one does. If a second authoring route had its own idea of validity, an unsound profile would
// eventually get in that way. Validate catches two things, an over-broad matcher and a completeness
// requirement with no anchor. Each produces a rule that cannot fire rather than one that errors.

// ProfileProto encodes an interface profile as its wire form.
func ProfileProto(p Profile) *checkspb.ProfileDef {
	out := &checkspb.ProfileDef{Name: p.Name, Instance: p.Instance}
	if p.HostAttrKey != "" || p.HostAttrVal != "" || p.HostClass != "" {
		out.Host = &checkspb.ProfileHost{Attr: p.HostAttrKey, Value: p.HostAttrVal, Class: p.HostClass}
	}
	for _, s := range p.Signals {
		out.Signals = append(out.Signals, signalProto(s))
	}
	for _, s := range p.Excludes {
		out.Excludes = append(out.Excludes, signalProto(s))
	}
	for _, r := range p.Requirements {
		out.Requirements = append(out.Requirements, &checkspb.ProfileRequirement{Type: r.Type, Params: r.Params})
	}
	return out
}

// ProfileFromProto decodes an interface profile. Callers that took it from outside this build should
// run Validate before compiling, as the deck reader does.
func ProfileFromProto(p *checkspb.ProfileDef) Profile {
	out := Profile{
		Name:        p.GetName(),
		HostAttrKey: p.GetHost().GetAttr(),
		HostAttrVal: p.GetHost().GetValue(),
		HostClass:   p.GetHost().GetClass(),
		Instance:    p.GetInstance(),
	}
	for _, s := range p.GetSignals() {
		out.Signals = append(out.Signals, signalFromProto(s))
	}
	for _, s := range p.GetExcludes() {
		out.Excludes = append(out.Excludes, signalFromProto(s))
	}
	for _, r := range p.GetRequirements() {
		out.Requirements = append(out.Requirements, Requirement{Type: r.GetType(), Params: r.GetParams()})
	}
	return out
}

func signalProto(s Signal) *checkspb.ProfileSignal {
	return &checkspb.ProfileSignal{
		Name:   s.Name,
		Prefix: s.Prefix,
		Suffix: s.Suffix,
		Glob:   s.Glob,
		Regex:  s.Regex,
		Pullup: s.PullUp,
		Anchor: s.Anchor,
	}
}

func signalFromProto(s *checkspb.ProfileSignal) Signal {
	return Signal{
		Name:   s.GetName(),
		Prefix: s.GetPrefix(),
		Suffix: s.GetSuffix(),
		Glob:   s.GetGlob(),
		Regex:  s.GetRegex(),
		PullUp: s.GetPullup(),
		Anchor: s.GetAnchor(),
	}
}
