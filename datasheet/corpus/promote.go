package corpus

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Promotion turns a workbench DRAFT into a seeded corpus file (agni issues 747, 749).
//
// A draft is the `<stem>.partspec.json` the workbench saves beside a datasheet. It is unvalidated on
// purpose, so a half-finished transcription can always be saved, and LoadSet never reads one, so an
// incoherent draft cannot reach a check by sitting on disk. Promotion is the one step that crosses
// from draft to corpus, and it is where Validate is enforced. It belongs to the datasheet service,
// which owns the corpus and its index, rather than to the engine, which only reads specs.

// Promoted is what a successful promotion produced: the spec, its seeded text, the corpus-relative
// file name to write it to, and the index to write after it. Replaces is true when that file already
// holds the same MPN, which is the case of re-promoting a draft that was edited after its first
// promotion.
type Promoted struct {
	Spec     *parampb.PartSpec
	Text     []byte
	File     string
	Replaces bool
	Index    *Index
}

// PromoteError is a refusal. Problems carries every validation finding when the draft itself is not
// fit for the corpus, so a caller can list them all rather than one at a time.
type PromoteError struct {
	Reason   string
	Problems []param.Problem
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
// and returns what to write: the spec's file, then the index. It writes nothing; the caller owns the
// filesystem.
//
// The corpus is walked and validated whole, as LoadSet does, rather than trusted from prev. Promotion
// is rare and human-paced, and walking it means a hand edit since the last index cannot slip a
// duplicate past the rule. prev is the index currently written, or nil; it supplies the generation the
// new index advances from.
//
// It refuses, with a *PromoteError, a draft that does not parse, one Validate rejects (every problem
// listed), a corpus that does not load as it stands, and an MPN another corpus file already seeds. A
// corpus file that already holds the SAME MPN under the promoted file's name is replaced rather than
// refused.
func Promote(draft []byte, corpus fs.FS, prev *Index) (*Promoted, error) {
	spec := &parampb.PartSpec{}
	if err := protojson.Unmarshal(draft, spec); err != nil {
		return nil, &PromoteError{Reason: fmt.Sprintf("the draft is not a PartSpec: %v", err)}
	}
	if probs := param.Problems(spec); len(probs) > 0 {
		what := "the draft (it names no mpn)"
		if spec.GetMpn() != "" {
			what = fmt.Sprintf("the draft for %q", spec.GetMpn())
		}
		return nil, &PromoteError{Reason: what + " is not ready for the corpus:", Problems: probs}
	}
	ix, err := Build(corpus)
	if err != nil {
		return nil, &PromoteError{Reason: fmt.Sprintf("the corpus does not load as it stands, so nothing is promoted into it: %v", err)}
	}
	file := SpecFileName(spec.GetMpn())
	prevEntry, seeded := ix.Lookup(spec.GetMpn())
	if seeded && prevEntry.File != file {
		return nil, &PromoteError{Reason: fmt.Sprintf("the corpus already seeds %q in %s; promoting would add %s, and one MPN in two files fails every load. Edit or remove %s first", spec.GetMpn(), prevEntry.File, file, prevEntry.File)}
	}
	text, err := param.MarshalSpecText(spec)
	if err != nil {
		return nil, err
	}
	ix.upsert(Entry{MPN: spec.GetMpn(), File: file, Hash: hashOf(text)})
	ix.Generation = prev.next()
	return &Promoted{Spec: spec, Text: text, File: file, Replaces: seeded, Index: ix}, nil
}

// upsert replaces the entry for e's MPN, or adds it, keeping the entries sorted.
func (ix *Index) upsert(e Entry) {
	key := strings.ToUpper(e.MPN)
	for i := range ix.Entries {
		if strings.ToUpper(ix.Entries[i].MPN) == key {
			ix.Entries[i] = e
			return
		}
	}
	ix.Entries = append(ix.Entries, e)
	ix.sort()
}

// unsafeFileChars is anything outside a conservative portable file-name set. An MPN routinely carries
// a slash ("LM1117-3.3/NOPB"), which would otherwise turn a name into a directory.
var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// SpecFileName is the corpus file a promoted spec is written to. The name is for people: LoadSet keys a
// spec by the MPN inside the file, never by its name.
func SpecFileName(mpn string) string {
	return unsafeFileChars.ReplaceAllString(mpn, "_") + ".textproto"
}
