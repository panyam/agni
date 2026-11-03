package formats

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// relocateSources rewrites the source_file of these locators. It matches them by full proto name
// rather than by Go type. So one walk covers both the netlist IR and the geometry sidecar, and this
// file imports neither.
const (
	irProvenanceName   = "agni.v1.ir.Provenance"
	geomProvenanceName = "agni.v1.geom.Provenance"
	sourceFileField    = "source_file"
)

// relocateSources rewrites every Provenance.source_file anywhere in a message tree through name.
//
// It walks by REFLECTION rather than by visiting the types that hold a locator. Over twenty IR
// messages hold one, and the geometry sidecar has its own. Adding a locator to a message is an
// ordinary schema change, and nobody would think to come back here for it. A hand-written walk would
// keep compiling and quietly stop covering the new node, and its output would still look complete.
//
// A nil name is the identity, so a loader that sets nothing leaves every source_file as read.
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
			// branch today. It exists because a map is the one field kind a walk silently skips.
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
