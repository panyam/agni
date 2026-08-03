package census

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureGlobs maps a format name to the committed fixtures the CI census walks. These are the
// hand-authored testdata files (plus the conformance boards/schematics) — the only design files
// CI can see (the real corpus is a separate private repo, swept by `agni census` / `make census`).
var fixtureGlobs = map[string][]string{
	"kicad-pcb": {"../kicad/testdata/*.kicad_pcb", "../cmd/agni/testdata/conformance/*.kicad_pcb"},
	"kicad-sch": {"../kicad/testdata/*.kicad_sch", "../cmd/agni/testdata/conformance/*.kicad_sch"},
	"edif":      {"../edif/testdata/*.edn", "../edif/testdata/*.eds"},
	"ipc2581":   {"../ipc2581/testdata/*.xml"},
	"xschem":    {"../xschem/testdata/*.sch", "../xschem/testdata/*.sym"},
	"geda":      {"../geda/testdata/*.sch", "../geda/testdata/*.sym"},
}

// TestFixtureCensus is the coverage guard: every construct in the committed fixtures must be
// classified in its format's manifest. An unclassified construct — e.g. a new fixture that
// introduces a source element the reader has never decided about — fails here, forcing a human to
// mark it consumed or a known drop rather than letting it be dropped silently (WS6-011).
func TestFixtureCensus(t *testing.T) {
	for _, m := range Manifests() {
		t.Run(m.Format, func(t *testing.T) {
			globs, ok := fixtureGlobs[m.Format]
			if !ok {
				t.Fatalf("no fixture globs registered for format %q", m.Format)
			}
			files := map[string][]byte{}
			for _, g := range globs {
				matches, err := filepath.Glob(g)
				if err != nil {
					t.Fatalf("glob %q: %v", g, err)
				}
				for _, f := range matches {
					b, err := os.ReadFile(f)
					if err != nil {
						t.Fatalf("read %q: %v", f, err)
					}
					files[f] = b
				}
			}
			if len(files) == 0 {
				t.Fatalf("no fixtures matched %v — the census would pass vacuously", globs)
			}
			for _, u := range m.Audit(files) {
				t.Errorf("unclassified %s construct %q (in %s): add it to the %s census manifest as consumed or a known drop",
					m.Format, u.Token, filepath.Base(u.File), m.Format)
			}
		})
	}
}

// TestManifestsWellFormed guards the manifests themselves: every entry has a non-empty reason, and
// every dropped entry that names a gap (cosmetic/analysis/latent) either cites a ticket or is a
// deliberate no-ticket backlog note — never an empty Status.
func TestManifestsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range Manifests() {
		if seen[m.Format] {
			t.Errorf("duplicate manifest for format %q", m.Format)
		}
		seen[m.Format] = true
		for tok, e := range m.Entries {
			if e.Why == "" {
				t.Errorf("%s: construct %q has no reason", m.Format, tok)
			}
			switch e.Status {
			case Consumed, DroppedCosmetic, DroppedAnalysis, DroppedLatent, DroppedByDesign:
			default:
				t.Errorf("%s: construct %q has invalid status %q", m.Format, tok, e.Status)
			}
		}
	}
}
