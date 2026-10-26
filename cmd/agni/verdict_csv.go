package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
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
var verdictCSVColumns = columnNames(service.VerdictColumns)

// writeVerdictCSV emits the verdicts table in run order (rule, then subject, as check.RunVerdicts
// produces them), so the csv and the json describe one run in one order unless --order-by asks
// otherwise. A row's url links to the viewer when meta names a server, and is empty otherwise.
func writeVerdictCSV(w io.Writer, vs []*checkspb.Verdict, meta rpt.Report, orderBy ...string) error {
	link := func(v *checkspb.Verdict) string {
		return rpt.VerdictURL(meta, v.GetId(), v.GetRule(), check.SubjectParams(subjectEntities(v.GetSubjects())))
	}
	return writeTableCSV(w, service.VerdictsTable(vs, link), orderBy)
}

// checkTable is one of the tables Tabulate projects a check run into, by name, so `check --table
// pin_map` writes the rows the TableService serves for that run (C35). An unknown name lists the
// tables the run has, since which exist depends on the run (pin_map needs an IO map).
func checkTable(resp *webapi.CheckDesignResponse, name string) (*webapi.Table, error) {
	out, err := service.TableService{}.Tabulate(context.Background(), &webapi.TabulateRequest{
		Answer: &webapi.TabulateRequest_Check{Check: resp},
	})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, t := range out.GetTables() {
		if t.GetName() == name {
			return t, nil
		}
		names = append(names, t.GetName())
	}
	return nil, fmt.Errorf("this run has no %q table; it has %s", name, strings.Join(names, ", "))
}
