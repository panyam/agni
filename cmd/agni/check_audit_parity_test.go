package main

import (
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	checks "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/goapplib/storage/mounts"
	"google.golang.org/protobuf/encoding/protojson"
)

// A server checks a design through two paths of its own, and either could read design.yaml
// differently from the CLI with nothing erroring (agni issue 1922). `check --remote` uploads the
// folder, and a stored design is pushed once and then checked by name. Each subtest holds a
// server's verdicts on the two audited boards to the CLI's run on the same folder, id for id.
//
// The runs pass no --params, because a remote command refuses it. Parity needs the same inputs on
// both sides, and TestVerdictAudit holds the audited configuration.
func TestVerdictAuditServerParity(t *testing.T) {
	for _, b := range auditBoards {
		t.Run(b.name, func(t *testing.T) {
			dir, err := filepath.Abs(b.dir)
			if err != nil {
				t.Fatal(err)
			}
			local := verdictOutcomes(t, b, runBoardVerdicts(t, auditBoard{name: b.name, dir: dir, minVerdicts: b.minVerdicts}))

			t.Run("remote", func(t *testing.T) {
				cli := []mounts.Mount{mountset.Folder("boards", filepath.Dir(dir))}
				srv := httptest.NewServer(servedMuxFor(t, cli, nil))
				t.Cleanup(srv.Close)
				out := runCheck(t, "--verdicts", "--format", "json", dir, "--remote", srv.URL)
				if diffs := compareOutcomes(local, verdictOutcomes(t, b, parseVerdicts(t, out))); len(diffs) > 0 {
					t.Errorf("check --remote disagrees with the CLI on %d verdict(s):\n%s", len(diffs), joinLines(diffs))
				}
			})

			t.Run("stored", func(t *testing.T) {
				srv := designServerCapped(t, t.TempDir(), t.TempDir(), 1<<28)
				id := strings.ToLower(b.name)
				if out, err := runDesigns(t, srv.URL, "push", dir, "--id", id); err != nil {
					t.Fatalf("push: %v\n%s", err, out)
				}
				out, err := runAgni(t, "check", "designs/"+id, "--remote", srv.URL, "--verdicts", "--format", "json")
				if err != nil {
					t.Fatalf("check by name: %v\n%s", err, out)
				}
				if diffs := compareOutcomes(local, verdictOutcomes(t, b, parseVerdicts(t, out))); len(diffs) > 0 {
					t.Errorf("a stored design's check disagrees with the CLI on %d verdict(s):\n%s", len(diffs), joinLines(diffs))
				}
			})
		})
	}
}

// The comparison reports a flipped outcome, a verdict only one side states, and nothing else.
func TestCompareOutcomesNamesEachDifference(t *testing.T) {
	cli := map[string]string{"r:(net:A)": "pass", "r:(net:B)": "fail", "r:(net:C)": "pass"}
	srv := map[string]string{"r:(net:A)": "pass", "r:(net:B)": "pass", "r:(net:D)": "fail"}
	got := compareOutcomes(cli, srv)
	want := []string{
		"r:(net:B): the CLI says fail, the server says pass",
		"r:(net:C): the CLI says pass, the server states no verdict",
		"r:(net:D): the CLI states no verdict, the server says fail",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("compareOutcomes =\n%s\nwant\n%s", joinLines(got), joinLines(want))
	}
	if d := compareOutcomes(cli, cli); len(d) != 0 {
		t.Errorf("a run compared with itself differs on %v", d)
	}
}

func parseVerdicts(t *testing.T, out string) []*checks.Verdict {
	t.Helper()
	var resp webapi.CheckDesignResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not a CheckDesignResponse: %v\n%.400s", err, out)
	}
	return resp.GetVerdicts()
}

// verdictOutcomes maps each verdict id to its outcome word, and holds the run to the board's count
// guard, so two empty runs never agree.
func verdictOutcomes(t *testing.T, b auditBoard, vs []*checks.Verdict) map[string]string {
	t.Helper()
	if len(vs) < b.minVerdicts {
		t.Fatalf("%s stated %d verdicts, want at least %d", b.name, len(vs), b.minVerdicts)
	}
	out := make(map[string]string, len(vs))
	for _, v := range vs {
		out[v.GetId()] = auditOutcome(v.GetOutcome())
	}
	return out
}

// compareOutcomes lists every id whose outcome differs between the two runs, sorted by id.
func compareOutcomes(cli, srv map[string]string) []string {
	ids := map[string]bool{}
	for id := range cli {
		ids[id] = true
	}
	for id := range srv {
		ids[id] = true
	}
	var out []string
	for id := range ids {
		a, inA := cli[id]
		b, inB := srv[id]
		switch {
		case !inB:
			out = append(out, fmt.Sprintf("%s: the CLI says %s, the server states no verdict", id, a))
		case !inA:
			out = append(out, fmt.Sprintf("%s: the CLI states no verdict, the server says %s", id, b))
		case a != b:
			out = append(out, fmt.Sprintf("%s: the CLI says %s, the server says %s", id, a, b))
		}
	}
	sort.Strings(out)
	return out
}

func joinLines(lines []string) string {
	if len(lines) > 20 {
		lines = append(lines[:20:20], fmt.Sprintf("... and %d more", len(lines)-20))
	}
	s := ""
	for _, l := range lines {
		s += "  " + l + "\n"
	}
	return s
}
