package check

import (
	"fmt"
	"strings"
)

// SubjectParam spells an entity the way a scoped check names it (agni issue 897): `net:NAME`,
// `component:REF`, or `pin:REF.PIN`. `agni check --subject` reads this spelling, a verdict link
// carries it as `subject=`, and the viewer parses it back, so a link asks the same scoped question
// the CLI does. It is "" for a kind no scope covers (a bus, an endpoint, a symbol).
//
// The viewer's reader of this spelling is held to this one by core/check/testdata/subject_param_twin.json.
func SubjectParam(e Entity) string {
	switch e.Kind {
	case KindNet, KindComponent:
		return e.Kind + ":" + e.Ref
	case KindPin:
		if e.Pin == "" {
			return ""
		}
		return KindPin + ":" + e.Ref + "." + e.Pin
	}
	return ""
}

// SubjectParams spells each entity a scope can name, in order, leaving out the ones it cannot.
func SubjectParams(es []Entity) []string {
	var out []string
	for _, e := range es {
		if p := SubjectParam(e); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseSubjectParam reads SubjectParam's spelling back. The kind ends at the first colon, so a net
// name may hold colons, and a pin's ref and designator split at the LAST dot, so `pin:J1.A.1` is
// pin 1 of J1.A. A spelling naming no entity a scope covers is an error.
func ParseSubjectParam(s string) (Entity, error) {
	kind, ref, ok := strings.Cut(s, ":")
	if !ok || ref == "" {
		return Entity{}, fmt.Errorf("%q: want net:NAME, component:REF or pin:REF.PIN", s)
	}
	switch kind {
	case KindNet:
		return NetNameEntity(ref), nil
	case KindComponent:
		return ComponentEntity(ref), nil
	case KindPin:
		i := strings.LastIndex(ref, ".")
		if i <= 0 || i == len(ref)-1 {
			return Entity{}, fmt.Errorf("%q: a pin is pin:REF.PIN, such as pin:U7.3", s)
		}
		return PinEntity(ref[:i], ref[i+1:]), nil
	}
	return Entity{}, fmt.Errorf("%q: kind %q is not one a rule answers for (want net, component or pin)", s, kind)
}
