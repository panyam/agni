package param

import (
	"regexp"

	"google.golang.org/protobuf/encoding/prototext"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// MarshalSpecText renders a PartSpec as the multi-line textproto the corpus is written in, the same
// output from every build.
//
// prototext deliberately puts one or two spaces after a field name, chosen by a hash of the running
// binary, so its output cannot be relied on byte for byte. A corpus lives in git, so two builds
// re-promoting one spec would otherwise produce a diff that is nothing but whitespace. The fix is to
// collapse that one run of spaces after each line's field name; values are untouched, since a string
// value cannot span lines in textproto.
func MarshalSpecText(spec *parampb.PartSpec) ([]byte, error) {
	b, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(spec)
	if err != nil {
		return nil, err
	}
	return normalizeText(b), nil
}

// fieldSpacing matches a line's leading indent, its field name (plain or [extension]), the colon a
// scalar carries, and the run of spaces after them.
var fieldSpacing = regexp.MustCompile(`(?m)^(\s*(?:\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*):?) +`)

func normalizeText(b []byte) []byte {
	return fieldSpacing.ReplaceAll(b, []byte("$1 "))
}
