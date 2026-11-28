package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestTheUnresolvedGateCoversEveryVerdictALostLibraryChanges is agni issue 88's acceptance oracle.
// The full read of Glasgow supplies the right answers independently. Each case deletes one symbol
// from the schematics' embedded library, so its parts lose their pins, and checks the board again.
// Every verdict that differs from the full read must be inconclusive, so the gate never turns a
// lost pin into a confident answer. The count of verdicts kept measures how much the per-subject
// gate keeps over the design-wide one, which kept none.
//
// Only the rules the gate covers are compared, meaning those that read connectivity, less
// symbol-unresolved, which reports the loss itself. A rule reading only net names judges the
// names the lossy read has. KiCad names a net after one of its pins, so a lost pin can rename a
// net. That rule then judges the new name, and the gate leaves it alone on purpose.
func TestTheUnresolvedGateCoversEveryVerdictALostLibraryChanges(t *testing.T) {
	src := auditBoards[0].dir
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("%s missing (run `make samples-oracle`): %v", src, err)
	}
	full := oracleVerdicts(t, src)
	if len(full) < auditBoards[0].minVerdicts {
		t.Fatalf("the full read stated %d verdicts, want at least %d", len(full), auditBoards[0].minVerdicts)
	}
	gatedRule := map[string]bool{}
	for _, r := range check.BuiltinRules() {
		for _, fact := range r.Reads {
			if check.TierOf(fact) == check.TierConnectivity && r.Name != "symbol-unresolved" {
				gatedRule[r.Name] = true
			}
		}
	}
	if len(gatedRule) < 20 {
		t.Fatalf("%d rules read connectivity, want at least 20", len(gatedRule))
	}
	victims := oracleVictims(t, src)
	if len(victims) < 6 {
		t.Fatalf("picked %d symbols to remove, want at least 6", len(victims))
	}
	for _, sym := range victims {
		t.Run(sym, func(t *testing.T) {
			dir := copyBoardWithout(t, src, sym)
			lost := oracleVerdicts(t, dir)
			kept, gated, vanished := 0, 0, 0
			for id, want := range full {
				if !gatedRule[ruleOfID(id)] {
					continue
				}
				got, ok := lost[id]
				switch {
				case !ok:
					vanished++
				case got == "inconclusive" && want != "inconclusive":
					gated++
				case got != want:
					t.Errorf("%s: the full read says %s, and with %s lost the check says %s", id, want, sym, got)
				default:
					kept++
				}
			}
			for id, got := range lost {
				if _, ok := full[id]; !ok && gatedRule[ruleOfID(id)] && got != "inconclusive" {
					t.Errorf("%s: a subject only the lossy read has says %s, want inconclusive", id, got)
				}
			}
			if gated == 0 {
				t.Errorf("removing %s gated no verdict, so the case proves nothing", sym)
			}
			t.Logf("%s lost: %d verdicts kept, %d gated, %d no longer stated", sym, kept, gated, vanished)
		})
	}
}

// ruleOfID returns the rule name a verdict id starts with.
func ruleOfID(id string) string {
	rule, _, _ := strings.Cut(id, ":(")
	return rule
}

// oracleVerdicts checks the board in dir and returns each verdict's outcome by id.
func oracleVerdicts(t *testing.T, dir string) map[string]string {
	t.Helper()
	var resp webapi.CheckDesignResponse
	if err := protojson.Unmarshal([]byte(runCheck(t, "--verdicts", "--format", "json", dir)), &resp); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, v := range resp.GetVerdicts() {
		out[v.GetId()] = auditOutcome(v.GetOutcome())
	}
	return out
}

// oracleVictims picks every fifth placed symbol from Glasgow's embedded library, so the cases spread
// across connectors, ICs and passives without depending on any one part's name. Power and graphic
// symbols are left out, since losing a logo loses no pin.
func oracleVictims(t *testing.T, dir string) []string {
	t.Helper()
	placed := map[string]bool{}
	for _, f := range schematics(t, dir) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, chunk := range strings.Split(string(b), `(lib_id "`)[1:] {
			if name, _, ok := strings.Cut(chunk, `"`); ok && !strings.HasPrefix(name, "power:") && !strings.HasPrefix(name, "Graphic:") {
				placed[name] = true
			}
		}
	}
	names := make([]string, 0, len(placed))
	for n := range placed {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []string
	for i := 0; i < len(names); i += 5 {
		out = append(out, names[i])
	}
	return out
}

func schematics(t *testing.T, dir string) []string {
	t.Helper()
	fs, err := filepath.Glob(filepath.Join(dir, "*.kicad_sch"))
	if err != nil || len(fs) == 0 {
		t.Fatalf("no schematics in %s: %v", dir, err)
	}
	return fs
}

// copyBoardWithout copies the board into a temporary folder with sym deleted from every
// schematic's embedded library.
func copyBoardWithout(t *testing.T, src, sym string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	removed := false
	for _, e := range entries {
		if e.IsDir() {
			if err := os.CopyFS(filepath.Join(dir, e.Name()), os.DirFS(filepath.Join(src, e.Name()))); err != nil {
				t.Fatal(err)
			}
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(e.Name(), ".kicad_sch") {
			var cut bool
			b, cut = withoutLibSymbol(b, sym)
			removed = removed || cut
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !removed {
		t.Fatalf("%s is in no schematic's embedded library", sym)
	}
	return dir
}

// withoutLibSymbol deletes the top-level (symbol "name" ...) form inside a schematic's lib_symbols.
func withoutLibSymbol(b []byte, name string) ([]byte, bool) {
	s := string(b)
	lib := strings.Index(s, "(lib_symbols")
	if lib < 0 {
		return b, false
	}
	head := `(symbol "` + name + `"`
	for i := lib + 1; i < len(s); {
		j := strings.Index(s[i:], head)
		if j < 0 {
			return b, false
		}
		start := i + j
		if depthAt(s, lib, start) != 1 {
			i = start + 1
			continue
		}
		end := formEnd(s, start)
		return []byte(s[:start] + s[end:]), true
	}
	return b, false
}

// depthAt counts the forms open between from and at, so 1 means a direct child of the form at from.
func depthAt(s string, from, at int) int {
	depth, inStr := 0, false
	for i := from; i < at; i++ {
		switch c := s[i]; {
		case inStr && c == '\\':
			i++
		case c == '"':
			inStr = !inStr
		case !inStr && c == '(':
			depth++
		case !inStr && c == ')':
			depth--
		}
	}
	return depth
}

// formEnd returns the index just past the form opening at start.
func formEnd(s string, start int) int {
	depth, inStr := 0, false
	for i := start; i < len(s); i++ {
		switch c := s[i]; {
		case inStr && c == '\\':
			i++
		case c == '"':
			inStr = !inStr
		case !inStr && c == '(':
			depth++
		case !inStr && c == ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
