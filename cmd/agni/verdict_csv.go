package main

import (
	"io"

	rpt "github.com/panyam/agni/core/report"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// verdictCSVColumns is the column set of `check --verdicts --format csv`, in emitted order.
//
// It is a SEPARATE TABLE from check --format csv rather than extra rows in it. A findings table means
// one row per violation, so passing rows added there would be counted as defects by every consumer
// that counts rows.
//
// `context` carries what to HIGHLIGHT, as role=ref pairs a viewer can resolve, and `terms` carries
// the VALUES the statement rests on. A term is a bare string with no kind and is not clickable, so a
// path proof puts its hops in context and carries no terms.
//
// Datasheet citations are absent for the reason checkCSVColumns gives. A consumer that needs them
// wants json.
var verdictCSVColumns = []string{
	"verdict_id",
	// url opens this verdict's proof in a running viewer, and is EMPTY unless the operator named one
	// with --server and the design is one the server could resolve. A loose file gets a blank cell,
	// because a link built from a guessed address resolves on nobody's server (agni issue 392).
	"url",
	"rule",
	"outcome",
	// One column for the whole subject tuple, as kind:ref pairs. A relation rule puts two or three
	// entities here, and separate kind/subject/pin columns would need a column set per rule.
	"subjects",
	"statement",
	"context",
	"terms",
	"reason",
}

// writeVerdictCSV emits one row per verdict in run order (rule, then subject, as check.RunVerdicts
// produces them). It does not re-sort, so the csv and the json describe one run in one order.
func writeVerdictCSV(w io.Writer, vs []*checkspb.Verdict, meta rpt.Report) error {
	c := rpt.NewCSVWriter(w)
	c.Header(verdictCSVColumns)
	for _, v := range vs {
		var stmt, terms string
		if wit := v.GetWitness(); wit != nil {
			stmt = wit.GetStatement()
			terms = termsCell(wit.GetTerms())
		}
		c.Row([]string{
			v.GetId(),
			rpt.VerdictURL(meta, v.GetId(), v.GetRule()),
			v.GetRule(),
			outcomeCell(v.GetOutcome()),
			subjectsCell(v.GetSubjects()),
			stmt,
			contextCell(v.GetContext()),
			terms,
			v.GetReason(),
		})
	}
	return c.Finish()
}

// termsCell flattens a witness's values to one pipe-separated cell of label=value pairs, the same
// shape contextCell uses for entities.
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

// outcomeCell renders the enum as the lower-case word the Go vocabulary uses. An unrecognised value
// renders as "unspecified" and never as a blank, since a blank cell would read as "nothing to report".
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

// subjectsCell renders a verdict's tuple as pipe-separated kind:ref pairs, the context column's
// shape. The kinds stay in because a relation is commonly heterogeneous (a part and a rail), and two
// bare refs do not say which is which.
func subjectsCell(ss []*checkspb.Subject) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		parts = append(parts, s.GetKind()+":"+subjectRefCell(s))
	}
	return strings.Join(parts, "|")
}

// subjectRefCell spells one entity the way VerdictID does, so a pin reads as U12.7 and everything
// else as its ref.
func subjectRefCell(s *checkspb.Subject) string {
	if s.GetPin() != "" {
		return s.GetRef() + "." + s.GetPin()
	}
	return s.GetRef()
}

// writeVerdictJSON emits the verdicts as the wire form, so a consumer that needs the datasheet
// citations the csv omits has them without a second run.
//
// It is a bare ARRAY rather than an envelope message. CheckResults is the persisted results-document
// schema and has no verdicts field, and adding one for a print statement would change that contract
// before anything stores a considered set.
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
