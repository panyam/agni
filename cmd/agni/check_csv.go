package main

import (
	"io"
	"strconv"

	rpt "github.com/panyam/agni/core/report"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// checkCSVColumns is the column set of `check --format csv`, in emitted order.
//
// It is fixed rather than selectable. The findings shape is small enough that a good default
// removes the reason anyone would want to choose, and a stable header is what lets a downstream
// sheet or script bind to the output at all.
//
// context flattens to one pipe-separated cell of role=ref pairs. The datasheet citations are
// deliberately absent: a citation is a document, a page and a section per entry, and squashing a
// repeated struct into a cell produces something no reader can parse and no writer can round-trip.
// A consumer that needs them wants --format json.
var checkCSVColumns = []string{
	"severity",
	"inconclusive",
	"rule",
	"kind",
	"subject",
	"pin",
	"net_id",
	"message",
	"source_file",
	"native_id",
	"context",
}

// writeCheckCSV emits one row per finding, in the order the run produced them.
//
// Run order is already deterministic (the catalog is walked in a fixed order and each rule reports
// its survivors in entity order), so this writer does not re-sort. Sorting here would decouple the
// csv from every other format's ordering, and two exports of one run would still have to agree with
// the json for the round-trip test to mean anything.
func writeCheckCSV(w io.Writer, findings []*checkspb.Finding) error {
	c := rpt.NewCSVWriter(w)
	c.Header(checkCSVColumns)
	for _, f := range findings {
		subject := f.GetSubject()
		prov := f.GetProvenance()
		c.Row([]string{
			f.GetSeverity(),
			strconv.FormatBool(f.GetInconclusive()),
			f.GetRule(),
			subject.GetKind(),
			subject.GetRef(),
			subject.GetPin(),
			subject.GetNetId(),
			f.GetMessage(),
			prov.GetSourceFile(),
			prov.GetNativeId(),
			contextCell(f.GetContext()),
		})
	}
	return c.Finish()
}

// contextCell renders a finding's context entities as role=ref pairs in author order, which is
// meaningful: issue 349's ContextSubject is an ordered list, not a map, because a role may repeat
// within one finding.
func contextCell(ctx []*checkspb.ContextSubject) string {
	if len(ctx) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ctx))
	for _, cs := range ctx {
		parts = append(parts, cs.GetRole()+"="+cs.GetSubject().GetRef())
	}
	return rpt.JoinCell(parts)
}
