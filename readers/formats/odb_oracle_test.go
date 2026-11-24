package formats

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// odbOracleBaseline holds the known differences between each sample board read from its .kicad_pcb
// and from kicad-cli's ODB++ export of it, one `<board>\t<difference>` per line. The exporter causes
// every line, not a reader, and the file says which kind each is.
const odbOracleBaseline = "testdata/odb_oracle.baseline"

// TestODBOracleCorpus reads each board in the samples corpus both ways, using the odb-kicad artifact
// beside oracle-corpus. It holds the differences to the committed baseline, as TestOracleCorpus does
// for the KiCad reader's two views. If the baseline lacks a difference, one of the readers
// regressed. If a baseline line no longer appears, a fix needs recording. The test needs the 15 MB
// export corpus, so it runs in `make odb-oracle` rather than the gate. AGNI_ODB_ORACLE_UPDATE=1
// rewrites the baseline.
func TestODBOracleCorpus(t *testing.T) {
	if os.Getenv("AGNI_ODB_ORACLE") == "" {
		t.Skip("not the default suite: run `make odb-oracle`, which fetches the export corpus first")
	}
	const samples = "../../tools/samples"
	jobs, _ := filepath.Glob(filepath.Join(samples, "odb", "kicad", "*.odb.tgz"))
	if len(jobs) == 0 {
		t.Fatalf("no exports under %s/odb/kicad; run `make odb-oracle`", samples)
	}
	update := os.Getenv("AGNI_ODB_ORACLE_UPDATE") != ""
	var want []string
	if !update {
		raw, err := os.ReadFile(odbOracleBaseline)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(raw), "\n") {
			if l != "" && !strings.HasPrefix(l, "#") {
				want = append(want, l)
			}
		}
	}
	var got []string
	boards := map[string]bool{}
	for _, job := range jobs {
		board := strings.TrimSuffix(filepath.Base(job), ".odb.tgz")
		boards[board] = true
		t.Run(board, func(t *testing.T) {
			pcbs, _ := filepath.Glob(filepath.Join(samples, "boards", board, "*.kicad_pcb"))
			if len(pcbs) != 1 {
				t.Fatalf("%s: want one .kicad_pcb beside the export, found %d", board, len(pcbs))
			}
			diffs, err := compareBoard(odbPair{kicad: pcbs[0], odb: job})
			// make exercise-formats reads these three lines into its table.
			if err != nil {
				diffs = []string{"error: " + err.Error()}
				t.Logf("read: does not read, %s", errKind(err))
			} else {
				t.Logf("read: the .kicad_pcb and its ODB++ export")
			}
			t.Logf("compared: pin nets, parts, pads, stack and copper against the .kicad_pcb; the exporter causes each gap")
			t.Logf("gaps: %d", len(diffs))
			var mine []string
			for _, d := range diffs {
				mine = append(mine, board+"\t"+d)
			}
			got = append(got, mine...)
			if update {
				return
			}
			var wantBoard []string
			for _, l := range want {
				if strings.HasPrefix(l, board+"\t") {
					wantBoard = append(wantBoard, l)
				}
			}
			reportODBMoves(t, setDiff(mine, wantBoard), setDiff(wantBoard, mine))
		})
	}
	sort.Strings(got)

	if update {
		if t.Failed() {
			return
		}
		if err := os.WriteFile(odbOracleBaseline, []byte(odbBaselineHeader+strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s with %d lines", odbOracleBaseline, len(got))
		return
	}
	// A baseline line for a board with no export is gone too.
	var orphans []string
	for _, l := range want {
		if b, _, _ := strings.Cut(l, "\t"); !boards[b] {
			orphans = append(orphans, l)
		}
	}
	reportODBMoves(t, nil, orphans)
}

// reportODBMoves fails t when the reads moved against the baseline.
func reportODBMoves(t *testing.T, added, gone []string) {
	t.Helper()
	if len(added)+len(gone) > 0 {
		t.Errorf("the two reads moved against %s (AGNI_ODB_ORACLE_UPDATE=1 rewrites it once the move is understood)\nnew differences (%d):\n  %s\nno longer different (%d):\n  %s",
			odbOracleBaseline, len(added), capLines(added), len(gone), capLines(gone))
	}
}

// errKind reduces a read error to its first clause, without the line number.
func errKind(err error) string {
	s := err.Error()
	if i := strings.Index(s, " at line "); i >= 0 {
		return s[:i]
	}
	return s
}

const odbBaselineHeader = `# Known differences between each samples board read from its .kicad_pcb and from kicad-cli's ODB++
# export of it (TestODBOracleCorpus, make odb-oracle). Each is the exporter's doing, not a reader's:
#   parts kicad only      a footprint marked exclude_from_pos_files, which the exporter leaves out
#   parts odb++ only      a designator the exporter renamed because two footprints share it (_10, ...)
#   pads odb++ only       a non-plated hole the KiCad file lists on mask alone, which ODB++ drills
#   error: kicad read     a board the KiCad reader cannot read yet (the Feather, agni issue 681)
# Regenerate with AGNI_ODB_ORACLE_UPDATE=1 make odb-oracle, and read the diff.
`

func setDiff(a, b []string) []string {
	in := map[string]int{}
	for _, x := range b {
		in[x]++
	}
	var out []string
	for _, x := range a {
		if in[x] > 0 {
			in[x]--
			continue
		}
		out = append(out, x)
	}
	return out
}
