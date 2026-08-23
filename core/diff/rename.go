package diff

import (
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// RenameOptions tunes the NEAR-match rename pass: how much a net may change and still read as
// itself. Zero value is disabled, so a caller that passes nothing gets the exact-signature
// behaviour and nothing else.
//
// These are a value rather than ambient state (C22) because "mostly the same net" is house style
// and board style at once. A dense board with heavy test point coverage wants different numbers
// from a small module, and the engine has no way to know which it is looking at.
//
// The thresholds come in significant/all pairs. Significant counts only endpoints whose component
// is not in InsignificantClasses, which is what makes the pass insensitive to probe churn: a test
// point added or dropped is routine and should not cost a net its identity, while a device pin
// moving should.
type RenameOptions struct {
	// Enabled gates the whole pass. False reproduces the exact-signature output exactly.
	Enabled bool
	// MinOldCoverage is the fraction of the OLD net's endpoints that must survive into the
	// candidate. Lower catches heavier rewires and starts pairing nets that merely overlap.
	MinOldCoverage float64
	// MinOldCoverageSignificant is the same fraction over significant endpoints only. This is the
	// threshold doing most of the work, because it is the one probe churn cannot move.
	MinOldCoverageSignificant float64
	// MinNewCoverage is the fraction of the NEW net made up of old endpoints. It guards the
	// asymmetric case: a large net that happens to contain all of a small one is not its rename.
	MinNewCoverage float64
	// MinNewCoverageSignificant is that guard over significant endpoints.
	MinNewCoverageSignificant float64
	// MaxAddedSignificantFloor is how many significant endpoints a net may GAIN and still read as
	// itself, when half its old significant count is smaller. The floor is what lets a two-endpoint
	// net become four.
	MaxAddedSignificantFloor int
	// MinSignificantEndpoints is the floor below which no near-match is attempted. A net with one
	// significant endpoint has no shape to match on.
	MinSignificantEndpoints int
	// InsignificantClasses are the device classes excluded from the overlap arithmetic, by
	// ir.Component.DeviceClasses rather than by ref-des spelling. A house that treats mounting holes
	// or fiducials as probe-like churn adds them here.
	InsignificantClasses []string
}

// DefaultRenameOptions returns the calibrated thresholds, with Enabled false.
//
// The numbers are not invented. They are the settled values of an in-house netlist comparison tool
// that has run against real revision pairs for years, and both failure directions were observed
// while arriving at them: looser values mis-paired unrelated power rails, and tighter values missed
// obvious renames where a single decoupling capacitor had been added or removed. Treat them as a
// calibrated starting point on OTHER boards rather than a proven one on yours, and produce a
// precision number before trusting the pass on a corpus that matters.
func DefaultRenameOptions() RenameOptions {
	return RenameOptions{
		MinOldCoverage:            0.70,
		MinOldCoverageSignificant: 0.80,
		MinNewCoverage:            0.35,
		MinNewCoverageSignificant: 0.60,
		MaxAddedSignificantFloor:  2,
		MinSignificantEndpoints:   2,
		InsignificantClasses:      []string{"test_point"},
	}
}

// renameScore ranks one candidate pairing. Comparison is LEXICOGRAPHIC over the fields in
// declaration order, so a stronger significant-coverage always beats a weaker one however the later
// fields fall, and sizeDelta only ever settles a tie among otherwise indistinguishable candidates.
//
// A scalar score would have to weight these against each other, and there is no defensible exchange
// rate between "80% of the old net survived" and "the new net is two endpoints bigger". Ordering
// them says which question is asked first instead of pretending the answers are commensurable.
type renameScore struct {
	oldCovSignificant  float64
	oldCov             float64
	newCovSignificant  float64
	overlapSignificant int
	overlap            int
	sizeDelta          int // negative absolute size difference, so closer to zero ranks higher
}

// better reports whether s outranks o.
func (s renameScore) better(o renameScore) bool {
	switch {
	case s.oldCovSignificant != o.oldCovSignificant:
		return s.oldCovSignificant > o.oldCovSignificant
	case s.oldCov != o.oldCov:
		return s.oldCov > o.oldCov
	case s.newCovSignificant != o.newCovSignificant:
		return s.newCovSignificant > o.newCovSignificant
	case s.overlapSignificant != o.overlapSignificant:
		return s.overlapSignificant > o.overlapSignificant
	case s.overlap != o.overlap:
		return s.overlap > o.overlap
	default:
		return s.sizeDelta > o.sizeDelta
	}
}

// significantOf returns the endpoints of conns whose component is not insignificant.
func significantOf(conns map[string]bool, insignificant map[string]bool, comps map[string]*ir.Component) map[string]bool {
	out := make(map[string]bool, len(conns))
	for k := range conns {
		if !insignificantEndpoint(k, insignificant, comps) {
			out[k] = true
		}
	}
	return out
}

// insignificantEndpoint reports whether an endpoint's component carries an insignificant class.
//
// It reads ir.Component.DeviceClasses, the normalized set stamped once at ingestion, rather than
// matching a ref-des prefix. A board whose probes are not spelled "TP" is the case a prefix rule
// gets wrong, and it gets it wrong SILENTLY: every probe counts as a device pin, so probe churn
// starts costing nets their identity and the pass simply recovers fewer renames.
func insignificantEndpoint(endpoint string, insignificant map[string]bool, comps map[string]*ir.Component) bool {
	ref := endpoint
	for i := len(endpoint) - 1; i >= 0; i-- {
		if endpoint[i] == '.' {
			ref = endpoint[:i]
			break
		}
	}
	c, ok := comps[ref]
	if !ok {
		return false
	}
	for _, cls := range c.DeviceClasses {
		if insignificant[cls] {
			return true
		}
	}
	return false
}

// intersectionSize counts the keys present in both sets, walking the smaller one.
func intersectionSize(a, b map[string]bool) int {
	if len(b) < len(a) {
		a, b = b, a
	}
	n := 0
	for k := range a {
		if b[k] {
			n++
		}
	}
	return n
}

// nearRenames pairs leftover deleted and added nets that are MOSTLY the same net, and returns one
// NetRenamedApprox per pairing plus the names it consumed.
//
// It runs only on what the exact-signature pass could not place, and it is a separate ranked
// assignment rather than a loosened version of that pass. The distinction is the whole design. A
// wrong pairing claims a net kept its identity across a revision when it did not, and every
// downstream reading of the diff inherits that claim, so the two passes must not be able to trade
// precision for recall with each other.
//
// Cost tracks shared endpoints rather than the product of the two leftover sets, because candidates
// are generated by inverting the endpoint index instead of scoring every pair.
func nearRenames(deleted, added []string, an, bn map[string]*netInfo, aComps, bComps map[string]*ir.Component, opts RenameOptions) ([]NetChange, map[string]bool, map[string]bool) {
	usedOld, usedNew := map[string]bool{}, map[string]bool{}
	if !opts.Enabled {
		return nil, usedOld, usedNew
	}
	insignificant := make(map[string]bool, len(opts.InsignificantClasses))
	for _, c := range opts.InsignificantClasses {
		insignificant[c] = true
	}

	oldSig := make(map[string]map[string]bool, len(deleted))
	for _, n := range deleted {
		oldSig[n] = significantOf(an[n].conns, insignificant, aComps)
	}
	newSig := make(map[string]map[string]bool, len(added))
	byEndpoint := map[string][]string{}
	for _, n := range added {
		s := significantOf(bn[n].conns, insignificant, bComps)
		newSig[n] = s
		for e := range s {
			byEndpoint[e] = append(byEndpoint[e], n)
		}
	}

	type candidate struct {
		score            renameScore
		oldName, newName string
	}
	var cands []candidate
	for _, dName := range deleted {
		os := oldSig[dName]
		required := requiredOverlap(len(os), opts)
		if len(os) < required {
			continue // too small to have a shape worth matching on
		}
		overlaps := map[string]int{}
		for e := range os {
			for _, aName := range byEndpoint[e] {
				overlaps[aName]++
			}
		}
		for aName, n := range overlaps {
			if n < required {
				continue
			}
			sc, ok := scoreRename(an[dName].conns, bn[aName].conns, os, newSig[aName], opts)
			if !ok {
				continue
			}
			cands = append(cands, candidate{sc, dName, aName})
		}
	}

	// Best-first, with the names as tie-breaks so a run is reproducible. Two candidates that score
	// identically must not pair differently between runs, or the diff of two fixed revisions stops
	// being a function of those revisions.
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score.better(cands[j].score)
		}
		if cands[i].oldName != cands[j].oldName {
			return cands[i].oldName < cands[j].oldName
		}
		return cands[i].newName < cands[j].newName
	})

	var out []NetChange
	for _, c := range cands {
		if usedOld[c.oldName] || usedNew[c.newName] {
			continue
		}
		usedOld[c.oldName], usedNew[c.newName] = true, true
		oldConns, newConns := an[c.oldName].conns, bn[c.newName].conns
		out = append(out, NetChange{
			Kind:    NetRenamedApprox,
			Name:    c.newName,
			OldName: c.oldName,
			Added:   setDiff(newConns, oldConns),
			Removed: setDiff(oldConns, newConns),
			OldProv: an[c.oldName].prov,
			NewProv: bn[c.newName].prov,
			Approx: &RenameEvidence{
				OldCoverage:            c.score.oldCov,
				OldCoverageSignificant: c.score.oldCovSignificant,
				NewCoverageSignificant: c.score.newCovSignificant,
				Overlap:                c.score.overlap,
				OverlapSignificant:     c.score.overlapSignificant,
				OldEndpoints:           len(oldConns),
				NewEndpoints:           len(newConns),
				OldSignificant:         len(oldSig[c.oldName]),
				NewSignificant:         len(newSig[c.newName]),
			},
		})
	}
	return out, usedOld, usedNew
}

// requiredOverlap is the significant-endpoint overlap a candidate must clear to be SCORED at all.
//
// It is derived from MinOldCoverageSignificant rather than being its own constant, because the two
// have to agree: a prefilter stricter than the threshold drops candidates that would have passed,
// and the knob then appears not to work. Deriving it means moving the threshold moves both.
func requiredOverlap(oldSignificant int, opts RenameOptions) int {
	n := int(float64(oldSignificant)*opts.MinOldCoverageSignificant + 0.999999)
	if n < opts.MinSignificantEndpoints {
		return opts.MinSignificantEndpoints
	}
	return n
}

// scoreRename returns the ranking score for one candidate pairing, or false when the pairing fails
// any threshold. Every rejection is a threshold, so a caller cannot end up with a partial score.
func scoreRename(oldConns, newConns, oldSig, newSig map[string]bool, opts RenameOptions) (renameScore, bool) {
	if len(oldConns) == 0 || len(newConns) == 0 {
		return renameScore{}, false
	}
	overlapSig := intersectionSize(oldSig, newSig)
	if len(oldSig) < opts.MinSignificantEndpoints || overlapSig < opts.MinSignificantEndpoints {
		return renameScore{}, false
	}
	overlap := intersectionSize(oldConns, newConns)
	oldCov := float64(overlap) / float64(len(oldConns))
	oldCovSig := float64(overlapSig) / float64(len(oldSig))
	newCov := float64(overlap) / float64(len(newConns))
	newCovSig := float64(overlapSig) / float64(len(newSig))

	if oldCovSig < opts.MinOldCoverageSignificant || oldCov < opts.MinOldCoverage {
		return renameScore{}, false
	}
	if newCov < opts.MinNewCoverage || newCovSig < opts.MinNewCoverageSignificant {
		return renameScore{}, false
	}
	maxAdded := opts.MaxAddedSignificantFloor
	if h := len(oldSig) / 2; h > maxAdded {
		maxAdded = h
	}
	if len(newSig)-overlapSig > maxAdded {
		return renameScore{}, false // it grew more than a net may grow and still be itself
	}

	delta := len(newConns) - len(oldConns)
	if delta < 0 {
		delta = -delta
	}
	return renameScore{
		oldCovSignificant:  oldCovSig,
		oldCov:             oldCov,
		newCovSignificant:  newCovSig,
		overlapSignificant: overlapSig,
		overlap:            overlap,
		sizeDelta:          -delta,
	}, true
}
