package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/review"
)

// This file is `agni review`'s CI gate: the predicate that turns a run's outcomes into an exit code.
//
// It is a SECOND gate rather than a reuse of `check --fail-on`, and the reason is the axis. --fail-on
// pivots on finding SEVERITY, which is a statement about consequence; this pivots on item OUTCOME,
// which is a statement about whether the question was answered at all. A checklist can go from
// answering 14 of its items to answering 13 with its failure count unchanged at zero, and no severity
// predicate can see that. Reusing one flag name for two vocabularies would make `--fail-on error` and
// `--fail-on fail` differ by a word nobody would read carefully.
//
// The gate lives at the CLI edge, not in the service. A browser has no exit code, and the numbers it
// reads are the same ones review.Tally already exposes, so there is nothing here for a served surface
// to call.

// gateExitCode is the process exit status for a tripped gate, distinct from 1 (the run itself failed).
//
// The distinction is the point. `agni check --fail-on` returns 1 for both "this board has errors" and
// "the design would not parse", which is tolerable with one gate and gets genuinely ambiguous with
// two: a pipeline that cannot tell a red board from a broken tool retries the wrong one. `check` keeps
// its single code for now rather than having its documented CI contract changed underneath it.
const gateExitCode = 2

// gateError is a gate trip. It carries an exit code so main can distinguish it from an ordinary
// failure without matching on message text.
type gateError struct{ msg string }

func (e *gateError) Error() string { return e.msg }

// exitCode maps a command error to a process exit status: 0 for success, gateExitCode for a tripped
// gate, 1 for everything else.
//
// It is a function rather than inline logic in main so it can be tested directly. Asserting an exit
// code the other way means spawning a subprocess, which is a slow test of an os.Exit call rather than
// a fast test of the decision behind it.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ge *gateError
	if errors.As(err, &ge) {
		return gateExitCode
	}
	return 1
}

// reviewGate is the configured gate for one `agni review` invocation. A zero value gates nothing,
// which is what an invocation that passed neither flag gets.
//
// Both gates are OPT-IN, matching `check --fail-on`'s empty default. `agni review` has always exited
// 0, so gating by default would turn every existing pipeline, both tutorial rungs that run it, and the
// three targets in examples/tutorial-project/Makefile red on a tool upgrade, with no flag anyone could
// have set in advance to opt out.
type reviewGate struct {
	// outcomes are the item outcomes that trip the gate, empty when --fail-on-outcome was not passed.
	outcomes map[review.Outcome]bool
	// minAnswered is the floor over Tally.Answered(), 0 when --min-answered was not passed. A floor of
	// 0 is indistinguishable from "no floor" and that is correct: every run answers at least 0 items,
	// so a zero floor could never trip.
	minAnswered int
}

// gatableOutcomes are the outcomes --fail-on-outcome accepts, and the set an unknown value is reported
// against.
//
// It is deliberately the WHOLE vocabulary rather than the two or three a team is likely to gate on.
// Each of these exists because a check that did not evaluate had been scoring as a pass, and a team
// that decides `needs-data` should block their release is making exactly the judgment the vocabulary
// was built to let them make. Restricting the set here would be this file having an opinion about
// another team's release policy.
var gatableOutcomes = []review.Outcome{
	review.Fail,
	review.Provisional,
	review.Inconclusive,
	review.NotApplicable,
	review.NotAutomated,
	review.NeedsData,
	review.NeedsDesignIntent,
	review.ComputedNA,
}

// parseReviewGate builds the gate from the two flag values. An unknown outcome is an ERROR naming the
// valid set, never a silently ignored argument: a typo in a CI config that quietly disabled the gate
// would report a clean pipeline for as long as nobody looked, which is the same silence-reads-as-
// coverage failure the outcome vocabulary exists to remove.
func parseReviewGate(failOnOutcome string, minAnswered int) (reviewGate, error) {
	g := reviewGate{minAnswered: minAnswered}
	if minAnswered < 0 {
		return reviewGate{}, fmt.Errorf("--min-answered must not be negative, got %d", minAnswered)
	}
	if strings.TrimSpace(failOnOutcome) == "" {
		return g, nil
	}
	valid := map[review.Outcome]bool{}
	for _, o := range gatableOutcomes {
		valid[o] = true
	}
	g.outcomes = map[review.Outcome]bool{}
	for _, part := range strings.Split(failOnOutcome, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		o := review.Outcome(name)
		if !valid[o] {
			return reviewGate{}, fmt.Errorf("unknown --fail-on-outcome %q (want any of: %s)", name, strings.Join(outcomeNames(), ", "))
		}
		g.outcomes[o] = true
	}
	return g, nil
}

// outcomeNames is gatableOutcomes as sorted strings, for the error message above.
func outcomeNames() []string {
	names := make([]string, len(gatableOutcomes))
	for i, o := range gatableOutcomes {
		names[i] = string(o)
	}
	sort.Strings(names)
	return names
}

// trip returns a gateError when any report violates the gate, nil otherwise.
//
// It takes every report rather than one, so the rollup gates on the same terms as the single-design
// run. A gate that only worked on one design would be a trap for the case most likely to want it: a
// team running a checklist across a family of boards in CI is exactly who reaches for a gate, and a
// silently-passing rollup is worse than no gate at all.
//
// A design trips independently of the others and the FIRST violation is reported, because a gate's job
// is to stop the pipeline rather than to be a second report. The full per-item detail was already
// rendered to stdout by the time this runs.
func (g reviewGate) trip(reports []review.Report) error {
	if len(g.outcomes) == 0 && g.minAnswered == 0 {
		return nil
	}
	for _, r := range reports {
		t := r.Tally()
		// The answered floor is checked FIRST because it is the one a severity gate cannot express, and
		// because a run that stopped answering its checklist explains a suspiciously low failure count.
		// Reporting "3 fails" on a run that answered 4 of 15 items would be the less useful of the two
		// true things.
		if g.minAnswered > 0 && t.Answered() < g.minAnswered {
			return &gateError{msg: fmt.Sprintf(
				"%s answered %d of %d checklist items, below --min-answered %d (%d covered; an item whose rule is present but whose inputs are absent reads not-applicable, which counts as covered and not as answered)",
				designLabel(r), t.Answered(), t.Total, g.minAnswered, t.Covered())}
		}
		if n, o := g.countGated(r); n > 0 {
			return &gateError{msg: fmt.Sprintf("%s has %d checklist %s at --fail-on-outcome %s",
				designLabel(r), n, pluralItems(n), o)}
		}
	}
	return nil
}

// countGated returns how many of a report's items sit at a gated outcome, and which outcome the first
// of them carried (for the message).
func (g reviewGate) countGated(r review.Report) (int, review.Outcome) {
	var n int
	var first review.Outcome
	for _, a := range r.Areas {
		for _, it := range a.Items {
			if g.outcomes[it.Outcome] {
				if n == 0 {
					first = it.Outcome
				}
				n++
			}
		}
	}
	return n, first
}

// designLabel names the design a gate message is about, falling back to the manifest when a report
// carries no design (which a hand-built Report in a test may not).
func designLabel(r review.Report) string {
	if r.Design != "" {
		return r.Design
	}
	return r.Manifest
}

func pluralItems(n int) string {
	if n == 1 {
		return "item"
	}
	return "items"
}
