package fshost

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panyam/agni/core/check"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	_ "github.com/panyam/agni/stdlib/rules/builtin"
	"google.golang.org/protobuf/proto"
)

const countedRule = "test/counted"

// checkFixture is the tutorial project copied into a temp folder, so a test can edit its files, and
// a check service over it whose catalog carries one rule that counts its evaluations.
type checkFixture struct {
	dir    string
	evals  *atomic.Int64
	svc    *service.CheckService
	design string
}

// newCheckFixture builds the fixture. outcome is what the counted rule answers, and cached false
// gives the same service with no DesignCache, the answer a check owes whether or not anything is kept.
func newCheckFixture(t *testing.T, outcome check.Outcome, cached bool, wrap func(service.ConfigResolver) service.ConfigResolver) *checkFixture {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "examples", "tutorial-project"))); err != nil {
		t.Fatal(err)
	}
	evals := &atomic.Int64{}
	counted := &check.Rule{
		Name:     "counted",
		Severity: "info",
		Summary:  "counts its evaluations",
		// So its verdicts reach the response, which is where an inconclusive one is seen.
		StatesConsideredSet: true,
		Eval: func(context.Context, check.Model) []check.Verdict {
			evals.Add(1)
			return []check.Verdict{{Outcome: outcome, Subjects: []check.Entity{{Kind: check.KindNet, Ref: "GND"}}, Reason: "counted"}}
		},
	}
	cat, err := check.DefaultCatalog().With(check.NewSource("test", []*check.Rule{counted}))
	if err != nil {
		t.Fatal(err)
	}
	ms := []Mount{{Name: "m", FS: os.DirFS(dir)}}
	var resolver service.ConfigResolver = &ConfigResolver{Mounts: ms}
	if wrap != nil {
		resolver = wrap(resolver)
	}
	res := &service.ProjectResolver{Store: projects.NewFSStore(projects.Tree{Mount: "m", FS: ms[0].FS}), Config: resolver}
	var c *service.DesignCache
	if cached {
		// Room for three overlays' designs and models over one parse, so nothing here is evicted.
		c = service.NewDesignCache(10)
	}
	loader := service.NewCachingLoader(New(ms...), c)
	svc := service.NewCheckService(loader, cat, nil, "", nil, res)
	return &checkFixture{dir: dir, evals: evals, svc: svc, design: "mount://m/designs/gateway"}
}

func (f *checkFixture) check(t *testing.T, req *webapi.CheckDesignRequest) *webapi.CheckDesignResponse {
	t.Helper()
	if req.Uri == "" {
		req.Uri = f.design
	}
	resp, err := f.svc.CheckDesign(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (f *checkFixture) counted(t *testing.T, req *webapi.CheckDesignRequest) {
	t.Helper()
	req.Rules = []string{countedRule}
	f.check(t, req)
}

// touch rewrites a file in the fixture with one byte more and a later time, so its stamp moves
// however coarse the filesystem's clock.
func (f *checkFixture) touch(t *testing.T, rel string) {
	t.Helper()
	p := filepath.Join(f.dir, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(p, later, later)
}

func (f *checkFixture) wantEvals(t *testing.T, want int64, why string) {
	t.Helper()
	if got := f.evals.Load(); got != want {
		t.Fatalf("the rule was evaluated %d times, want %d: %s", got, want, why)
	}
}

// TestASecondCheckDoesNotEvaluateTheRuleAgain is agni issue 390's first acceptance: the same check
// asked twice evaluates its rule once.
func TestASecondCheckDoesNotEvaluateTheRuleAgain(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, nil)
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 1, "the first check")
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 1, "an identical second check ran the rule again")
}

// TestEditingAnyTierEvaluatesTheRuleAgain edits the netlist, then the board, which the issue's first
// key (the entry's hash alone) would have missed.
func TestEditingAnyTierEvaluatesTheRuleAgain(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, nil)
	f.counted(t, &webapi.CheckDesignRequest{})
	f.touch(t, "designs/gateway/gateway.edn")
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 2, "an edited netlist was answered from the run before the edit")
	f.touch(t, "designs/gateway/gateway.kicad_pcb")
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 3, "an edited board was answered from the run before the edit")
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 3, "the check after the edits ran the rule again, so the edits are not what moved the count")
}

// TestADifferentConventionEvaluatesTheRuleAgain: a convention changes the lexicon every rule reads,
// so a run under one must never answer for a run under another.
func TestADifferentConventionEvaluatesTheRuleAgain(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, nil)
	f.counted(t, &webapi.CheckDesignRequest{})
	conv := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: &configpb.NamingConvention{Name: "other"}}}
	f.counted(t, &webapi.CheckDesignRequest{Overlay: conv})
	f.wantEvals(t, 2, "a check under another convention was answered from the project's")
	f.counted(t, &webapi.CheckDesignRequest{Overlay: &webapi.OverlayConfig{IgnoreProject: true}})
	f.wantEvals(t, 3, "a check ignoring the project was answered from the project's")
	f.counted(t, &webapi.CheckDesignRequest{Overlay: conv})
	f.wantEvals(t, 3, "the second check under the other convention ran the rule again")
}

// noDigest is a config resolver that does not say what it read, which leaves an overlay built on it
// unable to identify itself.
type noDigest struct{ service.ConfigResolver }

func (r noDigest) ResolveConfig(ctx context.Context, cfg *webapi.AnalysisConfig, ns string) (service.ResolvedConfig, error) {
	rc, err := r.ConfigResolver.ResolveConfig(ctx, cfg, ns)
	rc.Digest = ""
	return rc, err
}

// TestAnOverlayThatDoesNotIdentifyIsNeverAnsweredFromAKeptRun: with no identity there is nothing to
// tell one configuration from another, so every check evaluates.
func TestAnOverlayThatDoesNotIdentifyIsNeverAnsweredFromAKeptRun(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, func(r service.ConfigResolver) service.ConfigResolver { return noDigest{r} })
	f.counted(t, &webapi.CheckDesignRequest{})
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 2, "a check whose overlay has no identity was answered from a kept run")
}

// TestABudgetedInconclusiveRuleIsNotKept: under a work budget an inconclusive answer may be the
// budget's doing, and a later request is owed the real one. Without a budget it is the answer.
func TestABudgetedInconclusiveRuleIsNotKept(t *testing.T) {
	f := newCheckFixture(t, check.Inconclusive, true, nil)
	f.counted(t, &webapi.CheckDesignRequest{WorkBudget: 1_000_000})
	f.counted(t, &webapi.CheckDesignRequest{WorkBudget: 1_000_000})
	f.wantEvals(t, 2, "an inconclusive answer under a budget was kept")
	f.counted(t, &webapi.CheckDesignRequest{})
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 3, "an inconclusive answer with no budget was not kept")
}

// TestAKeptRunAnswersAsAFreshOneDoes runs part of the catalog and then all of it, so the second
// answer joins kept rules to rules run now, and compares it with the whole catalog run on a service
// that keeps nothing. Every finding and verdict must match, in order.
func TestAKeptRunAnswersAsAFreshOneDoes(t *testing.T) {
	kept := newCheckFixture(t, check.Fail, true, nil)
	fresh := newCheckFixture(t, check.Fail, false, nil)
	whole := fresh.check(t, &webapi.CheckDesignRequest{})
	if len(whole.GetFindings()) < 5 || len(whole.GetVerdicts()) < 20 {
		t.Fatalf("the fixture answered %d findings and %d verdicts, too few for an order to be worth comparing", len(whole.GetFindings()), len(whole.GetVerdicts()))
	}
	// Every other rule that fired, so the kept and the new rules interleave by name.
	seen := map[string]bool{}
	var half []string
	for _, fd := range whole.GetFindings() {
		if !seen[fd.GetRule()] {
			seen[fd.GetRule()] = true
			if len(seen)%2 == 1 {
				half = append(half, fd.GetRule())
			}
		}
	}
	kept.check(t, &webapi.CheckDesignRequest{Rules: half})
	got := kept.check(t, &webapi.CheckDesignRequest{})
	if !proto.Equal(got, whole) {
		t.Fatalf("a check joining %d kept rules to the rest answered differently from a fresh one:\n%d findings, %d verdicts, %d skipped\nwant %d, %d, %d",
			len(half), len(got.GetFindings()), len(got.GetVerdicts()), len(got.GetSkipped()),
			len(whole.GetFindings()), len(whole.GetVerdicts()), len(whole.GetSkipped()))
	}
	if kept.evals.Load() != 1 {
		t.Fatalf("the counted rule ran %d times on the kept service, want 1", kept.evals.Load())
	}
}
