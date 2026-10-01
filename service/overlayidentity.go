package service

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"google.golang.org/protobuf/proto"
)

// An overlay's identity tells a caller two runs were configured the SAME way, for a cache of analysis
// results (agni issue 390).
//
// It is built from the inputs an overlay was composed FROM, never from the composed value. `Overlay`
// carries a `*classify.Lexicon`, compiled `check.RuleSource` values and a `param.ParamProvider`, none
// of which has a canonical serialization. A hand-written serializer would silently stop covering a
// field added later, and a cache keyed on it would serve wrong answers. The inputs are protos and
// content digests, which have canonical forms already.
//
// REFUSING IS PART OF THE CONTRACT. An input nothing can identify (config a resolver read without
// reporting what it read) makes Identity return false rather than a value covering less than it
// appears to.

// overlayID accumulates the labeled inputs one overlay was composed from, in composition order.
type overlayID struct {
	parts []overlayIDPart
}

type overlayIDPart struct {
	label string
	data  []byte
	// unknown marks an input that contributed to the overlay and could not be identified. One is
	// enough to make the whole identity refuse.
	unknown bool
}

func (id *overlayID) add(label string, data []byte) {
	if id == nil {
		return
	}
	id.parts = append(id.parts, overlayIDPart{label: label, data: data})
}

// addProto folds a message in by its deterministic encoding. A nil message is a real input value
// (no project, no request config) and folds in as an empty one.
func (id *overlayID) addProto(label string, m proto.Message) {
	if id == nil {
		return
	}
	if m == nil || !m.ProtoReflect().IsValid() {
		id.add(label, nil)
		return
	}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		id.unidentifiable(label)
		return
	}
	id.add(label, b)
}

// addDigest folds in config a resolver read. An empty digest means the resolver did not report what
// it read, so what it contributed cannot be identified.
func (id *overlayID) addDigest(label, digest string) {
	if id == nil {
		return
	}
	if digest == "" {
		id.unidentifiable(label)
		return
	}
	id.add(label, []byte(digest))
}

func (id *overlayID) unidentifiable(label string) {
	if id == nil {
		return
	}
	id.parts = append(id.parts, overlayIDPart{label: label, unknown: true})
}

// value hashes the accumulated inputs, and reports false when any of them was unidentifiable.
//
// Each part is folded in with its label and an explicit length, so two different compositions cannot
// hash alike by running their bytes together. Without the lengths, ("ab", "c") and ("a", "bc") would
// be one string.
func (id *overlayID) value() (string, bool) {
	if id == nil {
		return "", false
	}
	h := sha256.New()
	var n [8]byte
	for _, p := range id.parts {
		if p.unknown {
			return "", false
		}
		binary.BigEndian.PutUint64(n[:], uint64(len(p.label)))
		h.Write(n[:])
		h.Write([]byte(p.label))
		binary.BigEndian.PutUint64(n[:], uint64(len(p.data)))
		h.Write(n[:])
		h.Write(p.data)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), true
}

// inherit folds in an overlay this one is layered on top of.
//
// A zero overlay contributes nothing and is identified as such. A non-zero one with no identity of its
// own is unidentifiable, because it holds only composed values and this scheme does not hash those.
func (id *overlayID) inherit(label string, o Overlay) {
	if id == nil {
		return
	}
	switch {
	case o.isZero():
		id.add(label, nil)
	case o.id == nil:
		id.unidentifiable(label)
	default:
		v, ok := o.id.value()
		if !ok {
			id.unidentifiable(label)
			return
		}
		id.add(label, []byte(v))
	}
}

// isZero reports whether this overlay contributes nothing to a run. It asks about the tiers an
// overlay can carry rather than comparing against a zero struct, because the unexported bookkeeping
// fields are not part of what it contributes.
func (o Overlay) isZero() bool {
	return o.Lexicon == nil && len(o.Sources) == 0 && o.Specs == nil && len(o.SymbolPaths) == 0 && len(o.Library) == 0
}

// Identity returns a value that differs whenever anything this overlay was composed FROM differs,
// and reports false when the composition cannot be fully identified.
//
// It is for a caller reusing work only under the same configuration, such as a check-result cache.
// Equal identities mean equal inputs. Different identities may still behave identically, since this
// compares inputs rather than outcomes, and that is the safe direction to be wrong in.
//
// FALSE IS NOT AN ERROR. It means an input could not be identified, most often config a resolver
// read without reporting a digest. A caller must then do the work rather than reuse any, and must
// never treat the empty string as a key.
func (o Overlay) Identity() (string, bool) { return o.id.value() }
