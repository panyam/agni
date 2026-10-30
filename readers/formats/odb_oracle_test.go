package formats

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// odbOracleBaseline holds the known differences between each sample board read from its .kicad_pcb
// and from kicad-cli's ODB++ export of it, one `<board>\t<difference>` per line. Every line is the
// exporter's doing rather than a reader's, and the file says which kind each is.
const odbOracleBaseline = "testdata/odb_oracle.baseline"

// TestODBOracleCorpus reads each board in the samples corpus both ways (the odb-kicad artifact
// beside oracle-corpus) and holds the differences to the committed baseline, as TestOracleCorpus
// does for the KiCad reader's two views. A difference the baseline lacks is a regression in either
// reader, and one the baseline has that no longer appears is a fix to record. It needs the 15 MB
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
	var got []string
	for _, job := range jobs {
		board := strings.TrimSuffix(filepath.Base(job), ".odb.tgz")
		pcbs, _ := filepath.Glob(filepath.Join(samples, "boards", board, "*.kicad_pcb"))
		if len(pcbs) != 1 {
			t.Fatalf("%s: want one .kicad_pcb beside the export, found %d", board, len(pcbs))
		}
		diffs, err := compareBoard(odbPair{kicad: pcbs[0], odb: job})
		if err != nil {
			diffs = []string{"error: " + err.Error()}
		}
		for _, d := range diffs {
			got = append(got, board+"\t"+d)
		}
	}
	sort.Strings(got)

	if os.Getenv("AGNI_ODB_ORACLE_UPDATE") != "" {
		if err := os.WriteFile(odbOracleBaseline, []byte(odbBaselineHeader+strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s with %d lines", odbOracleBaseline, len(got))
		return
	}
	raw, err := os.ReadFile(odbOracleBaseline)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			want = append(want, l)
		}
	}
	added, gone := setDiff(got, want), setDiff(want, got)
	if len(added)+len(gone) > 0 {
		t.Errorf("the two reads moved against %s (AGNI_ODB_ORACLE_UPDATE=1 rewrites it once the move is understood)\nnew differences (%d):\n  %s\nno longer different (%d):\n  %s",
			odbOracleBaseline, len(added), capLines(added), len(gone), capLines(gone))
	}
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
