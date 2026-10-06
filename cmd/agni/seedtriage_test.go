package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// The public demo's seeded boards are the first thing anyone judges, so every finding the full
// catalog makes on them carries a recorded verdict (agni issue 857). hack/seed-triage holds one
// record per seeded design, and TestSeedTriage holds the engine to it in both directions: a finding
// nobody has judged fails, and so does a judged finding that stopped firing. An engine change that
// moves a seed's findings therefore shows up as a diff to that record in review.
//
// AGNI_SEED_TRIAGE_UPDATE=1 (make seed-triage) rewrites the records: vanished subjects are dropped
// and new findings land in an `untriaged` group per rule, which still fails until someone classifies
// it.

const seedTriageDir = "../../hack/seed-triage"

// triageClasses are the verdicts a group can carry. needs-intent is a finding that exists because a
// fact about the board has not been declared, which is neither a defect nor a choice the designer
// made.
var triageClasses = map[string]bool{
	"defect":         true,
	"design-choice":  true,
	"needs-intent":   true,
	"false-positive": true,
	"untriaged":      true,
}

// triageGroup is one judgement over a set of findings of one rule.
type triageGroup struct {
	Class string `yaml:"class"`
	// Outcome is "fail" unless stated, and "inconclusive" for a rule that could not decide.
	Outcome string `yaml:"outcome,omitempty"`
	Why     string `yaml:"why"`
	// Ticket is the issue tracking a false positive, required for that class.
	Ticket   int      `yaml:"ticket,omitempty"`
	Subjects []string `yaml:"subjects"`
}

// triageRecord is one seeded design's record.
type triageRecord struct {
	// Design is the path the check reads, relative to the repo root.
	Design   string                   `yaml:"design"`
	Findings map[string][]triageGroup `yaml:"findings"`
}

// seedDesignDirs are the designs the demo seeds, keyed by the name each one's design.yaml declares,
// read from the Makefile's DEMO_SEEDS so a new seed without a record fails here.
func seedDesignDirs(t *testing.T) map[string]string {
	t.Helper()
	mk, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	seeds := regexp.MustCompile(`--seed [a-z0-9_-]+=(\S+)`).FindAllSubmatch(mk, -1)
	if len(seeds) == 0 {
		t.Fatal("no --seed in the Makefile's DEMO_SEEDS")
	}
	out := map[string]string{}
	for _, m := range seeds {
		root := string(m[1])
		err := filepath.WalkDir(filepath.Join("../..", root), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "design.yaml" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			var desc struct {
				Name string `yaml:"name"`
			}
			if err := yaml.Unmarshal(b, &desc); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			rel, err := filepath.Rel("../..", filepath.Dir(p))
			if err != nil {
				return err
			}
			out[desc.Name] = filepath.ToSlash(rel)
			return nil
		})
		if err != nil {
			t.Fatalf("seed %s: %v (make samples-oracle fetches the boards)", root, err)
		}
	}
	return out
}

// seedFinding is one failing or inconclusive verdict, as the check's csv names it.
type seedFinding struct{ rule, outcome, subjects string }

// seedFindings checks a design the way `agni check --verdicts` does and keeps what did not pass.
func seedFindings(t *testing.T, design string) map[seedFinding]bool {
	t.Helper()
	freshWorkspace(t)
	out := runCLI(t, rootCmd(), "check", filepath.Join("../..", design), "--verdicts", "--format", "csv")
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("%s: csv %v, %d rows", design, err, len(rows))
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	got := map[seedFinding]bool{}
	for _, r := range rows[1:] {
		o := r[col["outcome"]]
		if o == "fail" || o == "inconclusive" {
			got[seedFinding{r[col["rule"]], o, r[col["subjects"]]}] = true
		}
	}
	return got
}

func loadTriage(path string) (*triageRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var rec triageRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &rec, nil
}

func outcomeOf(g triageGroup) string {
	if g.Outcome == "" {
		return "fail"
	}
	return g.Outcome
}

func TestSeedTriage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	update := os.Getenv("AGNI_SEED_TRIAGE_UPDATE") != ""
	designs := seedDesignDirs(t)
	if len(designs) < 4 {
		t.Fatalf("found %d seeded designs, want the gateway, the Feather, its antenna and the Jetson baseboard: %v", len(designs), designs)
	}
	names := make([]string, 0, len(designs))
	for n := range designs {
		names = append(names, n)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(seedTriageDir, name+".triage.yaml")
			rec, err := loadTriage(path)
			if os.IsNotExist(err) && update {
				rec, err = &triageRecord{Design: designs[name]}, nil
			}
			if err != nil {
				t.Fatalf("%v (AGNI_SEED_TRIAGE_UPDATE=1 drafts one)", err)
			}
			if rec.Design != designs[name] {
				t.Fatalf("record reads %q, the seed declares the design at %q", rec.Design, designs[name])
			}
			got := seedFindings(t, rec.Design)
			total += len(got)
			if update {
				writeTriage(t, path, rec, got)
				return
			}
			checkTriage(t, rec, got)
		})
	}
	// One clean board is a result; four are a catalog that never installed.
	if total == 0 {
		t.Fatal("no seeded design produced a failing verdict, so the comparison above checked nothing")
	}
}

// checkTriage compares a record with what the engine found, and fails on each difference.
func checkTriage(t *testing.T, rec *triageRecord, got map[seedFinding]bool) {
	t.Helper()
	recorded := map[seedFinding]bool{}
	for rule, groups := range rec.Findings {
		for i, g := range groups {
			where := fmt.Sprintf("%s group %d", rule, i+1)
			if !triageClasses[g.Class] {
				t.Errorf("%s: class %q is not one of defect, design-choice, needs-intent, false-positive", where, g.Class)
			}
			if g.Class == "untriaged" {
				t.Errorf("%s: %d finding(s) are untriaged", where, len(g.Subjects))
			}
			if g.Class == "false-positive" && g.Ticket == 0 {
				t.Errorf("%s: a false positive needs the ticket that fixes it", where)
			}
			if strings.TrimSpace(g.Why) == "" && g.Class != "untriaged" {
				t.Errorf("%s: no why", where)
			}
			for _, s := range g.Subjects {
				f := seedFinding{rule, outcomeOf(g), s}
				if recorded[f] {
					t.Errorf("%s: %s is recorded twice", where, s)
				}
				recorded[f] = true
				if !got[f] {
					t.Errorf("%s: %s %s no longer fires", where, outcomeOf(g), s)
				}
			}
		}
	}
	var missing []string
	for f := range got {
		if !recorded[f] {
			missing = append(missing, fmt.Sprintf("%s %s %s", f.rule, f.outcome, f.subjects))
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("untriaged finding: %s (make seed-triage drafts it)", m)
	}
}

// writeTriage rewrites a record against what the engine found. Recorded groups keep their order and
// judgement and lose the subjects that stopped firing; what is new goes into an untriaged group per
// rule and outcome.
func writeTriage(t *testing.T, path string, rec *triageRecord, got map[seedFinding]bool) {
	t.Helper()
	if rec.Findings == nil {
		rec.Findings = map[string][]triageGroup{}
	}
	recorded := map[seedFinding]bool{}
	for rule, groups := range rec.Findings {
		kept := groups[:0]
		for _, g := range groups {
			subs := g.Subjects[:0]
			for _, s := range g.Subjects {
				f := seedFinding{rule, outcomeOf(g), s}
				if got[f] {
					subs = append(subs, s)
					recorded[f] = true
				}
			}
			g.Subjects = subs
			if len(subs) > 0 {
				kept = append(kept, g)
			}
		}
		rec.Findings[rule] = kept
	}
	fresh := map[[2]string][]string{}
	for f := range got {
		if !recorded[f] {
			k := [2]string{f.rule, f.outcome}
			fresh[k] = append(fresh[k], f.subjects)
		}
	}
	keys := make([][2]string, 0, len(fresh))
	for k := range fresh {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i][0]+keys[i][1] < keys[j][0]+keys[j][1] })
	for _, k := range keys {
		subs := fresh[k]
		sort.Strings(subs)
		g := triageGroup{Class: "untriaged", Subjects: subs}
		if k[1] != "fail" {
			g.Outcome = k[1]
		}
		rec.Findings[k[0]] = append(rec.Findings[k[0]], g)
	}
	for rule, groups := range rec.Findings {
		if len(groups) == 0 {
			delete(rec.Findings, rule)
		}
	}
	var buf bytes.Buffer
	buf.WriteString("# Every failing or inconclusive verdict the full catalog makes on this seeded design, each with\n")
	buf.WriteString("# a judgement (agni issue 857). TestSeedTriage fails on any difference from what the engine finds,\n")
	buf.WriteString("# and `make seed-triage` drafts new findings as untriaged. Classes: defect, design-choice,\n")
	buf.WriteString("# needs-intent (a fact about the board is undeclared) and false-positive (with its ticket).\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(rec); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}
