package main

import (
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// verdictCSVColumns is the column set of `check --verdicts --format csv`, in emitted order.
//
// A SEPARATE TABLE from check --format csv rather than extra rows in it. That header is fixed so a
// downstream sheet or script can bind to it, and a findings table means "one row per violation": add
// passing rows and every consumer that counts rows starts counting passes as defects. The two answer
// different questions and a reader has to be able to tell which one they are holding.
//
// The two entity columns are the point of the table. `context` carries what to HIGHLIGHT, as
// role=ref pairs a viewer can resolve, and `terms` carries the VALUES the statement rests on. A term
// is a bare string with no kind, so it is not clickable and never pretends to be; that split is why
// a path proof puts its hops in context and carries no terms at all.
//
// Datasheet citations are deliberately absent, the same call check --format csv makes: a citation is
// a document, a page and a section per entry, and squashing a repeated struct into a cell produces
// something no reader can parse and no writer can round-trip. A consumer that needs them wants json.
var verdictCSVColumns = []string{
	"verdict_id",
	"rule",
	"outcome",
	// One column for the whole subject tuple, as kind:ref pairs. Most rules put one entity here; a
	// rule whose question is a relation puts two or three, and splitting them back into kind/subject/
	// pin columns would need a column set that changes per rule, which is not a table.
	"subjects",
	"statement",
	"context",
	"terms",
	"reason",
}

// writeVerdictCSV emits one row per verdict, in the order the run produced them (rule, then subject,
// matching check.RunVerdicts and therefore the findings table's axis). Like the findings writer this
// does not re-sort, so the csv and the json describe one run in one order.
func writeVerdictCSV(w io.Writer, vs []*checkspb.Verdict) error {
	c := newCSVWriter(w)
	c.header(verdictCSVColumns)
	for _, v := range vs {
		var stmt, terms string
		if wit := v.GetWitness(); wit != nil {
			stmt = wit.GetStatement()
			terms = termsCell(wit.GetTerms())
		}
		c.row([]string{
			v.GetId(),
			v.GetRule(),
			outcomeCell(v.GetOutcome()),
			subjectsCell(v.GetSubjects()),
			stmt,
			contextCell(v.GetContext()),
			terms,
			v.GetReason(),
		})
	}
	return c.finish()
}

// termsCell flattens a witness's values to one pipe-separated cell of label=value pairs, the same
// shape contextCell uses for entities so a reader learns one convention rather than two.
func termsCell(ts []*checkspb.WitnessTerm) string {
	if len(ts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, t.GetLabel()+"="+t.GetValue())
	}
	return strings.Join(parts, "|")
}

// outcomeCell renders the enum as the lower-case word the Go vocabulary uses, so the column reads as
// prose rather than as OUTCOME_NOT_CONSIDERED. An unrecognised value renders as "unspecified" and
// never as a blank, because a blank outcome cell would read as "nothing to report".
func outcomeCell(o checkspb.Outcome) string {
	switch o {
	case checkspb.Outcome_OUTCOME_PASS:
		return "pass"
	case checkspb.Outcome_OUTCOME_FAIL:
		return "fail"
	case checkspb.Outcome_OUTCOME_NO_LIMIT:
		return "no-limit"
	case checkspb.Outcome_OUTCOME_NOT_CONSIDERED:
		return "not-considered"
	case checkspb.Outcome_OUTCOME_INCONCLUSIVE:
		return "inconclusive"
	default:
		return "unspecified"
	}
}

// writeVerdictText is the human terminal form: one line per verdict, outcome first so a column of
// them scans, then the subject and the proof. A verdict with no witness prints its reason instead,
// which is the NOT_CONSIDERED case and the one a reader most needs spelled out.
func writeVerdictText(w io.Writer, vs []*checkspb.Verdict) {
	if len(vs) == 0 {
		fmt.Fprintln(w, "No rule reported a considered set. Only some rules state one; see --verdicts.")
		return
	}
	byOutcome := map[string]int{}
	width := 0
	for _, v := range vs {
		byOutcome[outcomeCell(v.GetOutcome())]++
		if n := len(subjectLabel(v)); n > width {
			width = n
		}
	}
	for _, v := range vs {
		proof := v.GetWitness().GetStatement()
		if proof == "" {
			proof = v.GetReason()
		}
		fmt.Fprintf(w, "%-14s  %-*s  %s\n", outcomeCell(v.GetOutcome()), width, subjectLabel(v), proof)
	}
	fmt.Fprintf(w, "\n%d verdicts", len(vs))
	for _, o := range []string{"pass", "fail", "inconclusive", "no-limit", "not-considered"} {
		if n := byOutcome[o]; n > 0 {
			fmt.Fprintf(w, ", %d %s", n, o)
		}
	}
	fmt.Fprintln(w)
}

// subjectLabel is the terminal spelling of a verdict's subject: a pin reads as U12.7, everything else
// as its ref. It matches the pin form VerdictID uses so a line and its id are recognisably the same
// entity.
func subjectLabel(v *checkspb.Verdict) string {
	parts := make([]string, 0, len(v.GetSubjects()))
	for _, s := range v.GetSubjects() {
		parts = append(parts, subjectRefCell(s))
	}
	return strings.Join(parts, " + ")
}

// subjectsCell renders a verdict's tuple for the csv as kind:ref pairs, pipe-separated, matching the
// context column's shape so the two read the same way. The kinds stay in: a relation is commonly
// heterogeneous (a part and a rail), and a reader of two bare refs cannot tell which is which.
func subjectsCell(ss []*checkspb.Subject) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		parts = append(parts, s.GetKind()+":"+subjectRefCell(s))
	}
	return strings.Join(parts, "|")
}

// subjectRefCell is one entity's spelling: a pin reads as U12.7, everything else as its ref. It
// matches the form VerdictID uses so a line and its id are recognisably the same entity.
func subjectRefCell(s *checkspb.Subject) string {
	if s.GetPin() != "" {
		return s.GetRef() + "." + s.GetPin()
	}
	return s.GetRef()
}

// writeVerdictJSON emits the verdicts as the wire form, so a consumer that needs the datasheet
// citations the csv omits has them without a second run.
//
// A bare ARRAY rather than an envelope message. CheckResults is the results-document schema and has
// no verdicts field; adding one so this function had something to marshal would put a field in a
// persisted contract to serve a print statement, and the document's shape is a decision to make when
// something actually stores a considered set.
func writeVerdictJSON(w io.Writer, vs []*checkspb.Verdict) error {
	mo := protojson.MarshalOptions{Multiline: true, Indent: "    "}
	if _, err := io.WriteString(w, "[\n"); err != nil {
		return err
	}
	for i, v := range vs {
		b, err := mo.Marshal(v)
		if err != nil {
			return err
		}
		sep := ",\n"
		if i == len(vs)-1 {
			sep = "\n"
		}
		if _, err := io.WriteString(w, "  "+strings.ReplaceAll(string(b), "\n", "\n  ")+sep); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]\n")
	return err
}
