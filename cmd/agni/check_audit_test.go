package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// Mission 1708's exercise (agni issue 1712). Each board's verdicts are held to a hand-audited file
// in testdata/verdict-audit. A row there records the outcome a reader reached by reading the
// schematic, and the bug that explains why agni disagrees, if it does. The outcome "none" says the
// rule should not judge that subject at all, such as a shield net under an ESD rule. The file lists audited rows
// only, so a verdict with no row counts as not yet audited.
//
// The gate runs this as a ratchet. A mismatch must name a bug, and a row that names a bug must
// still mismatch, so a fix fails here until its rows are updated. `make exercise-checks` sets
// AGNI_EXERCISE_CHECKS=1, prints the table, and fails while any mismatch remains.

type auditBoard struct {
	name, dir, file string
	minVerdicts     int // count guard, well under today's count
}

var auditBoards = []auditBoard{
	{"glasgow-revC3", "../../tools/samples/boards/glasgow-revC3", "testdata/verdict-audit/glasgow-revC3.tsv", 5000},
	{"jetson", "../../tools/samples/boards/jetson-agx-thor-baseboard", "testdata/verdict-audit/jetson.tsv", 20000},
}

type auditRow struct {
	id, want, bug, note string
	line                int
}

// The four ways an audited verdict can disagree, plus a row whose verdict agni no longer states.
const (
	kindFalsePositive  = "false positive"       // agni fails, the audit says pass
	kindFalseNegative  = "false negative"       // agni passes, the audit says fail
	kindUnclassified   = "unclassified pass"    // agni passes, the audit says the rule could not judge
	kindMissedJudgment = "missed judgment"      // agni did not consider it, the audit judged it
	kindMissing        = "missing verdict"      // the audited verdict is gone from the run
	kindOther          = "outcome disagreement" // fail against not-considered
)

type auditMismatch struct {
	row       auditRow
	got, kind string
}

type auditTally struct {
	verdicts, audited, match                         int
	fp, fn, unclassified, missed, missing, unaudited int
}

type auditReport struct {
	rules      map[string]*auditTally
	mismatches []auditMismatch
}

func auditOutcome(o checks.Outcome) string {
	switch o {
	case checks.Outcome_OUTCOME_PASS:
		return "pass"
	case checks.Outcome_OUTCOME_FAIL:
		return "fail"
	case checks.Outcome_OUTCOME_NOT_CONSIDERED:
		return "not-considered"
	}
	return strings.ToLower(strings.TrimPrefix(o.String(), "OUTCOME_"))
}

func readAudit(path string) ([]auditRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows []auditRow
	seen := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			return nil, fmt.Errorf("%s:%d: want 4 tab-separated columns (verdict, want, bug, note), got %d", path, n, len(cols))
		}
		r := auditRow{id: cols[0], want: cols[1], bug: cols[2], note: cols[3], line: n}
		switch r.want {
		case "pass", "fail", "not-considered", "none":
		default:
			return nil, fmt.Errorf("%s:%d: want is %q, not pass, fail, not-considered or none", path, n, r.want)
		}
		if r.bug != "" && !strings.HasPrefix(r.bug, "#") {
			return nil, fmt.Errorf("%s:%d: bug is %q, want an issue such as #769 or empty", path, n, r.bug)
		}
		if strings.TrimSpace(r.note) == "" {
			return nil, fmt.Errorf("%s:%d: an audited row says what was checked", path, n)
		}
		if prev, dup := seen[r.id]; dup {
			return nil, fmt.Errorf("%s:%d: %s is audited again, first at line %d", path, n, r.id, prev)
		}
		seen[r.id] = n
		rows = append(rows, r)
	}
	return rows, sc.Err()
}

// auditRuleOf takes the rule from a verdict id of the form rule:(subject). A rule name has no "(".
func auditRuleOf(id string) string {
	if i := strings.Index(id, ":("); i > 0 {
		return id[:i]
	}
	return id
}

func compareAudit(verdicts []*checks.Verdict, rows []auditRow) auditReport {
	rep := auditReport{rules: map[string]*auditTally{}}
	tally := func(rule string) *auditTally {
		if rep.rules[rule] == nil {
			rep.rules[rule] = &auditTally{}
		}
		return rep.rules[rule]
	}
	got := map[string]string{}
	for _, v := range verdicts {
		got[v.GetId()] = auditOutcome(v.GetOutcome())
		tally(v.GetRule()).verdicts++
	}
	audited := map[string]bool{}
	for _, r := range rows {
		audited[r.id] = true
		t := tally(auditRuleOf(r.id))
		t.audited++
		g, ok := got[r.id]
		kind := ""
		switch {
		case r.want == "none" && (!ok || g == "not-considered"):
			t.match++ // the rule should not judge this subject, and does not
		case !ok:
			kind, t.missing = kindMissing, t.missing+1
		case g == r.want:
			t.match++
		case g == "fail" && r.want == "none":
			kind, t.fp = kindFalsePositive, t.fp+1
		case g == "pass" && r.want == "none":
			kind, t.unclassified = kindUnclassified, t.unclassified+1
		case g == "fail" && r.want == "pass":
			kind, t.fp = kindFalsePositive, t.fp+1
		case g == "pass" && r.want == "fail":
			kind, t.fn = kindFalseNegative, t.fn+1
		case g == "pass" && r.want == "not-considered":
			kind, t.unclassified = kindUnclassified, t.unclassified+1
		case g == "not-considered":
			kind, t.missed = kindMissedJudgment, t.missed+1
		default:
			kind, t.fp = kindOther, t.fp+1
		}
		if kind != "" {
			rep.mismatches = append(rep.mismatches, auditMismatch{row: r, got: g, kind: kind})
		}
	}
	for id := range got {
		if !audited[id] {
			tally(auditRuleOf(id)).unaudited++
		}
	}
	return rep
}

func (rep auditReport) table(board string) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintf(w, "%s\tverdicts\taudited\tmatch\tFP\tFN\tunclassified pass\tmissed\tmissing\tnot yet audited\t\n", board)
	names := make([]string, 0, len(rep.rules))
	for n := range rep.rules {
		names = append(names, n)
	}
	sort.Strings(names)
	var sum auditTally
	for _, n := range names {
		t := rep.rules[n]
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t\n", n, t.verdicts, t.audited, t.match, t.fp, t.fn, t.unclassified, t.missed, t.missing, t.unaudited)
		sum.verdicts += t.verdicts
		sum.audited += t.audited
		sum.match += t.match
		sum.fp += t.fp
		sum.fn += t.fn
		sum.unclassified += t.unclassified
		sum.missed += t.missed
		sum.missing += t.missing
		sum.unaudited += t.unaudited
	}
	fmt.Fprintf(w, "total\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t\n", sum.verdicts, sum.audited, sum.match, sum.fp, sum.fn, sum.unclassified, sum.missed, sum.missing, sum.unaudited)
	w.Flush()
	return buf.String()
}

func runBoardVerdicts(t *testing.T, dir string) []*checks.Verdict {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		// Fatal rather than skipped, because a missing corpus must not read as a pass.
		t.Fatalf("%s missing (run `make samples-oracle`): %v", dir, err)
	}
	var resp webapi.CheckDesignResponse
	if err := protojson.Unmarshal([]byte(runCheck(t, "--verdicts", "--format", "json", dir)), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.GetVerdicts()
}

func TestVerdictAudit(t *testing.T) {
	exercise := os.Getenv("AGNI_EXERCISE_CHECKS") == "1"
	for _, b := range auditBoards {
		t.Run(b.name, func(t *testing.T) {
			rows, err := readAudit(b.file)
			if err != nil {
				t.Fatal(err)
			}
			verdicts := runBoardVerdicts(t, b.dir)
			if len(verdicts) < b.minVerdicts {
				t.Fatalf("%s stated %d verdicts, want at least %d", b.name, len(verdicts), b.minVerdicts)
			}
			rep := compareAudit(verdicts, rows)
			if exercise {
				fmt.Println(rep.table(b.name))
				for _, m := range rep.mismatches {
					fmt.Printf("  %s %s: agni says %s, the audit says %s (%s)\n", m.kind, m.row.id, m.got, m.row.want, orNone(m.row.bug))
				}
				fmt.Println()
				if len(rep.mismatches) > 0 {
					t.Errorf("%s: %d audited verdict(s) disagree with agni", b.name, len(rep.mismatches))
				}
				return
			}
			t.Log("\n" + rep.table(b.name))
			mismatched := map[string]bool{}
			for _, m := range rep.mismatches {
				mismatched[m.row.id] = true
				if m.row.bug == "" {
					t.Errorf("%s:%d: %s %s: agni says %s, the audit says %s, and the row names no bug", b.file, m.row.line, m.kind, m.row.id, m.got, m.row.want)
				}
			}
			for _, r := range rows {
				if r.bug != "" && !mismatched[r.id] {
					t.Errorf("%s:%d: %s now agrees with the audit (%s), so %s no longer bites it; clear the bug column", b.file, r.line, r.id, r.want, r.bug)
				}
			}
		})
	}
}

func orNone(bug string) string {
	if bug == "" {
		return "no bug named"
	}
	return bug
}

// TestVerdictAuditComparison holds compareAudit to each kind on a hand-built run, and flips one
// audited row to show a changed verdict is reported as exactly that row.
func TestVerdictAuditComparison(t *testing.T) {
	v := func(id string, o checks.Outcome) *checks.Verdict {
		return &checks.Verdict{Id: id, Rule: auditRuleOf(id), Outcome: o}
	}
	run := []*checks.Verdict{
		v("r:(net:A)", checks.Outcome_OUTCOME_PASS),
		v("r:(net:B)", checks.Outcome_OUTCOME_FAIL),
		v("r:(net:C)", checks.Outcome_OUTCOME_PASS),
		v("r:(net:D)", checks.Outcome_OUTCOME_PASS),
		v("r:(net:E)", checks.Outcome_OUTCOME_NOT_CONSIDERED),
		v("r:(net:F)", checks.Outcome_OUTCOME_PASS),
	}
	rows := []auditRow{
		{id: "r:(net:A)", want: "pass"},
		{id: "r:(net:B)", want: "pass"},
		{id: "r:(net:C)", want: "fail"},
		{id: "r:(net:D)", want: "not-considered"},
		{id: "r:(net:E)", want: "fail"},
		{id: "r:(net:Z)", want: "pass"},
		{id: "r:(net:F)", want: "none"},
		{id: "r:(net:Y)", want: "none"},
	}
	rep := compareAudit(run, rows)
	got := map[string]string{}
	for _, m := range rep.mismatches {
		got[m.row.id] = m.kind
	}
	want := map[string]string{
		"r:(net:B)": kindFalsePositive,
		"r:(net:C)": kindFalseNegative,
		"r:(net:D)": kindUnclassified,
		"r:(net:E)": kindMissedJudgment,
		"r:(net:Z)": kindMissing,
		"r:(net:F)": kindUnclassified,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d mismatches %v, want %d", len(got), got, len(want))
	}
	for id, k := range want {
		if got[id] != k {
			t.Errorf("%s: got %q, want %q", id, got[id], k)
		}
	}
	tl := rep.rules["r"]
	if tl.verdicts != 6 || tl.audited != 8 || tl.match != 2 || tl.unaudited != 0 {
		t.Errorf("tally %+v: want 6 verdicts, 8 audited, 2 matches (A, and Y gone as it should be), 0 not yet audited", *tl)
	}

	t.Run("a flipped row is reported as exactly that row", func(t *testing.T) {
		agree := []auditRow{{id: "r:(net:A)", want: "pass"}, {id: "r:(net:B)", want: "fail"}}
		if n := len(compareAudit(run, agree).mismatches); n != 0 {
			t.Fatalf("an agreeing audit reported %d mismatches", n)
		}
		agree[1].want = "pass"
		ms := compareAudit(run, agree).mismatches
		if len(ms) != 1 || ms[0].row.id != "r:(net:B)" || ms[0].kind != kindFalsePositive {
			t.Errorf("after flipping B, got %+v, want one false positive on r:(net:B)", ms)
		}
	})
}

func TestVerdictAuditFilesParse(t *testing.T) {
	for _, b := range auditBoards {
		if _, err := readAudit(filepath.Clean(b.file)); err != nil {
			t.Error(err)
		}
	}
}
