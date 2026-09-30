package param

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Promotion turns a workbench DRAFT into a seeded corpus file (agni issue 747).
//
// A draft is the `<stem>.partspec.json` the workbench saves beside a datasheet. It is unvalidated on
// purpose, so a half-finished transcription can always be saved, and LoadSet never reads one, so an
// incoherent draft cannot reach a check by sitting on disk. Promotion is the one step that crosses
// from draft to corpus, and it is where Validate is enforced.

// Promoted is what a successful promotion produced: the spec, its seeded text, and the corpus-relative
// file name to write it to. Replaces is true when that file already holds the same MPN, which is the
// case of re-promoting a draft that was edited after its first promotion.
type Promoted struct {
	Spec     *parampb.PartSpec
	Text     []byte
	File     string
	Replaces bool
}

// PromoteError is a refusal. Problems carries every validation finding when the draft itself is not
// fit for the corpus, so a caller can list them all rather than one at a time.
type PromoteError struct {
	Reason   string
	Problems []Problem
}

func (e *PromoteError) Error() string {
	if len(e.Problems) == 0 {
		return e.Reason
	}
	var b strings.Builder
	b.WriteString(e.Reason)
	for _, p := range e.Problems {
		fmt.Fprintf(&b, "\n  [%s] %s", p.Kind, p.Message)
	}
	return b.String()
}

// Promote checks a draft (protojson, as the workbench writes it) against the corpus it is headed for
// and returns what to write. It writes nothing: the caller owns the filesystem (C1, C22).
//
// It refuses, with a *PromoteError, a draft that does not parse, one Validate rejects (every problem
// listed), a corpus that does not load as it stands, and an MPN another corpus file already seeds,
// which is the duplicate rule LoadSet applies at load time. A corpus file that already holds the SAME
// MPN under the promoted file's name is replaced rather than refused.
func Promote(draft []byte, corpus fs.FS) (*Promoted, error) {
	spec := &parampb.PartSpec{}
	if err := protojson.Unmarshal(draft, spec); err != nil {
		return nil, &PromoteError{Reason: fmt.Sprintf("the draft is not a PartSpec: %v", err)}
	}
	if probs := Problems(spec); len(probs) > 0 {
		return nil, &PromoteError{Reason: fmt.Sprintf("the draft for %q is not ready for the corpus:", spec.GetMpn()), Problems: probs}
	}
	_, from, err := loadSet(corpus)
	if err != nil {
		return nil, &PromoteError{Reason: fmt.Sprintf("the corpus does not load as it stands, so nothing is promoted into it: %v", err)}
	}
	file := SpecFileName(spec.GetMpn())
	prev, seeded := from[strings.ToUpper(spec.GetMpn())]
	if seeded && prev != file {
		return nil, &PromoteError{Reason: fmt.Sprintf("the corpus already seeds %q in %s; promoting would add %s, and one MPN in two files fails every load. Edit or remove %s first", spec.GetMpn(), prev, file, prev)}
	}
	text, err := MarshalSpecText(spec)
	if err != nil {
		return nil, err
	}
	return &Promoted{Spec: spec, Text: text, File: file, Replaces: seeded}, nil
}

// unsafeFileChars is anything outside a conservative portable file-name set. An MPN routinely carries
// a slash ("LM1117-3.3/NOPB"), which would otherwise turn a name into a directory.
var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// SpecFileName is the corpus file a promoted spec is written to. The name is for people: LoadSet keys a
// spec by the MPN inside the file, never by its name.
func SpecFileName(mpn string) string {
	return unsafeFileChars.ReplaceAllString(mpn, "_") + ".textproto"
}

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
