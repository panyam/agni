package results

import (
	"fmt"
	"io"
	"sort"
	"strings"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// Comparing two results documents is what turns a foreign checker from an oracle a person reads
// alongside ours into a harness (WS3-104). Verifying rule semantics against kicad-cli has repeatedly
// paid — it is what caught mid-span labels, endpoint-only pin connections, and the brace escapes when
// unit tests did not — and every one of those was somebody reading two outputs side by side.
//
// The comparison is keyed on the ENTITY each tool flagged, not on rule names. Two tools have two rule
// vocabularies, and a table asserting "our track-width means their track_width" would be an unverified
// mapping that rots — the same objection that killed identifying an interface host by an MPN prefix
// list. What can be stated without asserting anything is: here is the set of entities we flagged, here
// is theirs, here is the overlap. Rule co-occurrence is then REPORTED as an observation, which lets an
// equivalence be discovered from evidence rather than declared up front.
//
// The residue is first-class. A foreign finding that named no entity we could join cannot participate
// in an entity comparison at all, and dropping it silently would make the overlap look better than it
// is. It is counted and reported as not comparable.

// Comparison is the three-way split between two runs plus what could not participate.
type Comparison struct {
	Ours, Theirs DocSummary
	// Both, OursOnly, TheirsOnly are entity keys, sorted. An entity is "flagged" by a run when any of
	// that run's findings names it.
	Both, OursOnly, TheirsOnly []string
	// CoOccurring is every (our rule, their rule) pair seen on a shared entity, most frequent first.
	// It is an observation about this pair of documents, never a claim that the two rules mean the
	// same thing.
	CoOccurring []RulePair
	// NotComparable counts findings on each side that named no entity, so a reader can tell a genuine
	// disagreement from evidence the comparison could not use.
	OursNotComparable, TheirsNotComparable int
}

// DocSummary is the identity of one side of a comparison: enough to know what produced it and whether
// its silence can be read as coverage.
type DocSummary struct {
	Producer     string
	Version      string
	Source       string
	Findings     int
	CoverageAxis bool
}

// RulePair is one observed co-occurrence: our rule and their rule fired on the same entities, on this
// pair of documents, this many times.
type RulePair struct {
	Ours, Theirs string
	Entities     int
}

// Compare computes the split. Either document may be a native run or an import; the argument order is
// only what the report calls each side.
func Compare(ours, theirs *checkspb.CheckResults) Comparison {
	c := Comparison{Ours: summarize(ours), Theirs: summarize(theirs)}
	oursBy, oursSkipped := entityRules(ours)
	theirsBy, theirsSkipped := entityRules(theirs)
	c.OursNotComparable, c.TheirsNotComparable = oursSkipped, theirsSkipped

	pairs := map[RulePair]map[string]bool{}
	for e, orules := range oursBy {
		trules, shared := theirsBy[e]
		if !shared {
			c.OursOnly = append(c.OursOnly, e)
			continue
		}
		c.Both = append(c.Both, e)
		for _, a := range orules {
			for _, b := range trules {
				k := RulePair{Ours: a, Theirs: b}
				if pairs[k] == nil {
					pairs[k] = map[string]bool{}
				}
				pairs[k][e] = true
			}
		}
	}
	for e := range theirsBy {
		if _, shared := oursBy[e]; !shared {
			c.TheirsOnly = append(c.TheirsOnly, e)
		}
	}
	sort.Strings(c.Both)
	sort.Strings(c.OursOnly)
	sort.Strings(c.TheirsOnly)
	for k, es := range pairs {
		k.Entities = len(es)
		c.CoOccurring = append(c.CoOccurring, k)
	}
	sort.Slice(c.CoOccurring, func(i, j int) bool {
		if c.CoOccurring[i].Entities != c.CoOccurring[j].Entities {
			return c.CoOccurring[i].Entities > c.CoOccurring[j].Entities
		}
		if c.CoOccurring[i].Ours != c.CoOccurring[j].Ours {
			return c.CoOccurring[i].Ours < c.CoOccurring[j].Ours
		}
		return c.CoOccurring[i].Theirs < c.CoOccurring[j].Theirs
	})
	return c
}

// entityRules indexes a document by the entity each finding names, returning the distinct rules that
// flagged each, plus how many findings named no entity at all.
func entityRules(doc *checkspb.CheckResults) (map[string][]string, int) {
	out := map[string]map[string]bool{}
	skipped := 0
	for _, f := range doc.GetFindings() {
		// The subject AND every context entity. A finding's subject is the one entity a reader has to
		// change, which is an editorial choice rather than the only entity the finding is about: a
		// clearance violation is filed under one of its two nets, and another tool reporting the same
		// violation may well file it under the other. Joining on the subject alone read that agreement
		// as a disagreement. Context carries the rest, typed and ordered, so the join can use it.
		keys := map[string]bool{}
		if k := entityKey(f.GetSubject()); k != "" {
			keys[k] = true
		}
		for _, c := range f.GetContext() {
			if k := entityKey(c.GetSubject()); k != "" {
				keys[k] = true
			}
		}
		if len(keys) == 0 {
			skipped++
			continue
		}
		for k := range keys {
			if out[k] == nil {
				out[k] = map[string]bool{}
			}
			out[k][f.GetRule()] = true
		}
	}
	flat := make(map[string][]string, len(out))
	for k, rs := range out {
		names := make([]string, 0, len(rs))
		for r := range rs {
			names = append(names, r)
		}
		sort.Strings(names)
		flat[k] = names
	}
	return flat, skipped
}

// entityKey is the join key between two runs. A PIN subject deliberately keys to its COMPONENT: one
// tool flags "R1 pin 2" where the other flags "R1", and treating those as different entities would
// report a disagreement that is only a difference in reporting granularity.
//
// It returns "" for a subject naming nothing, which is what excludes an unjoined import finding from
// the comparison rather than letting it count as a disagreement.
func entityKey(s *checkspb.Subject) string {
	switch s.GetKind() {
	case "component", "pin":
		if s.GetRef() == "" {
			return ""
		}
		return "component:" + s.GetRef()
	case "net":
		if s.GetRef() == "" {
			return ""
		}
		return "net:" + s.GetRef()
	}
	return ""
}

func summarize(doc *checkspb.CheckResults) DocSummary {
	return DocSummary{
		Producer:     doc.GetMeta().GetProducer(),
		Version:      doc.GetMeta().GetProducerVersion(),
		Source:       doc.GetDesign().GetSource(),
		Findings:     len(doc.GetFindings()),
		CoverageAxis: doc.GetMeta().GetCoverageAxis(),
	}
}

// WriteComparison renders a comparison as text. The wording is deliberate in two places: the
// co-occurrence table says it is an observation, and a side with no coverage axis is labelled, because
// "they flagged nothing here" from a flat violation list does not mean the same thing as it does from
// a run that records what it could not check.
func WriteComparison(w io.Writer, c Comparison) error {
	side := func(label string, d DocSummary) {
		axis := ""
		if !d.CoverageAxis {
			axis = "   [no coverage axis: its silence is not a pass]"
		}
		fmt.Fprintf(w, "  %-7s %s %s — %d finding(s) over %s%s\n", label+":", d.Producer, d.Version, d.Findings, d.Source, axis)
	}
	fmt.Fprintln(w, "comparing:")
	side("ours", c.Ours)
	side("theirs", c.Theirs)

	fmt.Fprintf(w, "\nentities flagged:\n  both         %d\n  ours only    %d\n  theirs only  %d\n",
		len(c.Both), len(c.OursOnly), len(c.TheirsOnly))
	list := func(label string, es []string) {
		if len(es) == 0 {
			return
		}
		fmt.Fprintf(w, "\n%s:\n", label)
		for _, e := range es {
			fmt.Fprintf(w, "  %s\n", strings.Replace(e, ":", " ", 1))
		}
	}
	list("ours only", c.OursOnly)
	list("theirs only", c.TheirsOnly)

	if len(c.CoOccurring) > 0 {
		fmt.Fprintln(w, "\nrules seen on the same entity (an observation about these two documents,")
		fmt.Fprintln(w, "not a claim that the rules ask the same question):")
		fmt.Fprintf(w, "  %-28s %-34s %s\n", "ours", "theirs", "entities")
		for _, p := range c.CoOccurring {
			fmt.Fprintf(w, "  %-28s %-34s %d\n", p.Ours, p.Theirs, p.Entities)
		}
	}
	if c.OursNotComparable > 0 || c.TheirsNotComparable > 0 {
		fmt.Fprintf(w, "\nnot comparable (a finding naming no component or net): ours %d, theirs %d\n",
			c.OursNotComparable, c.TheirsNotComparable)
	}
	return nil
}
