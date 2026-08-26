package formats

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The locators whose source_file this rewrites, matched by full proto name rather than by Go type so
// one walk covers both the netlist IR and the geometry sidecar without this file importing either.
const (
	irProvenanceName   = "agni.v1.ir.Provenance"
	geomProvenanceName = "agni.v1.geom.Provenance"
	sourceFileField    = "source_file"
)

// relocateSources rewrites every Provenance.source_file anywhere in a message tree through name.
//
// It walks by REFLECTION rather than by visiting the types that carry a locator today. Nineteen IR
// messages carry one and the geometry sidecar carries its own, so a hand-written walk would be long,
// and a twentieth message gaining one is an ordinary schema change nobody would think to come back
// here for. Such a walk keeps compiling and quietly stops covering the new node, which is the
// failure this repo spends most of its tests on: the output still looks like an answer.
//
// A nil name is the identity, so a loader that sets nothing behaves exactly as it did before.
func relocateSources(m proto.Message, name func(string) string) {
	if m == nil || name == nil {
		return
	}
	relocate(m.ProtoReflect(), name)
}

func relocate(m protoreflect.Message, name func(string) string) {
	if !m.IsValid() {
		return
	}
	switch m.Descriptor().FullName() {
	case irProvenanceName, geomProvenanceName:
		if fd := m.Descriptor().Fields().ByName(sourceFileField); fd != nil && m.Has(fd) {
			if s := m.Get(fd).String(); s != "" {
				m.Set(fd, protoreflect.ValueOfString(name(s)))
			}
		}
	}
	// Range visits populated fields only, which is the set that can hold a locator.
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			// No schema here has a message-valued map that reaches a locator, so nothing exercises this
			// branch today. It is here because a map is the one field kind a walk silently skips.
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
					relocate(mv.Message(), name)
					return true
				})
			}
		case fd.IsList():
			if fd.Message() != nil {
				l := v.List()
				for i := 0; i < l.Len(); i++ {
					relocate(l.Get(i).Message(), name)
				}
			}
		case fd.Message() != nil:
			relocate(v.Message(), name)
		}
		return true
	})
}
