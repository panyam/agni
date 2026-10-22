package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/query"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/frontends/mcp"
)

// answering is one call to a jaala tool over a design. It keeps the read the program was evaluated
// over, so the answer is presented from the same facts, and presents each page once for both its
// text and its structured content.
type answering struct {
	s      *srv
	design artifact.URI
	snap   *webapi.Snapshot

	// ctx is the call's, kept from baseFor for presenting, since jaala hands Format and Data the
	// answer alone.
	ctx  context.Context
	read *service.QueryRead
	q    query.Query

	shown *shown
	err   error
}

// shown is one page of an answer as agni presents it.
type shown struct {
	Design   string                   `json:"design"`
	Query    string                   `json:"query"`
	Bindings map[string]string        `json:"bindings,omitempty"`
	Link     string                   `json:"link,omitempty"`
	NoLink   string                   `json:"no_link,omitempty"`
	Answer   any                      `json:"answer"`
	Tables   any                      `json:"tables"`
	tables   []*webapi.Table          // the projected tables, for the text
	resp     *webapi.RunQueryResponse // the answer, for the text's counts
	page     mcp.Answer               // where the page sits in the whole answer
}

// baseFor reads the design for q, with the board only when q reads it, and keeps the read.
func (a *answering) baseFor(ctx context.Context, q datalog.Query) (*datalog.Base, error) {
	err := a.s.designs.ask(ctx, a.design, func(snap *webapi.Snapshot) error {
		r, err := a.s.api.Query.ReadQuery(a.s.withBudget(ctx), snap.GetName(), q)
		if err != nil {
			return err
		}
		a.ctx, a.snap, a.read, a.q = ctx, snap, r, q
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a.read.Base(), nil
}

// present is the page as agni answers it: RunQuery's projection of its rows, TableService's tables of
// them, and a link that asks the question again in the viewer.
func (a *answering) present(page mcp.Answer) (*shown, error) {
	if a.shown != nil || a.err != nil {
		return a.shown, a.err
	}
	a.shown, a.err = a.presentOnce(page)
	return a.shown, a.err
}

func (a *answering) presentOnce(page mcp.Answer) (*shown, error) {
	if a.read == nil {
		return nil, fmt.Errorf("the answer was not read from %s", a.design)
	}
	bind := map[string]query.Value{}
	for name, vals := range page.Bind {
		if len(vals) != 1 {
			return nil, fmt.Errorf("bind: ?%s has %d values, and agni binds one value per variable; ask once per value", name, len(vals))
		}
		bind[name] = vals[0]
	}
	resp, err := a.read.Present(a.ctx, a.q, page.Program, page.Table, page.Offset, page.Offset+page.Len, bind)
	if err != nil {
		return nil, err
	}
	tabs, err := service.TableService{}.Tabulate(a.ctx, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Query{Query: resp}})
	if err != nil {
		return nil, err
	}
	sh := &shown{Design: a.design.String(), Query: page.Program, Answer: protoJSON(resp), Tables: tablesJSON(tabs.GetTables()), tables: tabs.GetTables(), resp: resp, page: page}
	if len(bind) > 0 {
		sh.Bindings = map[string]string{}
		for name, v := range bind {
			sh.Bindings[name] = query.Display(v)
		}
	}
	meta, why := a.s.linkMeta(a.snap)
	sh.NoLink = why
	sh.Link = rpt.QueryURL(meta, page.Program, "", query.BindArgs(bind))
	return sh, nil
}

// format is the page as text an agent reads: the question, the rows with what they rest on, how much
// of the answer this is, and the link.
func (a *answering) format(page mcp.Answer) string {
	sh, err := a.present(page)
	if err != nil {
		return "agni could not present this answer: " + err.Error() + "\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "design: %s\nquery: %s\n", sh.Design, strings.Join(strings.Fields(sh.Query), " "))
	for _, name := range sortedKeys(sh.Bindings) {
		fmt.Fprintf(&b, "bind: ?%s = %s\n", name, sh.Bindings[name])
	}
	b.WriteString("\n")
	for _, t := range sh.tables {
		writeTable(&b, t)
	}
	total := page.Table.Len()
	switch {
	case total == 0:
		b.WriteString("no rows\n" + page.Why)
	case page.Next != "":
		fmt.Fprintf(&b, "rows %d to %d of %d; continue with page_token %q\n", page.Offset+1, page.Offset+page.Len, total, page.Next)
	default:
		fmt.Fprintf(&b, "%s\n", rowCount(total))
	}
	writeLink(&b, sh.Link, sh.NoLink)
	return b.String()
}

// data is the page's structured content.
func (a *answering) data(page mcp.Answer) (any, error) {
	return a.present(page)
}

// linkMeta is where the viewer shows snap's design, and why there is no link when it can't: the
// design's declared entry within its mount (C32), on the server this one serves beside.
func (s *srv) linkMeta(snap *webapi.Snapshot) (rpt.Report, string) {
	if s.o.URLBase == "" {
		why := s.o.NoLinks
		if why == "" {
			why = "this server has no viewer to link to"
		}
		return rpt.Report{}, why
	}
	entry := snap.GetTiers().GetNetlistUri()
	if entry == "" {
		entry = snap.GetUri()
	}
	u, err := artifact.Parse(entry)
	if err != nil || u.Path == "" {
		return rpt.Report{}, fmt.Sprintf("%s names no file in a mount", entry)
	}
	return rpt.Report{Design: entry, URLBase: strings.TrimRight(s.o.URLBase, "/"), MountPath: u.Mount + "/" + u.Path}, ""
}
