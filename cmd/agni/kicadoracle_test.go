package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// KiCad's own ERC and DRC are an independent answer for the agni rules that mirror one of its
// checks, and the seeds are KiCad designs, so a seed finding from such a rule can be checked rather
// than asserted (agni issue 967). hack/kicad_oracle.sh writes KiCad's reports beside the triage
// records, since CI has no kicad-cli, and TestSeedTriageAgainstKiCad holds each record to them in both
// directions. A group of a mirrored rule says `kicad: agrees` or `kicad: disagrees`, and a KiCad
// violation agni does not report is recorded under `kicad_only`, keyed by KiCad's check.

// kicadMirrors are the agni rules that answer a KiCad check. A KiCad violation whose subject one of
// them reports is a finding KiCad agrees with. pin_to_pin mirrors output-output-conflict only for two
// outputs, since KiCad's matrix also reports pairs agni's has no rule for.
func kicadMirrors(check, description string) []string {
	switch check {
	case "power_pin_not_driven":
		return []string{"power-input-not-driven"}
	case "pin_not_connected":
		return []string{"single-pin-net", "unconnected-pin"}
	case "duplicate_reference":
		return []string{"duplicate-ref-des"}
	case "wire_dangling":
		return []string{"wire-no-junction"}
	case "track_width":
		return []string{"track-width"}
	case "drill_out_of_range":
		return []string{"hole-size"}
	case "annular_width":
		return []string{"annular-width"}
	case "clearance":
		return []string{"copper-clearance"}
	case "pin_to_pin":
		if strings.Contains(description, "Output and Output") {
			return []string{"output-output-conflict"}
		}
	}
	return nil
}

// kicadUnmirrored are the KiCad checks no agni rule answers, each with the reason. A check in an
// oracle that is neither mirrored nor listed here fails the test, so a new KiCad report is read before
// it is ignored.
var kicadUnmirrored = map[string]string{
	"pin_to_pin":                "only two outputs are mirrored; agni has no rule for a power output on a bidirectional pin",
	"label_dangling":            "agni reads nets, not the labels that name them",
	"unconnected_wire_endpoint": "agni reads nets, not the wires that draw them",
	"endpoint_off_grid":         "a drawing-grid check",
	"multiple_net_names":        "agni names the net as KiCad's netlist does and reports no alias",
	"copper_edge_clearance":     "agni reads no board outline clearance",
	"silk_edge_clearance":       "agni checks no silkscreen",
	"silk_over_copper":          "agni checks no silkscreen",
	"courtyards_overlap":        "agni checks no courtyards",
	"pth_inside_courtyard":      "agni checks no courtyards",
	"npth_inside_courtyard":     "agni checks no courtyards",
	"holes_co_located":          "agni checks no hole placement",
	"track_dangling":            "agni checks no track ends",
	"unconnected_items":         "agni has no rule for a connection the board leaves unrouted",
}

// kicadOnlyClasses are the judgements a KiCad violation agni does not report can carry: agni's rule
// declines it on purpose, agni misses it, agni reads no input that holds it, or KiCad reads the file
// differently from agni. All but a decline name the ticket that settles them. A group whose subject is
// `*` covers every subject KiCad reports for its check, for a check whose violations share one cause.
var kicadOnlyClasses = map[string]bool{"declined": true, "missed": true, "unread": true, "differs": true}

// kicadViolation is one oracle line agni has a mirroring rule for.
type kicadViolation struct {
	check, subject string
	rules          []string
}

// loadKiCadOracle reads an oracle file, returning the violations a rule mirrors and failing on a
// check it has no word for.
func loadKiCadOracle(t *testing.T, path string) []kicadViolation {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("%v (hack/kicad_oracle.sh writes it)", err)
	}
	defer f.Close()
	var out []kicadViolation
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "#") {
			continue
		}
		cols := strings.Split(sc.Text(), "\t")
		if len(cols) < 5 {
			t.Fatalf("%s: malformed line %q", path, sc.Text())
		}
		check, subject, desc := cols[1], cols[3], cols[4]
		rules := kicadMirrors(check, desc)
		if rules == nil {
			if _, ok := kicadUnmirrored[check]; !ok {
				t.Errorf("%s: KiCad check %q is neither mirrored by an agni rule nor listed as unmirrored", path, check)
			}
			continue
		}
		if subject == "-" {
			t.Errorf("%s: a %s violation resolved to no subject: %q", path, check, sc.Text())
			continue
		}
		out = append(out, kicadViolation{check, subject, rules})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSeedTriageAgainstKiCad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mirrored := map[string]bool{}
	for _, r := range []string{"power-input-not-driven", "single-pin-net", "unconnected-pin", "duplicate-ref-des", "wire-no-junction", "track-width", "hole-size", "annular-width", "copper-clearance", "output-output-conflict"} {
		mirrored[r] = true
	}

	designs := seedDesignDirs(t)
	names := make([]string, 0, len(designs))
	for n := range designs {
		names = append(names, n)
	}
	sort.Strings(names)
	agreed, compared := 0, 0
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			rec, err := loadTriage(filepath.Join(seedTriageDir, name+".triage.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			oracle := loadKiCadOracle(t, filepath.Join(seedTriageDir, name+".kicad.oracle"))
			compared += len(oracle)
			// kicadSays is each (rule, subject) KiCad reports, and byCheck the subjects per KiCad check.
			kicadSays := map[[2]string]bool{}
			byCheck := map[string]map[string][]string{}
			for _, v := range oracle {
				for _, r := range v.rules {
					kicadSays[[2]string{r, v.subject}] = true
				}
				if byCheck[v.check] == nil {
					byCheck[v.check] = map[string][]string{}
				}
				byCheck[v.check][v.subject] = v.rules
			}
			found := map[[2]string]bool{}
			for f := range seedFindings(t, rec.Design) {
				found[[2]string{f.rule, f.subjects}] = true
			}

			for rule, groups := range rec.Findings {
				for i, g := range groups {
					where := fmt.Sprintf("%s group %d", rule, i+1)
					if !mirrored[rule] {
						if g.Kicad != "" {
							t.Errorf("%s: says kicad: %s, but no KiCad check mirrors %s", where, g.Kicad, rule)
						}
						continue
					}
					if g.Kicad != "agrees" && g.Kicad != "disagrees" {
						t.Errorf("%s: a rule KiCad mirrors needs kicad: agrees or kicad: disagrees, has %q", where, g.Kicad)
						continue
					}
					for _, s := range g.Subjects {
						says := kicadSays[[2]string{rule, s}]
						if says != (g.Kicad == "agrees") {
							t.Errorf("%s: %s says kicad: %s, and KiCad reporting it is %v", where, s, g.Kicad, says)
						}
						if says {
							agreed++
						}
					}
				}
			}

			recorded := map[[2]string]bool{}
			for check, groups := range rec.KicadOnly {
				for i, g := range groups {
					where := fmt.Sprintf("kicad_only %s group %d", check, i+1)
					if !kicadOnlyClasses[g.Class] {
						t.Errorf("%s: class %q is not one of declined, missed, unread, differs", where, g.Class)
					}
					if g.Class != "declined" && g.Ticket == 0 {
						t.Errorf("%s: a %s violation needs the ticket that closes it", where, g.Class)
					}
					if strings.TrimSpace(g.Why) == "" {
						t.Errorf("%s: no why", where)
					}
					subjects := g.Subjects
					if len(subjects) == 1 && subjects[0] == "*" {
						if len(byCheck[check]) == 0 {
							t.Errorf("%s: KiCad no longer reports any %s violation", where, check)
						}
						subjects = nil
						for s := range byCheck[check] {
							subjects = append(subjects, s)
						}
					}
					for _, s := range subjects {
						recorded[[2]string{check, s}] = true
						rules, ok := byCheck[check][s]
						if !ok {
							t.Errorf("%s: KiCad no longer reports %s", where, s)
						}
						for _, r := range rules {
							if found[[2]string{r, s}] {
								t.Errorf("%s: %s now reports %s, so it is no longer KiCad's alone", where, r, s)
							}
						}
					}
				}
			}
			var missing []string
			for check, subjects := range byCheck {
				for s, rules := range subjects {
					reported := false
					for _, r := range rules {
						reported = reported || found[[2]string{r, s}]
					}
					if !reported && !recorded[[2]string{check, s}] {
						missing = append(missing, check+" "+s)
					}
				}
			}
			sort.Strings(missing)
			for _, m := range missing {
				t.Errorf("KiCad reports %s and agni does not; record it under kicad_only", m)
			}
		})
	}
	// Without these the comparison could pass over oracles that mirror nothing.
	if compared == 0 || agreed == 0 {
		t.Fatalf("compared %d KiCad violations and agreed on %d findings, want both above zero", compared, agreed)
	}
}
