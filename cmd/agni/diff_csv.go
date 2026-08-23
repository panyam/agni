package main

import (
	"io"
	"strconv"

	"github.com/panyam/agni/core/diff"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// diffCSVColumns is the column set of `diff --format csv`, in emitted order.
//
// A diff report is four collections of different shapes (components added, removed, changed, and
// net changes), and a csv is one table. Rather than emit four tables or make the caller ask for one
// collection at a time, every change is one row and change_class says which kind it is. A reader
// filters that column to get back any single collection, and sorting across all of them at once is
// the thing four tables could not do.
//
// The cost is sparse columns: a component row leaves the net columns empty and vice versa. That is
// the trade, and it is the right way round, because a spreadsheet handles an empty cell better than
// a person handles two files.
var diffCSVColumns = []string{
	"change_class",
	"subject",
	"old_name",
	"field",
	"old_value",
	"new_value",
	"added",
	"removed",
	"old_source_file",
	"new_source_file",
	// Populated only on net-renamed-approx rows. A near match is a judgement rather than a
	// recovered fact, and the spreadsheet is where a reviewer triages one, so the numbers that
	// decided it belong in sortable columns rather than in prose the reader has to parse.
	"match_old_coverage",
	"match_old_coverage_significant",
	"match_new_coverage_significant",
}

// Change classes emitted in the change_class column. Net classes are the diff taxonomy's own kinds
// (core/diff.NetChangeKind) under a net- prefix, so the csv names them exactly as every other
// surface does rather than inventing a second vocabulary.
const (
	classComponentAdded   = "component-added"
	classComponentRemoved = "component-removed"
	classComponentChanged = "component-changed"
	classNetPrefix        = "net-"
)

// writeDiffCSV emits one row per change, components first and then nets.
//
// Ordering is inherited, not imposed: diff.Designs already sorts all four collections (by ref des,
// by ref des then field, and nets by kind then name), so two runs over the same pair produce the
// same bytes without this writer sorting anything. Re-sorting here would create a second ordering
// opinion that could drift from the one the text and json forms use.
func writeDiffCSV(w io.Writer, rep *diff.Report) error {
	c := newCSVWriter(w)
	c.header(diffCSVColumns)

	for _, ref := range rep.ComponentsAdded {
		c.row(diffRow(classComponentAdded, ref))
	}
	for _, ref := range rep.ComponentsRemoved {
		c.row(diffRow(classComponentRemoved, ref))
	}
	for _, cc := range rep.ComponentsChanged {
		row := diffRow(classComponentChanged, cc.RefDes)
		row[3], row[4], row[5] = cc.Field, cc.Old, cc.New
		c.row(row)
	}
	for _, nc := range rep.Nets {
		row := diffRow(classNetPrefix+string(nc.Kind), nc.Name)
		row[2] = nc.OldName
		row[6], row[7] = joinCell(nc.Added), joinCell(nc.Removed)
		row[8], row[9] = provFile(nc.OldProv), provFile(nc.NewProv)
		if e := nc.Approx; e != nil {
			row[10] = strconv.FormatFloat(e.OldCoverage, 'f', 3, 64)
			row[11] = strconv.FormatFloat(e.OldCoverageSignificant, 'f', 3, 64)
			row[12] = strconv.FormatFloat(e.NewCoverageSignificant, 'f', 3, 64)
		}
		c.row(row)
	}
	return c.finish()
}

// diffRow builds a row with every column present and empty, so a caller fills the ones its change
// class populates by index and the record width always matches the header.
func diffRow(class, subject string) []string {
	row := make([]string, len(diffCSVColumns))
	row[0], row[1] = class, subject
	return row
}

// provFile returns a provenance's source file, or empty on the side where the entity does not
// exist. A new net has no old provenance and a deleted one has no new provenance, which is what
// makes the two columns worth having separately.
func provFile(p *ir.Provenance) string { return p.GetSourceFile() }
