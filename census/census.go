// Package census is the element-coverage guard (WS6-011). For each source format it holds a
// reviewed MANIFEST classifying every construct the format's files carry as either consumed by
// the reader or a known drop (with a reason and, where tracked, a roadmap ticket). A test walks
// the committed fixtures and fails if a construct appears that the manifest does not classify, so
// a newly-added fixture construct forces a human decision instead of being dropped silently; the
// `agni census` CLI runs the same audit over the private corpus (report-only) to surface
// real-world constructs the fixtures lack.
//
// The census asserts CLASSIFICATION coverage, not behavioral consumption: it catches "a construct
// we never decided about appeared" and (via the corpus report) "the real world has one our
// fixtures do not". Behavioral correctness stays the conformance harness's job (WS6-004); the two
// are complementary. Motivation and the seed classifications are in the private research repo's
// reader-coverage audit (docs/18).
package census

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
)

// Lookup resolves a design file to the manifest that classifies its constructs, by extension
// (and, for the ambiguous `.sch`/`.sym`, a light header sniff of data to tell xschem from gEDA
// from legacy KiCad). ok is false for a file no manifest covers. This is what `agni census`
// uses to walk a mixed corpus directory.
func Lookup(path string, data []byte) (Manifest, bool) {
	var format string
	switch strings.ToLower(filepath.Ext(path)) {
	case ".kicad_pcb":
		format = "kicad-pcb"
	case ".kicad_sch":
		format = "kicad-sch"
	case ".edn", ".eds":
		format = "edif"
	case ".xml", ".cvg":
		format = "ipc2581"
	case ".sch", ".sym":
		format = sniffSch(data)
	}
	for _, m := range Manifests() {
		if m.Format == format {
			return m, true
		}
	}
	return Manifest{}, false
}

// sniffSch distinguishes the header of the three formats that share `.sch`/`.sym`: xschem files
// name "xschem" in their version line, gEDA's version line is `v <int> <int>`, and a legacy
// KiCad s-expr schematic opens with "(kicad_sch". Returns "" when none matches.
func sniffSch(data []byte) string {
	head := data
	if len(head) > 256 {
		head = head[:256]
	}
	if bytes.Contains(head, []byte("xschem")) {
		return "xschem"
	}
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if bytes.HasPrefix(trimmed, []byte("(kicad_sch")) {
		return "kicad-sch"
	}
	if bytes.HasPrefix(trimmed, []byte("v ")) && len(trimmed) > 2 && trimmed[2] >= '0' && trimmed[2] <= '9' {
		return "geda"
	}
	return ""
}

// Status is how a manifest classifies one source construct.
type Status string

const (
	// Consumed: the reader transforms this construct into the IR.
	Consumed Status = "consumed"
	// DroppedCosmetic: not read, and the only loss is render fidelity.
	DroppedCosmetic Status = "dropped-cosmetic"
	// DroppedAnalysis: not read, and its absence blocks a check or diff (a rule can't be written).
	DroppedAnalysis Status = "dropped-analysis"
	// DroppedLatent: not read, and its absence produces WRONG data when the construct is present
	// (a correctness hole), even if no current corpus/fixture exercises it.
	DroppedLatent Status = "dropped-latent"
	// DroppedByDesign: deliberately not read and never will be (editor/tool metadata, 3D models).
	DroppedByDesign Status = "dropped-by-design"
)

// Entry is one construct's classification. Why is a one-line rationale; Ticket is the roadmap id
// that tracks closing the gap ("" when none applies, e.g. a Consumed or DroppedByDesign entry).
type Entry struct {
	Status Status
	Why    string
	Ticket string
}

// Manifest classifies a format's constructs. Keys are the tokens Enumerate emits: s-expr head
// atoms, XML element names, line-type chars, or "@key" for a line-format attribute key; the
// numeric-head sentinel is NumberToken.
type Manifest struct {
	Format  string // display name, e.g. "kicad-pcb"
	Kind    Kind   // which extractor enumerates this format's files
	Entries map[string]Entry
}

// Unclassified is one construct found in a file that the manifest does not list.
type Unclassified struct {
	Token string
	File  string
}

// Audit enumerates the constructs in each file (via the manifest's extractor Kind) and returns the
// tokens absent from the manifest, sorted by token then file. An empty result means full
// classification coverage. filesByPath maps a display path to its raw bytes.
func (m Manifest) Audit(filesByPath map[string][]byte) []Unclassified {
	var out []Unclassified
	for path, data := range filesByPath {
		for _, tok := range Enumerate(data, m.Kind) {
			if _, ok := m.Entries[tok]; !ok {
				out = append(out, Unclassified{Token: tok, File: path})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Token != out[j].Token {
			return out[i].Token < out[j].Token
		}
		return out[i].File < out[j].File
	})
	return out
}
