package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	rpt "github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// This file holds the one projection of each answer into rows (agni issue 862). `check --format
// csv`, `query --format csv` and the TableService all emit these tables, so a csv and a workbook of
// one answer cannot disagree about its columns. Cells are raw; escaping belongs to the encoder.

// FindingColumns is the findings table's header, which `check --format csv` has published since
// before this projection existed, so a script binding to it keeps working. Datasheet citations are
// left out because each is a document, page and section, which does not fit one cell; json has them.
var FindingColumns = []webapi.TableColumn{
	{Name: "severity"},
	{Name: "inconclusive"},
	{Name: "rule"},
	{Name: "kind"},
	{Name: "subject", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "pin", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "net_id", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "message"},
	{Name: "source_file"},
	{Name: "native_id"},
	{Name: "context"},
}

// VerdictColumns is the verdicts table's header, the one `check --verdicts --format csv` published.
// url is filled only by a caller that can link to a viewer (the CLI with --server); the TableService
// has no viewer to link to and leaves it empty. subjects spells a relation's whole tuple as kind:ref
// pairs, since a relation commonly joins different kinds and two bare refs do not say which is which.
var VerdictColumns = []webapi.TableColumn{
	{Name: "verdict_id"},
	{Name: "url"},
	{Name: "rule"},
	{Name: "outcome"},
	{Name: "subjects", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "statement"},
	{Name: "context"},
	{Name: "terms"},
	{Name: "reason"},
}

func columns(cs []webapi.TableColumn) []*webapi.TableColumn {
	out := make([]*webapi.TableColumn, len(cs))
	for i := range cs {
		out[i] = &webapi.TableColumn{Name: cs[i].Name, Type: cs[i].Type, Kind: cs[i].Kind}
	}
	return out
}

// FindingsTable is one row per finding, in the order the run produced them.
func FindingsTable(findings []*checkspb.Finding) *webapi.Table {
	t := &webapi.Table{Name: "findings", Columns: columns(FindingColumns)}
	for _, f := range findings {
		s, prov := f.GetSubject(), f.GetProvenance()
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: []string{
			f.GetSeverity(),
			strconv.FormatBool(f.GetInconclusive()),
			f.GetRule(),
			s.GetKind(),
			s.GetRef(),
			s.GetPin(),
			s.GetNetId(),
			f.GetMessage(),
			prov.GetSourceFile(),
			prov.GetNativeId(),
			contextCell(f.GetContext()),
		}})
	}
	return t
}

// VerdictsTable is one row per verdict, passes included, in run order. link gives a verdict's viewer
// URL, and nil leaves the url column empty.
func VerdictsTable(verdicts []*checkspb.Verdict, link func(*checkspb.Verdict) string) *webapi.Table {
	t := &webapi.Table{Name: "verdicts", Columns: columns(VerdictColumns)}
	for _, v := range verdicts {
		var url string
		if link != nil {
			url = link(v)
		}
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: []string{
			v.GetId(),
			url,
			v.GetRule(),
			OutcomeWord(v.GetOutcome()),
			subjectsCell(v.GetSubjects()),
			v.GetWitness().GetStatement(),
			contextCell(v.GetContext()),
			termsCell(v.GetWitness().GetTerms()),
			v.GetReason(),
		}})
	}
	return t
}

// DiffColumns is the diff table's header, the one `diff --format csv` published. A diff is four
// collections of different shapes, and one table with change_class naming each row's kind lets a
// reader filter back to any one of them and sort across all of them at once. A row leaves the
// columns its class does not use empty. The match_ columns are filled on a near rename only, since a
// near match is a judgement and the numbers that decided it are what a reviewer triages it by.
var DiffColumns = []webapi.TableColumn{
	{Name: "change_class"},
	{Name: "subject", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "old_name", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "field"},
	{Name: "old_value"},
	{Name: "new_value"},
	{Name: "added", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "removed", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "old_source_file"},
	{Name: "new_source_file"},
	{Name: "match_old_coverage", Type: webapi.ColumnType_COLUMN_TYPE_NUMBER},
	{Name: "match_old_coverage_significant", Type: webapi.ColumnType_COLUMN_TYPE_NUMBER},
	{Name: "match_new_coverage_significant", Type: webapi.ColumnType_COLUMN_TYPE_NUMBER},
}

// DiffTable is one row per change, components first and then nets, in the order the diff reported
// them (by ref des, by ref des then field, nets by kind then name), with the unchanged nets last when
// the diff carried them. A net row's class is its kind under a net- prefix, so the table names kinds
// exactly as the diff does.
func DiffTable(resp *webapi.DiffDesignsResponse) *webapi.Table {
	t := &webapi.Table{Name: "diff", Columns: columns(DiffColumns)}
	row := func(class, subject string) []string {
		r := make([]string, len(DiffColumns))
		r[0], r[1] = class, subject
		return r
	}
	rep := resp.GetReport()
	for _, ref := range rep.GetComponentsAdded() {
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: row("component-added", ref)})
	}
	for _, ref := range rep.GetComponentsRemoved() {
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: row("component-removed", ref)})
	}
	for _, cc := range rep.GetComponentsChanged() {
		r := row("component-changed", cc.GetRefDes())
		r[3], r[4], r[5] = cc.GetField(), cc.GetOld(), cc.GetNew()
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: r})
	}
	for _, nc := range rep.GetNets() {
		r := row("net-"+nc.GetKind(), nc.GetName())
		r[2] = nc.GetOldName()
		r[6], r[7] = strings.Join(nc.GetAdded(), rpt.MultiValueSep), strings.Join(nc.GetRemoved(), rpt.MultiValueSep)
		r[8], r[9] = nc.GetOldProv().GetSourceFile(), nc.GetNewProv().GetSourceFile()
		if e := nc.GetApprox(); e != nil {
			r[10] = strconv.FormatFloat(e.GetOldCoverage(), 'f', 3, 64)
			r[11] = strconv.FormatFloat(e.GetOldCoverageSignificant(), 'f', 3, 64)
			r[12] = strconv.FormatFloat(e.GetNewCoverageSignificant(), 'f', 3, 64)
		}
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: r})
	}
	return t
}

// ReviewColumns is the review table's header. findings names each firing behind an item as
// rule=kind:ref, and unmet each part whose datasheet the item needed, so a failed or undecided row
// says what to look at.
var ReviewColumns = []webapi.TableColumn{
	{Name: "area"},
	{Name: "id", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "title"},
	{Name: "outcome"},
	{Name: "note"},
	{Name: "findings"},
	{Name: "unmet"},
}

// ReviewTables is a review's items, one row each in the checklist's own order, since a reviewer reads
// a checklist in the order it was written, and its one-row summary.
func ReviewTables(rv *webapi.Review) []*webapi.Table {
	items := &webapi.Table{Name: "review", Columns: columns(ReviewColumns)}
	for _, area := range rv.GetResults().GetAreas() {
		for _, it := range area.GetItems() {
			findings := make([]string, 0, len(it.GetFindings()))
			for _, f := range it.GetFindings() {
				findings = append(findings, f.GetRule()+"="+subjectsCell([]*checkspb.Subject{f.GetSubject()}))
			}
			unmet := make([]string, 0, len(it.GetUnmet()))
			for _, u := range it.GetUnmet() {
				part := strings.TrimSpace(u.GetManufacturer() + " " + u.GetMpn())
				if u.GetSpecAbsent() {
					part += " (no spec)"
				}
				unmet = append(unmet, part)
			}
			items.Rows = append(items.Rows, &webapi.TableRow{Cells: []string{
				area.GetName(), it.GetId(), it.GetTitle(), it.GetOutcome(), it.GetNote(),
				strings.Join(findings, rpt.MultiValueSep), strings.Join(unmet, rpt.MultiValueSep),
			}})
		}
	}
	s := rv.GetSummary()
	summary := &webapi.Table{Name: "review_summary"}
	var cells []string
	for _, kv := range []struct {
		name string
		n    int32
	}{{"total", s.GetTotal()}, {"covered", s.GetCovered()}, {"answered", s.GetAnswered()}, {"pass", s.GetPass()}, {"fail", s.GetFail()}, {"provisional", s.GetProvisional()}} {
		summary.Columns = append(summary.Columns, &webapi.TableColumn{Name: kv.name, Type: webapi.ColumnType_COLUMN_TYPE_NUMBER})
		cells = append(cells, strconv.Itoa(int(kv.n)))
	}
	summary.Rows = []*webapi.TableRow{{Cells: cells}}
	return []*webapi.Table{items, summary}
}

// SkippedTable is the selected rules that could not run on this design and why, so a workbook of a
// run says which rules it is silent on. A revision read with no board lists its board-tier rules here
// (agni issue 848). The reason is check.Available's own words.
func SkippedTable(skipped []*webapi.SkippedRule) *webapi.Table {
	t := &webapi.Table{Name: "skipped", Columns: []*webapi.TableColumn{{Name: "rule"}, {Name: "reason"}}}
	for _, s := range skipped {
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: []string{s.GetName(), s.GetReason()}})
	}
	return t
}

// verdictOutcomes are the outcome columns of the per-rule count, in the order a reader triages them.
var verdictOutcomes = []checkspb.Outcome{
	checkspb.Outcome_OUTCOME_PASS, checkspb.Outcome_OUTCOME_FAIL, checkspb.Outcome_OUTCOME_INCONCLUSIVE,
	checkspb.Outcome_OUTCOME_NO_LIMIT, checkspb.Outcome_OUTCOME_NOT_CONSIDERED,
}

// VerdictCountsTable is one row per rule, in the order rules first appear among the verdicts, counting
// each outcome and the total, so a reader sees which rules decided most and which declined.
func VerdictCountsTable(verdicts []*checkspb.Verdict) *webapi.Table {
	t := &webapi.Table{Name: "verdicts_by_rule", Columns: []*webapi.TableColumn{{Name: "rule"}}}
	for _, o := range verdictOutcomes {
		t.Columns = append(t.Columns, &webapi.TableColumn{Name: OutcomeWord(o), Type: webapi.ColumnType_COLUMN_TYPE_NUMBER})
	}
	t.Columns = append(t.Columns, &webapi.TableColumn{Name: "total", Type: webapi.ColumnType_COLUMN_TYPE_NUMBER})
	var order []string
	counts := map[string]map[checkspb.Outcome]int{}
	for _, v := range verdicts {
		if counts[v.GetRule()] == nil {
			counts[v.GetRule()] = map[checkspb.Outcome]int{}
			order = append(order, v.GetRule())
		}
		counts[v.GetRule()][v.GetOutcome()]++
	}
	for _, rule := range order {
		cells, total := []string{rule}, 0
		for _, o := range verdictOutcomes {
			cells = append(cells, strconv.Itoa(counts[rule][o]))
			total += counts[rule][o]
		}
		// total counts every verdict, an unspecified outcome included, so the row adds up to the rule's
		// verdicts even when a column does not.
		for o, n := range counts[rule] {
			if o == checkspb.Outcome_OUTCOME_UNSPECIFIED {
				total += n
			}
		}
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: append(cells, strconv.Itoa(total))})
	}
	return t
}

// QueryTable is a query answer's columns, then its provenance as the last column, the shape `query
// --format csv` prints. A column the answer types as an entity kind orders as a name.
func QueryTable(name string, resp *webapi.RunQueryResponse) *webapi.Table {
	t := &webapi.Table{Name: name}
	kinds := resp.GetColumnKinds()
	for i, c := range resp.GetColumns() {
		col := &webapi.TableColumn{Name: c}
		if i < len(kinds) && kinds[i] != "" {
			col.Type, col.Kind = webapi.ColumnType_COLUMN_TYPE_NAME, kinds[i]
		}
		t.Columns = append(t.Columns, col)
	}
	t.Columns = append(t.Columns, &webapi.TableColumn{Name: rpt.ProvenanceColumn})
	for _, r := range resp.GetRows() {
		cells := append(append(make([]string, 0, len(r.GetCells())+1), r.GetCells()...), strings.Join(RowCites(resp, r), " ; "))
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: cells})
	}
	return t
}

// RowCites is a row's citations, read through the response's sources table when the row carries
// indices into it (agni issue 916) and from the row's own list when it does not, so an answer from
// either side of that change reads the same.
func RowCites(resp *webapi.RunQueryResponse, r *webapi.QueryRow) []string {
	idx := r.GetCiteIndex()
	if len(idx) == 0 {
		return r.GetCites()
	}
	src := resp.GetSources()
	out := make([]string, 0, len(idx))
	for _, i := range idx {
		if int(i) < len(src) {
			out = append(out, src[i])
		}
	}
	return out
}

// IndexSources fills a response's sources table and each row's cite_index from the rows' own cites,
// for an answer built outside answer (agni issue 916).
func IndexSources(resp *webapi.RunQueryResponse) {
	at := map[string]int32{}
	for _, r := range resp.GetRows() {
		r.CiteIndex = r.CiteIndex[:0]
		for _, c := range r.GetCites() {
			n, ok := at[c]
			if !ok {
				resp.Sources = append(resp.Sources, c)
				n = int32(len(resp.Sources) - 1)
				at[c] = n
			}
			r.CiteIndex = append(r.CiteIndex, n)
		}
	}
}

// OutcomeWord is a verdict outcome as the lower-case word every surface prints. An unrecognised value
// is "unspecified" and never blank, since a blank would read as nothing to report.
func OutcomeWord(o checkspb.Outcome) string {
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

// contextCell is a finding's or verdict's context entities as role=ref pairs in author order, since a
// role may repeat and the order is the author's (issue 349).
func contextCell(ctx []*checkspb.ContextSubject) string {
	parts := make([]string, 0, len(ctx))
	for _, cs := range ctx {
		parts = append(parts, cs.GetRole()+"="+cs.GetSubject().GetRef())
	}
	return strings.Join(parts, rpt.MultiValueSep)
}

// termsCell is a witness's values as label=value pairs, the shape contextCell uses for entities.
func termsCell(ts []*checkspb.WitnessTerm) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, t.GetLabel()+"="+t.GetValue())
	}
	return strings.Join(parts, "|")
}

func subjectsCell(ss []*checkspb.Subject) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		ref := s.GetRef()
		if s.GetPin() != "" {
			ref += "." + s.GetPin()
		}
		parts = append(parts, s.GetKind()+":"+ref)
	}
	return strings.Join(parts, "|")
}

// TableService answers Tabulate. It holds nothing, so the zero value serves.
type TableService struct{}

// Tabulate projects the answer the request carries and orders each table as asked. An empty answer,
// a failed query in a set, or an order_by naming a column no table has is ErrInvalidArgument.
func (TableService) Tabulate(_ context.Context, req *webapi.TabulateRequest) (*webapi.TabulateResponse, error) {
	var tables []*webapi.Table
	switch a := req.GetAnswer().(type) {
	case *webapi.TabulateRequest_Check:
		tables = append(tables, FindingsTable(a.Check.GetFindings()))
		if vs := a.Check.GetVerdicts(); len(vs) > 0 {
			tables = append(tables, VerdictsTable(vs, nil), VerdictCountsTable(vs))
		}
		tables = append(tables, SkippedTable(a.Check.GetSkipped()))
	case *webapi.TabulateRequest_Diff:
		tables = append(tables, DiffTable(a.Diff))
	case *webapi.TabulateRequest_Review:
		tables = append(tables, ReviewTables(a.Review)...)
	case *webapi.TabulateRequest_Query:
		tables = append(tables, QueryTable("query", a.Query))
	case *webapi.TabulateRequest_QuerySet:
		var failed []string
		for _, r := range a.QuerySet.GetResults() {
			if r.GetError() != "" {
				failed = append(failed, r.GetName()+": "+r.GetError())
				continue
			}
			tables = append(tables, QueryTable(r.GetName(), r.GetResult()))
		}
		if len(failed) > 0 {
			return nil, fmt.Errorf("%w: the set has unanswered queries, so their tables would be missing: %s", ErrInvalidArgument, strings.Join(failed, "; "))
		}
	default:
		return nil, fmt.Errorf("%w: Tabulate needs an answer to project", ErrInvalidArgument)
	}
	if err := OrderTables(tables, req.GetOrderBy(), req.GetColumnTypes()); err != nil {
		return nil, err
	}
	return &webapi.TabulateResponse{Tables: tables}, nil
}

// OrderTables sorts each table by orderBy, as TabulateRequest.order_by describes, with types
// overriding a column's own type by name. It sorts in place and is stable.
func OrderTables(tables []*webapi.Table, orderBy []string, types map[string]webapi.ColumnType) error {
	if len(orderBy) == 0 {
		return nil
	}
	type key struct {
		name string
		desc bool
	}
	keys := make([]key, 0, len(orderBy))
	for _, o := range orderBy {
		k := key{name: strings.TrimPrefix(o, "-"), desc: strings.HasPrefix(o, "-")}
		if k.name == "" {
			return fmt.Errorf("%w: order_by entry %q names no column", ErrInvalidArgument, o)
		}
		keys = append(keys, k)
	}
	found := map[string]bool{}
	for _, t := range tables {
		type col struct {
			i    int
			desc bool
			typ  webapi.ColumnType
		}
		var cols []col
		for _, k := range keys {
			for i, c := range t.GetColumns() {
				if c.GetName() != k.name {
					continue
				}
				typ := c.GetType()
				if over, ok := types[k.name]; ok {
					typ = over
				}
				cols = append(cols, col{i, k.desc, typ})
				found[k.name] = true
				break
			}
		}
		if len(cols) == 0 {
			continue
		}
		sort.SliceStable(t.Rows, func(a, b int) bool {
			ra, rb := t.Rows[a].GetCells(), t.Rows[b].GetCells()
			for _, c := range cols {
				x, y := cellAt(ra, c.i), cellAt(rb, c.i)
				// Empty cells sort last whichever way the column runs.
				if (x == "") != (y == "") {
					return y == ""
				}
				cmp := compareCells(x, y, c.typ)
				if c.desc {
					cmp = -cmp
				}
				if cmp != 0 {
					return cmp < 0
				}
			}
			return false
		})
	}
	var missing []string
	for _, k := range keys {
		if !found[k.name] {
			missing = append(missing, k.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: order_by names %s, which no table has", ErrInvalidArgument, strings.Join(missing, ", "))
	}
	return nil
}

func cellAt(cells []string, i int) string {
	if i < len(cells) {
		return cells[i]
	}
	return ""
}

func compareCells(x, y string, typ webapi.ColumnType) int {
	switch typ {
	case webapi.ColumnType_COLUMN_TYPE_NAME:
		return compareNatural(x, y)
	case webapi.ColumnType_COLUMN_TYPE_NUMBER:
		a, errA := strconv.ParseFloat(x, 64)
		b, errB := strconv.ParseFloat(y, 64)
		switch {
		case errA == nil && errB == nil:
			switch {
			case a < b:
				return -1
			case a > b:
				return 1
			}
			return 0
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		}
	}
	return strings.Compare(x, y)
}

// compareNatural compares two names with their digit runs read as numbers, so R2 < R10 and
// U10.3 < U10.12. Text runs compare case-insensitively, and the plain strings break a tie, so R01
// and R1 still order the same way every time.
func compareNatural(x, y string) int {
	a, b := x, y
	for a != "" && b != "" {
		da, db := isDigit(a[0]), isDigit(b[0])
		switch {
		case da && db:
			ra, rb := digitRun(a), digitRun(b)
			na, nb := strings.TrimLeft(ra, "0"), strings.TrimLeft(rb, "0")
			if len(na) != len(nb) {
				return cmpInt(len(na), len(nb))
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			a, b = a[len(ra):], b[len(rb):]
		case da != db:
			// A digit sorts before a letter, as it does in plain text.
			if da {
				return -1
			}
			return 1
		default:
			ta, tb := textRun(a), textRun(b)
			if c := strings.Compare(strings.ToLower(ta), strings.ToLower(tb)); c != 0 {
				return c
			}
			a, b = a[len(ta):], b[len(tb):]
		}
	}
	if a != b {
		return cmpInt(len(a), len(b))
	}
	return strings.Compare(x, y)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digitRun(s string) string {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i]
}

func textRun(s string) string {
	i := 0
	for i < len(s) && !isDigit(s[i]) {
		i++
	}
	return s[:i]
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
