package diff

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// classed builds an ir.Component carrying normalized device classes, which is how the near-match
// pass decides an endpoint is insignificant.
func classed(refDes string, classes ...string) *ir.Component {
	return &ir.Component{RefDes: refDes, DeviceClasses: classes}
}

// probes is the component set the near-rename tests share: three device parts and two test points,
// so a fixture can add or drop a probe without changing any significant endpoint.
func probes() []*ir.Component {
	return []*ir.Component{
		classed("U1", "ic"), classed("U2", "ic"), classed("C5", "capacitor"),
		classed("TP7", "test_point"), classed("PROBE9", "test_point"),
	}
}

func approxChanges(r *Report) []NetChange {
	var out []NetChange
	for _, nc := range r.Nets {
		if nc.Kind == NetRenamedApprox {
			out = append(out, nc)
		}
	}
	return out
}

func kindOf(r *Report, name string) NetChangeKind {
	for _, nc := range r.Nets {
		if nc.Name == name {
			return nc.Kind
		}
	}
	return ""
}

func TestNearRenameRecoversARenameThatAlsoGainedAProbe(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("XTAL_IN", "old", "U1.4", "U2.1", "C5.1"),
	}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("CLK_IN", "new", "U1.4", "U2.1", "C5.1", "TP7.1"),
	}}

	if got := approxChanges(Designs(a, b)); len(got) != 0 {
		t.Fatalf("near-match must not run unless enabled, got %d: %+v", len(got), got)
	}

	opts := DefaultRenameOptions()
	opts.Enabled = true
	got := approxChanges(Designs(a, b, opts))
	if len(got) != 1 {
		t.Fatalf("want 1 approximate rename, got %d: %+v", len(got), got)
	}
	if got[0].OldName != "XTAL_IN" || got[0].Name != "CLK_IN" {
		t.Errorf("paired %s -> %s, want XTAL_IN -> CLK_IN", got[0].OldName, got[0].Name)
	}
	if len(got[0].Added) != 1 || got[0].Added[0] != "TP7.1" {
		t.Errorf("Added = %v, want the one gained endpoint", got[0].Added)
	}
	if len(got[0].Removed) != 0 {
		t.Errorf("Removed = %v, want none", got[0].Removed)
	}
	// The probe is what makes this the interesting case: it moved the all-endpoint coverage off 1.0
	// while leaving every device endpoint in place, which is the asymmetry the significant thresholds
	// exist to see through.
	ev := got[0].Approx
	if ev == nil {
		t.Fatal("an approximate rename must carry its evidence")
	}
	if ev.OldCoverageSignificant != 1.0 {
		t.Errorf("significant coverage = %v, want 1.0: no device endpoint moved", ev.OldCoverageSignificant)
	}
	if ev.OldCoverage != 1.0 || ev.NewCoverageSignificant != 1.0 {
		t.Errorf("coverages = %v / %v", ev.OldCoverage, ev.NewCoverageSignificant)
	}
	if ev.OverlapSignificant != 3 || ev.OldEndpoints != 3 || ev.NewEndpoints != 4 {
		t.Errorf("overlap %d, sizes %d -> %d", ev.OverlapSignificant, ev.OldEndpoints, ev.NewEndpoints)
	}
}

// The negative control, and the one that matters more than the positive. A pass that pairs
// everything scores perfectly on "did it find the rename" and is useless.
func TestNearRenameDoesNotPairUnrelatedNets(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("RAIL_A", "old", "U1.1", "U1.2", "C5.1"),
		net("RAIL_B", "old", "U2.1", "U2.2", "C5.2"),
	}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("RAIL_C", "new", "U1.5", "U1.6", "C5.3"),
		net("RAIL_D", "new", "U2.5", "U2.6", "C5.4"),
	}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	r := Designs(a, b, opts)

	if got := approxChanges(r); len(got) != 0 {
		t.Fatalf("unrelated nets must not pair, got %d: %+v", len(got), got)
	}
	for _, n := range []string{"RAIL_A", "RAIL_B"} {
		if k := kindOf(r, n); k != NetDeleted {
			t.Errorf("%s = %q, want deleted", n, k)
		}
	}
	for _, n := range []string{"RAIL_C", "RAIL_D"} {
		if k := kindOf(r, n); k != NetNew {
			t.Errorf("%s = %q, want new", n, k)
		}
	}
}

// The passes must stay ordered: an unchanged rename is a FACT about the two revisions and must never
// be downgraded to the pass that guesses.
func TestExactRenameStillWinsWhenNearMatchIsEnabled(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{net("OLD", "old", "U1.1", "U2.1", "C5.1")}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{net("NEW", "new", "U1.1", "U2.1", "C5.1")}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	r := Designs(a, b, opts)

	if len(r.Nets) != 1 {
		t.Fatalf("want one change, got %d: %+v", len(r.Nets), r.Nets)
	}
	if r.Nets[0].Kind != NetRenamed {
		t.Errorf("kind = %q, want exact %q", r.Nets[0].Kind, NetRenamed)
	}
	if r.Nets[0].Approx != nil {
		t.Error("an exact rename carries no near-match evidence")
	}
}

// A large net that happens to contain all of a small one is not its rename. Without the new-coverage
// guard the old net's coverage is a perfect 1.0 and the pairing looks excellent.
func TestNearRenameRejectsASupersetSwallowingASmallNet(t *testing.T) {
	// Sized so ONLY the new-coverage guard can reject it. Two significant endpoints gain two more,
	// which is exactly MaxAddedSignificantFloor, so the growth guard passes and every old endpoint
	// survives, so old coverage is a perfect 1.0. What disqualifies it is that the old endpoints make
	// up half of the new net, under the 0.60 significant floor.
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{net("SMALL", "old", "U1.1", "U1.2")}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("BIG", "new", "U1.1", "U1.2", "U2.1", "C5.1"),
	}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	if got := approxChanges(Designs(a, b, opts)); len(got) != 0 {
		t.Fatalf("a superset must not read as a rename, got %+v", got)
	}

	// Positive control for the same fixture: relax only the new-coverage floors and it pairs, which
	// is what proves the rejection above came from those and not from another threshold.
	loose := opts
	loose.MinNewCoverage, loose.MinNewCoverageSignificant = 0.0, 0.0
	if got := approxChanges(Designs(a, b, loose)); len(got) != 1 {
		t.Fatalf("with the new-coverage floors removed this pairs, got %d: %+v", len(got), got)
	}
}

// Significance comes from the normalized device class, not from how a ref des is spelled. PROBE9 is
// a test point that no prefix rule would catch.
func TestNearRenameReadsSignificanceFromDeviceClassNotRefDesSpelling(t *testing.T) {
	// Four probes, none of them spelled "TP", added to a two-endpoint net in one revision. Read by
	// class they are all insignificant and nothing grew, so the rename is recovered. Read by ref-des
	// prefix they are four new device endpoints against a growth allowance of two, and the pairing is
	// rejected. The two rules disagree about the OUTCOME here, not about a fraction.
	comps := append(probes(),
		classed("PROBE1", "test_point"), classed("PROBE2", "test_point"),
		classed("PROBE3", "test_point"), classed("PROBE4", "test_point"))
	a := &ir.Design{Components: comps, Nets: []*ir.Net{net("SIG_OLD", "old", "U1.4", "U2.1")}}
	b := &ir.Design{Components: comps, Nets: []*ir.Net{
		net("SIG_NEW", "new", "U1.4", "U2.1", "PROBE1.1", "PROBE2.1", "PROBE3.1"),
	}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	got := approxChanges(Designs(a, b, opts))
	if len(got) != 1 {
		t.Fatalf("probes are insignificant however they are spelled, got %d: %+v", len(got), got)
	}
	ev := got[0].Approx
	if ev.OldCoverageSignificant != 1.0 || ev.NewCoverageSignificant != 1.0 {
		t.Errorf("significant coverages = %v / %v, want 1.0 / 1.0: no device endpoint moved",
			ev.OldCoverageSignificant, ev.NewCoverageSignificant)
	}
	// All-endpoint coverage still sees the probes, which is what makes the significant pair the
	// threshold doing the work.
	if ev.OldCoverage != 1.0 || ev.NewEndpoints != 5 {
		t.Errorf("old coverage %v over %d new endpoints", ev.OldCoverage, ev.NewEndpoints)
	}
}

// A characterized limit of the calibrated defaults, asserted so that changing it is a decision
// rather than an accident.
//
// MinNewCoverage counts ALL endpoints, probes included, so a small net that gains enough probes
// falls under it however insignificant those probes are. Three probes on a two-endpoint net gives
// 2/5 and pairs; a fourth gives 2/6 and does not. The significant thresholds insulate the pass from
// probe churn and this one does not, which is a real seam in the defaults rather than a bug in the
// implementation: the reference numbers behave the same way, and moving MinNewCoverage without a
// precision run would trade a known behaviour for an unknown one.
func TestProbeChurnCanStillSinkAPairingThroughTheAllEndpointFloor(t *testing.T) {
	comps := append(probes(),
		classed("PROBE1", "test_point"), classed("PROBE2", "test_point"),
		classed("PROBE3", "test_point"), classed("PROBE4", "test_point"))
	opts := DefaultRenameOptions()
	opts.Enabled = true

	build := func(probeRefs ...string) *ir.Design {
		conns := append([]string{"U1.4", "U2.1"}, probeRefs...)
		return &ir.Design{Components: comps, Nets: []*ir.Net{net("SIG_NEW", "new", conns...)}}
	}
	a := &ir.Design{Components: comps, Nets: []*ir.Net{net("SIG_OLD", "old", "U1.4", "U2.1")}}

	if got := approxChanges(Designs(a, build("PROBE1.1", "PROBE2.1", "PROBE3.1"), opts)); len(got) != 1 {
		t.Errorf("three probes is 2/5 and clears the 0.35 floor, got %d", len(got))
	}
	if got := approxChanges(Designs(a, build("PROBE1.1", "PROBE2.1", "PROBE3.1", "PROBE4.1"), opts)); len(got) != 0 {
		t.Errorf("four probes is 2/6 and falls under it, got %d", len(got))
	}
}

// Two equally good candidates must resolve the same way every run, or the diff of two fixed
// revisions stops being a function of those revisions.
func TestNearRenameIsDeterministicAcrossRuns(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("OLD_A", "old", "U1.1", "U1.2", "C5.1"),
		net("OLD_B", "old", "U1.1", "U1.2", "C5.1"),
	}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("NEW_A", "new", "U1.1", "U1.2", "C5.1", "TP7.1"),
		net("NEW_B", "new", "U1.1", "U1.2", "C5.1", "PROBE9.1"),
	}}
	opts := DefaultRenameOptions()
	opts.Enabled = true

	first := approxChanges(Designs(a, b, opts))
	for i := 0; i < 20; i++ {
		got := approxChanges(Designs(a, b, opts))
		if len(got) != len(first) {
			t.Fatalf("run %d produced %d pairings, first produced %d", i, len(got), len(first))
		}
		for j := range got {
			if got[j].OldName != first[j].OldName || got[j].Name != first[j].Name {
				t.Fatalf("run %d paired %s->%s where the first paired %s->%s",
					i, got[j].OldName, got[j].Name, first[j].OldName, first[j].Name)
			}
		}
	}
}

// A net is assigned at most once on each side, so an old net with two plausible successors becomes
// one rename and one addition rather than two renames of the same net.
//
// Both candidates differ from the old net, so the exact pass claims neither and the near pass really
// has to choose. A version of this test where one candidate matched exactly would prove nothing,
// because the exact pass would take it and leave the near pass with a single option.
func TestNearRenameAssignsOneToOne(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{net("OLD", "old", "U1.1", "U1.2", "C5.1")}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("NEW_1", "new", "U1.1", "U1.2", "C5.1", "TP7.1"),
		net("NEW_2", "new", "U1.1", "U1.2", "C5.1", "PROBE9.1"),
	}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	r := Designs(a, b, opts)

	got := approxChanges(r)
	if len(got) != 1 {
		t.Fatalf("one old net pairs once, got %d: %+v", len(got), got)
	}
	if got[0].OldName != "OLD" {
		t.Errorf("paired from %q, want OLD", got[0].OldName)
	}
	// Whichever candidate lost is an ordinary addition, not a second rename of the same net.
	loser := "NEW_2"
	if got[0].Name == "NEW_2" {
		loser = "NEW_1"
	}
	if k := kindOf(r, loser); k != NetNew {
		t.Errorf("%s = %q, want new", loser, k)
	}
}

// The exact pass takes precedence when one candidate matches exactly, leaving the near pass nothing
// to do. This is the ordering guarantee stated from the assignment side.
func TestExactMatchClaimsTheCandidateBeforeTheNearPassSeesIt(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{net("OLD", "old", "U1.1", "U1.2", "C5.1")}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{
		net("NEW_1", "new", "U1.1", "U1.2", "C5.1", "TP7.1"),
		net("NEW_2", "new", "U1.1", "U1.2", "C5.1"),
	}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	r := Designs(a, b, opts)

	if k := kindOf(r, "NEW_2"); k != NetRenamed {
		t.Errorf("NEW_2 = %q, want the exact rename to claim it", k)
	}
	if got := approxChanges(r); len(got) != 0 {
		t.Errorf("nothing is left for the near pass, got %+v", got)
	}
	if k := kindOf(r, "NEW_1"); k != NetNew {
		t.Errorf("NEW_1 = %q, want new", k)
	}
}

// A one-endpoint net has no shape to match on, so it is never a near-rename candidate however well
// its single endpoint lines up.
func TestNearRenameSkipsNetsBelowTheSignificantFloor(t *testing.T) {
	a := &ir.Design{Components: probes(), Nets: []*ir.Net{net("TINY_OLD", "old", "U1.1")}}
	b := &ir.Design{Components: probes(), Nets: []*ir.Net{net("TINY_NEW", "new", "U1.1", "TP7.1")}}
	opts := DefaultRenameOptions()
	opts.Enabled = true
	if got := approxChanges(Designs(a, b, opts)); len(got) != 0 {
		t.Fatalf("a one-endpoint net has no shape to match, got %+v", got)
	}
}

func TestRequiredOverlapTracksTheThreshold(t *testing.T) {
	opts := DefaultRenameOptions()
	// Derived from MinOldCoverageSignificant rather than a constant of its own: a prefilter stricter
	// than the threshold would drop candidates that would have passed scoring, and the knob would
	// then appear not to work.
	for _, tc := range []struct{ significant, want int }{
		{2, 2}, {3, 3}, {4, 4}, {5, 4}, {10, 8}, {1, 2},
	} {
		if got := requiredOverlap(tc.significant, opts); got != tc.want {
			t.Errorf("requiredOverlap(%d) = %d, want %d", tc.significant, got, tc.want)
		}
	}
	loose := opts
	loose.MinOldCoverageSignificant = 0.5
	if got := requiredOverlap(10, loose); got != 5 {
		t.Errorf("loosening the threshold must loosen the prefilter: got %d, want 5", got)
	}
}

// The growth guard in isolation. Six significant endpoints all survive, so both old coverages are
// 1.0, and the four gained endpoints leave the new coverages at exactly 0.60, on the floor rather
// than under it. The only threshold left to reject it is how much a net may GROW and still read as
// itself: four added against an allowance of max(2, 6/2) = 3.
func TestNearRenameRejectsANetThatGrewPastItsAllowance(t *testing.T) {
	comps := []*ir.Component{
		classed("U1", "ic"), classed("U2", "ic"), classed("U3", "ic"),
		classed("U4", "ic"), classed("U5", "ic"),
	}
	old := []string{"U1.1", "U1.2", "U2.1", "U2.2", "U3.1", "U3.2"}
	grown := append(append([]string{}, old...), "U4.1", "U4.2", "U5.1", "U5.2")

	a := &ir.Design{Components: comps, Nets: []*ir.Net{net("BUS_OLD", "old", old...)}}
	b := &ir.Design{Components: comps, Nets: []*ir.Net{net("BUS_NEW", "new", grown...)}}
	opts := DefaultRenameOptions()
	opts.Enabled = true

	if got := approxChanges(Designs(a, b, opts)); len(got) != 0 {
		t.Fatalf("a net that grew past its allowance is not itself renamed, got %+v", got)
	}
	// Positive control: raise ONLY the growth allowance and the same pairing is accepted, which is
	// what proves the rejection came from that guard and not from a coverage floor.
	loose := opts
	loose.MaxAddedSignificantFloor = 4
	if got := approxChanges(Designs(a, b, loose)); len(got) != 1 {
		t.Fatalf("with the allowance raised this pairs, got %d: %+v", len(got), got)
	}
}

// scoreRename's endpoint floor is tested directly because the candidate prefilter enforces the same
// bound before scoring ever runs, so no whole-design fixture can reach it. Testing it through
// Designs would pass whatever this guard did.
func TestScoreRenameEnforcesTheSignificantFloorItself(t *testing.T) {
	opts := DefaultRenameOptions()
	set := func(keys ...string) map[string]bool {
		m := map[string]bool{}
		for _, k := range keys {
			m[k] = true
		}
		return m
	}
	oldConns, newConns := set("U1.1", "TP7.1"), set("U1.1", "TP7.1")

	// One significant endpoint on each side, perfectly overlapping. Every coverage is 1.0 and the
	// pairing is still refused, because one endpoint is not a shape.
	if _, ok := scoreRename(oldConns, newConns, set("U1.1"), set("U1.1"), opts); ok {
		t.Error("a single significant endpoint must not score, however well it overlaps")
	}
	// Two makes it a shape.
	oldConns, newConns = set("U1.1", "U2.1"), set("U1.1", "U2.1")
	if _, ok := scoreRename(oldConns, newConns, set("U1.1", "U2.1"), set("U1.1", "U2.1"), opts); !ok {
		t.Error("two significant endpoints clear the floor")
	}
}
