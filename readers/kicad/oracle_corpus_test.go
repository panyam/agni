package kicad

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestOracleCorpus is the reader cross-check against real boards, and the oracle is free: every
// KiCad project carries both the schematic and the .kicad_pcb, and the board file holds the netlist
// KiCad itself resolved. So we read a design our way, read KiCad's answer for the same design, and
// compare which pins share a net.
//
// The boards are third-party designs under their own licences and live in panyam/agni-samples. They
// need the BOTH-VIEWS tarball, which is 19MB against tutorial-board's 3MB and deliberately not in the
// gate's default fetch, so this runs as its own target: `make oracle` fetches the corpus and then
// runs exactly this test. `make browser-test` was the other suite outside the gate for a comparable
// reason, until PR 629 moved it inside. This one stays out: a browser is a fixed one-time
// install, where the corpus is 19MB fetched per cache miss.
//
// It asserts against a COMMITTED BASELINE of the disagreements rather than demanding zero, because
// several reader defects are still open and a test that has never passed teaches nothing. The
// baseline lists the reference nets we get wrong BY NAME, so a fix shrinks it, a regression grows it,
// and a change that swaps one defect for another of equal size is still caught.
//
// What the corpus does NOT cover is worth stating, since two boards is not a survey. Neither of them
// crosses a sheet boundary with a bus VECTOR, so this test does not move when the agni issue 561 fix
// is reverted — TestBusVectorCrossesSheetBoundary is that guard, on a fixture with kicad-cli's own
// answer beside it. The boards that would cover it (kit-dev-coldfire, video) ship with no licence, so
// they cannot be redistributed; run the sweep against a local KiCad demos directory for those.
// Breaking the scalar sheet-pin join moves jetson by 27 lines, which is what red-checked this file.
func TestOracleCorpus(t *testing.T) {
	if os.Getenv("AGNI_ORACLE") == "" {
		t.Skip("not the default suite: run `make oracle`, which fetches the both-views corpus first")
	}
	const dir = "../../tools/samples"
	boards, err := filepath.Glob(filepath.Join(dir, "boards", "*"))
	if err != nil || len(boards) == 0 {
		// Fatal, never skipped: reached only via `make oracle`, which has already fetched.
		t.Fatalf("no boards under %s/boards, run `make samples-oracle` (glob err %v)", dir, err)
	}
	baseline := readBaseline(t)
	update := os.Getenv("AGNI_ORACLE_UPDATE") != ""
	fresh := map[string][]string{}
	seen := map[string]bool{}
	for _, b := range boards {
		name := filepath.Base(b)
		t.Run(name, func(t *testing.T) {
			seen[name] = true
			got := crossCheckBoard(t, b)
			fresh[name] = got
			if update {
				return
			}
			if diff := lineDiff(baseline[name], got); diff != "" {
				t.Errorf("oracle disagreements changed for %s.\n%s\n\nIf this is an improvement, rerun with AGNI_ORACLE_UPDATE=1 to rewrite %s, and say in the commit what moved.",
					name, diff, baselinePath)
			}
		})
	}
	if update {
		writeBaseline(t, fresh)
		t.Logf("rewrote %s", baselinePath)
		return
	}
	for name := range baseline {
		if !seen[name] {
			t.Errorf("baseline names board %q, which is not in the corpus", name)
		}
	}
}

// baselinePath is deliberately NOT under testdata/. The docsite run specs mount that directory as a
// fixture and stamp each capture with a hash of every tracked file in it, so a baseline living there
// would restamp five tutorial captures every time a reader fix shrinks it — coupling two things that
// have nothing to do with each other.
const baselinePath = "oracle_corpus.baseline"

// crossCheckBoard reads one board both ways and returns its disagreement lines, sorted. A board
// whose two halves cannot both be read reports that as its single line, so the corpus records it
// rather than passing by silently skipping.
func crossCheckBoard(t *testing.T, dir string) []string {
	t.Helper()
	schs, _ := filepath.Glob(filepath.Join(dir, "*.kicad_sch"))
	var root, pcb string
	for _, s := range schs {
		if p := strings.TrimSuffix(s, ".kicad_sch") + ".kicad_pcb"; fileExists(p) {
			root, pcb = s, p
			break
		}
	}
	if root == "" {
		// The schematics-only tarball has no copper, so there is nothing to cross-check against.
		t.Fatalf("%s has no .kicad_sch with a sibling .kicad_pcb; run `make samples-oracle`", dir)
	}
	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(root)
	open := func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join(base, rel)) }
	openSym := func(nick string) ([]byte, error) { return os.ReadFile(filepath.Join(base, nick+".kicad_sym")) }
	d, _, err := ReadSchematicHierarchyNetsWithSymbols(root, content, open, openSym)
	if err != nil {
		return []string{"schematic does not read: " + err.Error()}
	}
	f, err := os.Open(pcb)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	board, err := Read(f, pcb)
	if err != nil {
		return []string{"board does not read: " + errKind(err)}
	}
	out := disagreements(pinNets(d), pinNets(board))
	sort.Strings(out)
	return out
}

// errKind reduces a read error to its stable prefix. The message carries a line number, which is
// part of what makes the error useful and is exactly the wrong thing to freeze into a baseline.
func errKind(err error) string {
	s := err.Error()
	if i := strings.Index(s, " at line "); i >= 0 {
		return s[:i]
	}
	return s
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

// readBaseline parses the committed expectations: "## <board>" sections of one disagreement per
// line, '#' comments elsewhere.
func readBaseline(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	out := map[string][]string{}
	cur := ""
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			cur = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			out[cur] = nil
		case strings.HasPrefix(line, "#"), strings.TrimSpace(line) == "":
		case cur != "":
			out[cur] = append(out[cur], strings.TrimSpace(line))
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// lineDiff reports the lines that appeared and disappeared, capped so one broad regression does not
// bury the summary.
func lineDiff(want, got []string) string {
	in := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	w, g := in(want), in(got)
	var added, gone []string
	for _, x := range got {
		if !w[x] {
			added = append(added, x)
		}
	}
	for _, x := range want {
		if !g[x] {
			gone = append(gone, x)
		}
	}
	if len(added) == 0 && len(gone) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %d new disagreement(s), %d resolved (baseline %d, now %d)",
		len(added), len(gone), len(want), len(got))
	show := func(tag string, xs []string) {
		sort.Strings(xs)
		for i, x := range xs {
			if i == 12 {
				fmt.Fprintf(&b, "\n  %s ... and %d more", tag, len(xs)-12)
				break
			}
			fmt.Fprintf(&b, "\n  %s %s", tag, x)
		}
	}
	show("+", added)
	show("-", gone)
	return b.String()
}

// writeBaseline regenerates the committed expectations. Sorted throughout so a rewrite that changes
// nothing produces no diff.
func writeBaseline(t *testing.T, got map[string][]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(baselineHeader)
	var names []string
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "\n## %s\n", n)
		lines := append([]string(nil), got[n]...)
		sort.Strings(lines)
		for _, l := range lines {
			fmt.Fprintf(&b, "%s\n", l)
		}
	}
	if err := os.WriteFile(baselinePath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

const baselineHeader = `# Where our KiCad read still disagrees with KiCad, board by board, one line per net.
#
# This file is DATA, not a target. Every line is a defect: a net KiCad resolves that we split, or
# one we join that KiCad keeps apart. It is committed so the set is visible and so a change has to
# say which way it moved, rather than asserting a clean read nobody has yet earned. A fix removes
# lines. A regression adds them.
#
# Regenerate with:  AGNI_ORACLE_UPDATE=1 make oracle
# Never hand-edit: the whole point is that KiCad chose these, not us.
#
# Known families still open, as of the agni issue 561 fix:
#   - group buses (` + "`CAM0{CSI}`" + ` with bus_alias members), which cross a sheet boundary the same way a
#     vector does and are not yet followed. Most of the jetson lines.
#   - sliced bus prefixes, where a parent cuts PP_OUT[0..31] into PP_OUT[0..7], PP_OUT[8..15], ...
#     and hands each to one instance of a child sheet. KiCad maps those by bit position.
#   - a pin-number swap on some symbols, which is why boards whose net COUNTS match exactly can
#     still disagree about which pins are on which net.
`

var _ = ir.Design{}
