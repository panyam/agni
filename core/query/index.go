package query

import (
	"strconv"
	"sync"
)

// Fact lookup by binding pattern (WS3-125/126).
//
// The evaluator used to satisfy every atom by scanning its whole relation, so a two-atom
// conjunction was an unindexed nested loop: quadratic, and the shape of nearly every rule. These
// indexes turn the bound positions of an atom into a bucket lookup.
//
// # The index NARROWS candidates, it does not decide matches
//
// That split is load-bearing, not caution. valueEq compares numerically when BOTH values carry a
// number and by string otherwise, which makes it non-transitive: {S:"10.0",Num:10} equals
// {S:"10",Num:10} numerically, which equals {S:"10",Num:nil} by string, while the first and last
// are unequal. No hash can reproduce that, because hashing forces an equivalence relation onto a
// comparison that is not one.
//
// So a bucket may only ever be a SUPERSET of the matches: unify (EDB) and valsEqual (IDB) still
// decide, exactly as before. False positives cost a comparison. A false NEGATIVE would silently
// drop a row, which reads as a clean result, so the keying below is deliberately generous.

// patternMask marks which positional arguments of an atom are bound at solve time — a constant, or
// a variable the binding already carries. Wildcards and unbound variables are free. Relations here
// are arity 2 or 3 (FactRow has six slots and each relation projects a couple), so one byte is
// ample and the number of DISTINCT masks a query actually probes stays in single digits.
type patternMask uint8

// idxKey identifies one index: a relation and the binding pattern it is keyed on. A relation
// probed free-bound and bound-free gets two indexes, which is correct — they bucket on different
// positions — and cheap at this arity.
type idxKey struct {
	rel  string
	mask patternMask
}

// valueKeys returns every bucket key one value may be filed or found under: its string, plus the
// canonical string of its number when it carries one.
//
// Both sides use this, and that symmetry is the correctness argument. Any two values valueEq
// considers equal share at least one key:
//
//	both numeric, same number  -> both carry ftoa(num)
//	otherwise, equal strings   -> both carry S
//
// which are exactly valueEq's two branches. So a match can never fall in a bucket the probe does
// not look in. The reverse is not guaranteed and does not need to be: extra candidates are rejected
// by the exact comparison that still runs.
//
// A fact's numeric field is already canonical (fieldValue sets S: ftoa(*f.Num)), so this usually
// returns one key. It is a query constant like `10.0`, or a rule head deriving one, that produces
// two.
func valueKeys(v Value) []string {
	// An absent field must not share a bucket with a legitimately empty string, or a probe for one
	// would find the other and the exact comparison would then have to reject it. The sentinel is a
	// byte no fact value can contain, so it cannot collide with real content.
	if v.Absent {
		return []string{absentKey}
	}
	if v.Num == nil {
		return []string{v.S}
	}
	if canon := ftoa(*v.Num); canon != v.S {
		return []string{v.S, canon}
	}
	return []string{v.S}
}

// absentKey is the index bucket an ABSENT value files under. A NUL byte cannot appear in a fact
// value read from any supported source, so it is unreachable as real content, which is what makes it
// safe as a sentinel rather than merely unlikely.
const absentKey = "\x00absent"

// tupleKeys expands values into every bucket key the tuple may be filed or found under: the cross
// product of each value's keys. One key in the common case, and bounded by 2^k for k values that
// carry a non-canonical number, with k at most the relation's arity.
//
// Each segment is length-prefixed so the encoding is injective: without it ("a|b","c") and
// ("a","b|c") would collide. Collisions would only cost comparisons rather than correctness, but
// they are free to avoid.
func tupleKeys(vals []Value) []string {
	keys := []string{""}
	for _, v := range vals {
		vks := valueKeys(v)
		next := make([]string, 0, len(keys)*len(vks))
		for _, prefix := range keys {
			for _, s := range vks {
				next = append(next, prefix+strconv.Itoa(len(s))+":"+s)
			}
		}
		keys = next
	}
	return keys
}

// boundArgs reads the atom's bound positions under the current binding, returning the mask and the
// values in positional order. ok is false when nothing is bound, which is the driver atom of a body
// and stays a full scan: there is nothing to look up by.
func boundArgs(args []Term, bnd *binding) (mask patternMask, vals []Value, ok bool) {
	for i, a := range args {
		v, bound := resolve(a, bnd)
		if !bound {
			continue
		}
		// A wildcard resolves to nothing; resolve already reports it unbound, so reaching here
		// means a constant or a genuinely bound variable.
		mask |= 1 << uint(i)
		vals = append(vals, v)
	}
	return mask, vals, mask != 0
}

// edbIndex buckets a relation's facts by the values at one binding pattern's positions.
//
// Buckets hold POSITIONS into the relation's slice, not copies. A fact is filed under each of its
// keys, so a probe that looks in several buckets can meet the same fact twice, and a duplicated
// candidate is not harmless: unify would yield it twice, which inflates a count() aggregate rather
// than merely repeating a row. Positions make that duplicate removable exactly.
type edbIndex map[string][]int

// edbIndexCache holds the per-(relation, pattern) indexes for one Base's immutable fact set.
//
// Behind a pointer on Base so Eval's shallow copy shares one cache instead of copying a lock, and
// so a second query over the same design reuses the indexes the first one built.
type edbIndexCache struct {
	mu  sync.RWMutex
	idx map[idxKey]edbIndex
}

// countWork records one candidate comparison. See Base.work.
func (b *Base) countWork() {
	if b.work != nil {
		*b.work++
	}
}

// newEDBIndexCache builds an empty cache. Never nil on a Base, so no probe path has to check.
func newEDBIndexCache() *edbIndexCache { return &edbIndexCache{idx: map[idxKey]edbIndex{}} }

// get returns the index for a relation at a pattern, building it once on first use. Lazy because a
// rule catalog probes a handful of the possible patterns, and eagerly indexing every relation at
// every mask would cost more than the scans it saves on a design nobody queries deeply.
func (c *edbIndexCache) get(rel string, facts []FactRow, fields []edbField, mask patternMask) edbIndex {
	k := idxKey{rel: rel, mask: mask}
	c.mu.RLock()
	idx, ok := c.idx[k]
	c.mu.RUnlock()
	if ok {
		return idx
	}
	built := buildEDBIndex(facts, fields, mask)
	c.mu.Lock()
	defer c.mu.Unlock()
	// Another goroutine may have built it while this one was working. Keep the existing entry so
	// every caller shares one map rather than racing to replace it.
	if idx, ok := c.idx[k]; ok {
		return idx
	}
	c.idx[k] = built
	return built
}

// buildEDBIndex indexes every fact of rel at the given mask. Facts are immutable for the life of a
// Base, so this is built once per (relation, pattern) and reused across queries.
func buildEDBIndex(facts []FactRow, fields []edbField, mask patternMask) edbIndex {
	idx := edbIndex{}
	vals := make([]Value, 0, len(fields))
	for pos, f := range facts {
		vals = vals[:0]
		for i, fld := range fields {
			if mask&(1<<uint(i)) != 0 {
				vals = append(vals, fieldValue(f, fld))
			}
		}
		for _, k := range tupleKeys(vals) {
			idx[k] = append(idx[k], pos)
		}
	}
	return idx
}

// indexMinFacts is the relation size below which scanning beats indexing. Building a map over a
// handful of facts costs more than the loop it replaces, and small relations are common (a design
// has few nets carrying a given class). The threshold is deliberately low: the quadratic behaviour
// this fixes needs both a large relation AND repeated probes, and neither happens under it.
const indexMinFacts = 16

// edbCandidates narrows an atom's facts to the positions that can match under the current binding.
// all is true when the caller should scan the whole relation instead: nothing is bound (the driver
// atom of a body, which has nothing to look up by) or the relation is too small to index.
func (b *Base) edbCandidates(atom *Atom, fields []edbField, bnd *binding) (pos []int, all bool) {
	facts := b.edb[atom.Relation]
	// No cache means no indexing: a Base built by struct literal rather than NewBase, and the
	// equivalence oracle in the tests, which needs the pre-index scan path to compare against.
	if b.edbIdx == nil || len(facts) < indexMinFacts {
		return nil, true
	}
	mask, vals, ok := boundArgs(atom.Args, bnd)
	if !ok {
		return nil, true
	}
	idx := b.edbIdx.get(atom.Relation, facts, fields, mask)
	keys := tupleKeys(vals)
	if len(keys) == 1 {
		return idx[keys[0]], false // the common path: the bucket IS the candidate list, no copy
	}
	// Several keys only arise for a non-canonical numeric value. A fact is filed under each of its
	// own keys, so the buckets can overlap and the union has to be deduplicated by position.
	seen := map[int]bool{}
	for _, k := range keys {
		for _, p := range idx[k] {
			if !seen[p] {
				seen[p] = true
				pos = append(pos, p)
			}
		}
	}
	return pos, false
}

// idbIndex is the derived-relation analogue, and it must be INCREMENTAL: an IDB relation grows
// during the fixpoint, so an index built at one point goes stale as the next round derives more.
// Rather than rebuilding (which would make the fixpoint quadratic again, in a new place), it tracks
// how many tuples it has seen and indexes only the tail on the next probe.
//
// Positions are stored rather than tuples so the index stays valid as the backing slice grows.
type idbIndex struct {
	buckets map[string][]int
	n       int // tuples of the relation already indexed
}

// sync indexes any tuples appended since the last call.
func (x *idbIndex) sync(tuples []idbTuple, mask patternMask) {
	if x.buckets == nil {
		x.buckets = map[string][]int{}
	}
	vals := make([]Value, 0, 4)
	for ; x.n < len(tuples); x.n++ {
		vals = vals[:0]
		for i, v := range tuples[x.n].vals {
			if mask&(1<<uint(i)) != 0 {
				vals = append(vals, v)
			}
		}
		for _, k := range tupleKeys(vals) {
			x.buckets[k] = append(x.buckets[k], x.n)
		}
	}
}

// fullMask is every position of an arity-n tuple, the pattern the dedup set probes on: "has this
// exact tuple been derived". It is the same machinery as a join lookup, which is why WS3-125 and
// WS3-126 are one structure rather than two.
func fullMask(arity int) patternMask {
	return patternMask(1)<<uint(arity) - 1
}

// idbCandidates narrows a derived relation to the tuples that can match under the current binding,
// or returns all of them when nothing is bound or the relation is small.
//
// Unlike the EDB, the relation may have grown since the index was last touched, so the index syncs
// its tail first. Tuples are returned by value to match the previous iteration, and the caller's
// own binding check still decides.
func (b *Base) idbCandidates(atom *Atom, bnd *binding) []idbTuple {
	tuples := b.idb[atom.Relation]
	if len(tuples) < indexMinFacts {
		return tuples
	}
	mask, vals, ok := boundArgs(atom.Args, bnd)
	if !ok {
		return tuples
	}
	x := b.idbIndexFor(atom.Relation, mask)
	x.sync(tuples, mask)
	keys := tupleKeys(vals)
	if len(keys) == 1 {
		return gatherTuples(tuples, x.buckets[keys[0]], nil)
	}
	// Overlapping buckets, so deduplicate by position. Only reachable for a non-canonical numeric
	// value; see valueKeys.
	seen := make(map[int]bool, len(keys))
	var out []idbTuple
	for _, k := range keys {
		out = append(out, gatherTuples(tuples, x.buckets[k], seen)...)
	}
	return out
}

// gatherTuples materializes the tuples at the given positions, skipping any already taken when seen
// is non-nil.
func gatherTuples(tuples []idbTuple, pos []int, seen map[int]bool) []idbTuple {
	out := make([]idbTuple, 0, len(pos))
	for _, p := range pos {
		if seen != nil {
			if seen[p] {
				continue
			}
			seen[p] = true
		}
		out = append(out, tuples[p])
	}
	return out
}
