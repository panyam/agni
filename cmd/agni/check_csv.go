package main

import (
	"io"
	"strconv"

	rpt "github.com/panyam/agni/core/report"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// checkCSVColumns is the column set of `check --format csv`, in emitted order. It is fixed so a
// downstream sheet or script can bind to a stable header.
//
// context flattens to one pipe-separated cell of role=ref pairs. Datasheet citations are left out,
// because each is a document, page and section and a repeated struct does not fit one cell. A
// consumer that needs them wants --format json.
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

// writeCheckCSV emits one row per finding, in the order the run produced them. Run order is
// already deterministic (catalog order, then entity order within a rule), so it does not re-sort,
// which keeps the csv in the same order as the json (TestCheckCSVMatchesJSON).
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

// contextCell renders a finding's context entities as role=ref pairs in author order. The order
// matters because ContextSubject is an ordered list in which a role may repeat (issue 349).
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
