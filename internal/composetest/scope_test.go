package composetest

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"
)

// A run asked about some nets or parts (agni issue 897) must answer for each exactly as a whole run
// filtered to it does, for every rule in the composed catalog. Per rule, it asks about a spread of
// the subjects the whole run named and one entity the rule said nothing about, and compares the two
// answers in order. A rule declaring ScopeSubjects is also run outside check.RunAll, so one that
// declares it and still walks the whole design fails here rather than only running slowly.
func TestScopedRunMatchesFilteredFullRun(t *testing.T) {
	e, err := agni.New()
	if err != nil {
		t.Fatal(err)
	}
	rules := e.Catalog().Rules()
	for name, path := range map[string]string{
		"gateway": "../../examples/tutorial-project/designs/gateway/gateway.edn",
		"jetson":  "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_sch",
		"feather": "../../tools/samples/boards/royalblue54L-feather/RoyalBlue54L-Feather.kicad_sch",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := os.Stat(path); err != nil {
				// Fatal rather than skipped: a missing corpus must not read as a pass.
				t.Fatalf("%s missing (run `make samples-oracle` for the sample boards): %v", path, err)
			}
			d, err := (&formats.Loader{}).ReadDesign(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			// Each side gets its own model, so nothing one side cached can make the other agree.
			full, scoped := check.NewModel(d), check.NewModel(d)
			fullF, fullV, err := check.RunAll(ctx, full, rules)
			if err != nil {
				t.Fatal(err)
			}
			asked, narrowed := 0, 0
			for _, r := range rules {
				if r.Scoping == check.ScopeDesignWide {
					continue
				}
				rf, rv := ofRule(r.Name, fullF), ofRuleV(r.Name, fullV)
				for _, v := range rv {
					if !namesScopable(v.Subjects) && len(v.Subjects) > 0 {
						t.Errorf("%s: verdict %s names no net, part or pin, so a scoped run keeps it for every subject; declare check.ScopeDesignWide", r.Name, check.VerdictID(v))
						break
					}
				}
				for _, ent := range askAbout(rf, rv, full) {
					s := check.NewScope(ent)
					sctx := check.WithSubjectScope(ctx, s)
					gotF, gotV, err := check.RunAll(sctx, scoped, []*check.Rule{r})
					if err != nil {
						t.Fatal(err)
					}
					wantF, wantV := s.Results(rf, rv)
					if !sameFindings(gotF, wantF) || !sameVerdicts(gotV, wantV) {
						t.Errorf("%s about %s:%s: scoped run gave %d findings and %d verdicts, the filtered whole run %d and %d",
							r.Name, ent.Kind, check.EntityRef(ent), len(gotF), len(gotV), len(wantF), len(wantV))
					}
					asked++
					if r.Scoping == check.ScopeSubjects {
						raw := r.Eval(sctx, scoped)
						if _, in := s.Results(nil, raw); len(in) != len(raw) {
							t.Errorf("%s declares ScopeSubjects but, asked about %s, evaluated %d subjects outside the scope",
								r.Name, check.EntityRef(ent), len(raw)-len(in))
						}
						narrowed++
					}
				}
			}
			// A loop over no subjects passes over nothing.
			if asked < 100 || narrowed < 20 {
				t.Fatalf("asked %d scoped questions, %d of them of rules that read the scope; too few to prove anything", asked, narrowed)
			}
		})
	}
}

// askAbout picks the entities to ask one rule about: up to eight of the subjects its whole run named,
// spread across them, and a net and a part it named nothing about, so a scoped run that invents an
// answer fails as well as one that drops one.
func askAbout(fs []check.Finding, vs []check.Verdict, m check.Model) []check.Entity {
	seen := map[string]bool{}
	var named []check.Entity
	add := func(e check.Entity) {
		switch e.Kind {
		case check.KindNet, check.KindComponent, check.KindPin:
		default:
			return
		}
		k := e.Kind + ":" + check.EntityRef(e)
		if !seen[k] {
			seen[k] = true
			named = append(named, e)
		}
	}
	for _, v := range vs {
		for _, e := range v.Subjects {
			add(e)
		}
	}
	for _, f := range fs {
		add(f.Subject)
	}
	sort.Slice(named, func(i, j int) bool {
		return named[i].Kind+check.EntityRef(named[i]) < named[j].Kind+check.EntityRef(named[j])
	})
	var out []check.Entity
	const most = 8
	for i := 0; i < len(named) && len(out) < most; i++ {
		out = append(out, named[i*len(named)/min(len(named), most)])
	}
	for _, n := range m.Nets() {
		if !seen[check.KindNet+":"+n.GetName()] {
			out = append(out, check.NetEntity(n))
			break
		}
	}
	for _, c := range m.Components() {
		if !seen[check.KindComponent+":"+c.GetRefDes()] {
			out = append(out, check.ComponentEntity(c.GetRefDes()))
			break
		}
	}
	return out
}

func namesScopable(es []check.Entity) bool {
	for _, e := range es {
		switch e.Kind {
		case check.KindNet, check.KindComponent, check.KindPin:
			return true
		}
	}
	return false
}

func ofRule(rule string, fs []check.Finding) []check.Finding {
	var out []check.Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func ofRuleV(rule string, vs []check.Verdict) []check.Verdict {
	var out []check.Verdict
	for _, v := range vs {
		if v.Rule == rule {
			out = append(out, v)
		}
	}
	return out
}

// sameFindings and sameVerdicts compare in order, with an empty list equal to a nil one.
func sameFindings(a, b []check.Finding) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}

func sameVerdicts(a, b []check.Verdict) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}
