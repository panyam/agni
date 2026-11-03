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

// TestOracleCorpus cross-checks the reader against real boards, and the oracle costs nothing. Every
// KiCad project has both the schematic and the .kicad_pcb, and the board file holds the netlist that
// KiCad itself resolved. So we read a design our way, read KiCad's answer for the same design, and
// compare which pins share a net.
//
// The boards are third-party designs under their own licences and live in panyam/agni-samples. They
// need the BOTH-VIEWS tarball, which is 19MB against tutorial-board's 3MB and not in the
// gate's default fetch, so this runs as its own target. `make oracle` fetches the corpus and then
// runs this test. `make browser-test` was the other suite outside the gate for a comparable
// reason, until PR 629 moved it inside. This one is still out because a browser is a fixed one-time
// install, where the corpus is 19MB fetched per cache miss.
//
// It asserts against a COMMITTED BASELINE of the disagreements rather than demanding zero, because
// several reader defects are still open and a test that has never passed teaches nothing. The
// baseline lists the reference nets we get wrong BY NAME, so a fix shrinks it, a regression grows it,
// and a change that swaps one defect for another of equal size is still caught.
//
// The corpus spans KiCad 6 to 9 file formats (agni issue 1233, where every pre-KiCad-8 board read
// empty and so passed this test by having nothing to disagree with). Six boards is still not a
// survey. Cynthion is the only one that crosses a sheet boundary with a bus VECTOR, and nobody has
// checked whether this test moves when the agni issue 561 fix is reverted. So
// TestBusVectorCrossesSheetBoundary is still the guard, on a fixture with kicad-cli's own answer
// beside it.
// Breaking the scalar sheet-pin join moves jetson by 27 lines, and that red-checked this file.
func TestOracleCorpus(t *testing.T) {
	if os.Getenv("AGNI_ORACLE") == "" {
		t.Skip("not the default suite: run `make oracle`, which fetches the both-views corpus first")
	}
	const dir = "../../tools/samples"
	boards, err := globCorpus(filepath.Join(dir, "boards", "*"))
	if err != nil || len(boards) == 0 {
		// Fatal, never skipped, because it is reached only via `make oracle`, which has already
		// fetched.
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

// globCorpus is filepath.Glob without macOS AppleDouble files. A tarball packed on a Mac carries a
// "._name" resource-fork shadow beside every file unless COPYFILE_DISABLE is set, and one sorts
// ahead of the real file, so it would be read as a board or as the root schematic (agni issue 812).
func globCorpus(pattern string) ([]string, error) {
	all, err := filepath.Glob(pattern)
	out := all[:0]
	for _, p := range all {
		if !strings.HasPrefix(filepath.Base(p), "._") {
			out = append(out, p)
		}
	}
	return out, err
}

// boardRoot picks the board's root schematic, the .kicad_sch with a sibling .kicad_pcb, and returns
// both paths, or two empty strings when the directory has no such pair.
func boardRoot(dir string) (root, pcb string) {
	schs, _ := globCorpus(filepath.Join(dir, "*.kicad_sch"))
	for _, s := range schs {
		if p := strings.TrimSuffix(s, ".kicad_sch") + ".kicad_pcb"; fileExists(p) {
			return s, p
		}
	}
	return "", ""
}

// baselinePath is deliberately NOT under testdata/. The docsite run specs mount that directory as a
// fixture and stamp each capture with a hash of every tracked file in it, so a baseline living there
// would restamp five tutorial captures every time a reader fix shrinks it, coupling two things that
// have nothing to do with each other.
const baselinePath = "oracle_corpus.baseline"

// crossCheckBoard reads one board both ways and returns its disagreement lines, sorted. If a board's
// two halves cannot both be read, it reports that as its single line, so the corpus records it
// rather than passing by silently skipping.
func crossCheckBoard(t *testing.T, dir string) []string {
	t.Helper()
	root, pcb := boardRoot(dir)
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

// errKind reduces a read error to its stable prefix. The message has a line number. That number
// makes the error useful, but it is the wrong thing to freeze into a baseline.
func errKind(err error) string {
	s := err.Error()
	if i := strings.Index(s, " at line "); i >= 0 {
		return s[:i]
	}
	return s
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

// readBaseline parses the committed expectations. They are "## <board>" sections of one
// disagreement per line, with '#' comments elsewhere.
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
# Known families still open, as of the agni issue 1234 fix:
#   - stubs that share a name, where several no-connect pins of one part carry the same pin name
#     (jetson's GL and DRAIN pins). We give them one unconnected-(...) net and KiCad keeps one per
#     pin, numbering the repeats _1, _2.
`

var _ = ir.Design{}

// The oracle's walk ignores AppleDouble shadows, which sort ahead of the files they shadow and pair
// up with each other, so without the filter "._csi" is chosen as the root and fails to parse
// (agni issue 812). This runs in the gate, where the oracle itself does not.
func TestOracleWalkSkipsAppleDouble(t *testing.T) {
	dir := t.TempDir()
	board := filepath.Join(dir, "boards", "csi")
	if err := os.MkdirAll(board, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		filepath.Join(dir, "boards", "._csi"),
		filepath.Join(board, "._csi.kicad_sch"), filepath.Join(board, "._csi.kicad_pcb"),
		filepath.Join(board, "csi.kicad_sch"), filepath.Join(board, "csi.kicad_pcb"),
	} {
		if err := os.WriteFile(f, []byte("(kicad)"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	boards, err := globCorpus(filepath.Join(dir, "boards", "*"))
	if err != nil || len(boards) != 1 || filepath.Base(boards[0]) != "csi" {
		t.Errorf("boards = %v (err %v), want the one real board", boards, err)
	}
	root, pcb := boardRoot(board)
	if filepath.Base(root) != "csi.kicad_sch" || filepath.Base(pcb) != "csi.kicad_pcb" {
		t.Errorf("root, pcb = %q, %q; want the real pair, not the AppleDouble shadows", root, pcb)
	}
}
