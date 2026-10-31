package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/panyam/mcpkit/core"
	mcpkit "github.com/panyam/mcpkit/server"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// maxLinks is the most verdict links one check answer lists; each verdict's id is in the tables.
const maxLinks = 50

// registerTools adds agni's own tools, each one wrapping a service the Connect API serves.
func (s *srv) registerTools(srv *mcpkit.Server) {
	addTool(s, srv, "designs", designsDesc, s.designsTool)
	addTool(s, srv, "check", checkDesc, s.check)
	addTool(s, srv, "trace", traceDesc, s.trace)
	addTool(s, srv, "diff", diffDesc, s.diff)
	addTool(s, srv, "design", designDesc, s.design)
	addTool(s, srv, "checklists", checklistsDesc, s.checklists)
	addTool(s, srv, "review", reviewDesc, s.review)
	addTool(s, srv, "query_set", querySetDesc, s.querySet)
}

// addTool registers fn as name, taking In's arguments and no others, so a misspelled argument is a
// tool error rather than ignored, as on jaala's tools. fn runs under the server's budget and timeout.
func addTool[In any](s *srv, srv *mcpkit.Server, name, desc string, fn func(context.Context, In) (core.ToolResult, error)) {
	srv.RegisterTool(core.ToolDef{Name: name, Description: desc, InputSchema: closed(core.GenerateSchema[In]())},
		func(tc core.ToolContext, req core.ToolRequest) (core.ToolResponse, error) {
			args := req.Arguments
			if len(bytes.TrimSpace(args)) == 0 {
				args = json.RawMessage("{}")
			}
			var in In
			if err := json.Unmarshal(args, &in); err != nil {
				return core.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
			ctx := s.withBudget(tc.Context)
			if s.o.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, s.o.Timeout)
				defer cancel()
			}
			return fn(ctx, in)
		})
}

// closed is schema with additionalProperties false.
func closed(schema any) any {
	data, err := json.Marshal(schema)
	if err != nil {
		return schema
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return schema
	}
	m["additionalProperties"] = false
	return m
}

// answer is a tool's text, then its structured content: the design, what was asked, the tables and
// the wire message the service returned.
func answer(text string, data map[string]any) (core.ToolResult, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return core.ToolResult{}, err
	}
	return core.StructuredResult(text, json.RawMessage(raw)), nil
}

// tabulate is TableService's projection of an answer (C35).
func tabulate(ctx context.Context, req *webapi.TabulateRequest) ([]*webapi.Table, error) {
	resp, err := service.TableService{}.Tabulate(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetTables(), nil
}

// on runs fn over the snapshot of the design an agent named.
func (s *srv) on(ctx context.Context, design string, fn func(*webapi.Snapshot) error) (artifact.URI, error) {
	u, err := designURI(design)
	if err != nil {
		return u, err
	}
	return u, s.designs.ask(ctx, u, fn)
}

const designsDesc = `List the designs this server can answer about, each with the name to pass as design and the revisions it declares (for diff). Call it first when you don't know a design's name.`

type designsInput struct{}

func (s *srv) designsTool(ctx context.Context, _ designsInput) (core.ToolResult, error) {
	list, err := s.listDesigns(ctx)
	if err != nil {
		return failed(err), nil
	}
	var b strings.Builder
	if len(list.Designs) == 0 {
		b.WriteString("no project on this server declares a design")
	}
	for _, d := range list.Designs {
		fmt.Fprintf(&b, "%s", d.Design)
		if d.Title != "" {
			fmt.Fprintf(&b, "  (%s)", d.Title)
		}
		b.WriteString("\n")
		for _, r := range d.Revisions {
			fmt.Fprintf(&b, "  revision: %s\n", r)
		}
	}
	if len(list.Mounts) > 0 {
		fmt.Fprintf(&b, "\nmounts: %s. A design outside a project is named by its file, as mount/path.\n", strings.Join(list.Mounts, ", "))
	}
	return answer(b.String(), map[string]any{"designs": list.Designs, "mounts": list.Mounts})
}

const checkDesc = `Run agni's design checks over a design and get every finding, with the verdict each rule reached on each subject, and a link per failing verdict that opens it in the viewer. rules narrows the run to those rules (the agni://rules resource lists them).`

type checkInput struct {
	Design string   `json:"design" jsonschema:"required" jsonschema_description:"The design, as mount/path (tut/designs/gateway)."`
	Rules  []string `json:"rules,omitempty" jsonschema_description:"Only these rules, by id. Empty runs every rule."`
}

func (s *srv) check(ctx context.Context, in checkInput) (core.ToolResult, error) {
	var resp *webapi.CheckDesignResponse
	var snap *webapi.Snapshot
	u, err := s.on(ctx, in.Design, func(sn *webapi.Snapshot) (err error) {
		snap = sn
		resp, err = s.api.Check.CheckDesign(ctx, &webapi.CheckDesignRequest{Snapshot: sn.GetName(), Rules: in.Rules})
		return err
	})
	if err != nil {
		return failed(err), nil
	}
	tables, err := tabulate(ctx, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: resp}})
	if err != nil {
		return failed(err), nil
	}
	meta, why := s.linkMeta(snap)
	if why == "" {
		meta.ContentHash = s.contentHash(ctx, snap)
	}
	var links []verdictLink
	for _, p := range resp.GetVerdicts() {
		v := service.VerdictFromProto(p)
		if v.Outcome != check.Fail && v.Outcome != check.Inconclusive {
			continue
		}
		subjects := check.SubjectParams(v.Subjects)
		if url := rpt.VerdictURL(meta, p.GetId(), v.Rule, subjects); url != "" && len(links) < maxLinks {
			links = append(links, verdictLink{Verdict: p.GetId(), Rule: v.Rule, Subjects: subjects, Link: url, Page: verdictAction(p.GetId(), v.Rule, subjects)})
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "design: %s\nchecked: %s\n\n", u, rulesAsked(in.Rules))
	for _, t := range tables {
		// The verdict list repeats the findings with every pass beside them; the per-rule counts and
		// the findings say the same in a page an agent can read, and the structured content has it.
		if t.GetName() == "verdicts" {
			continue
		}
		writeTable(&b, t)
	}
	for _, l := range links {
		fmt.Fprintf(&b, "open %s (%s) in the viewer: %s\n", l.Verdict, l.Rule, l.Link)
	}
	if why != "" {
		writeLink(&b, "", why)
	}
	return answer(b.String(), map[string]any{"design": u.String(), "rules": in.Rules, "links": links, "no_link": why,
		"tables": tablesJSON(tables), "answer": protoJSON(resp)})
}

// verdictLink is one failing verdict a check answer links to. A host with no viewer open opens Link;
// one embedding a viewer sends Page to it instead (agni issue 1155).
type verdictLink struct {
	Verdict  string      `json:"verdict"`
	Rule     string      `json:"rule"`
	Subjects []string    `json:"subjects"`
	Link     string      `json:"link"`
	Page     *pageAction `json:"page,omitempty"`
}

func rulesAsked(rules []string) string {
	if len(rules) == 0 {
		return "every rule"
	}
	return strings.Join(rules, ", ")
}

const traceDesc = `Follow a signal from one pin to another through the series parts between them (resistors, inductors, ferrites, fuses; never a capacitor), and get the route, the nets it crosses and where each can be probed. Name each pin as ref.pin (U7.3). An endpoint naming no pin the design has is an error, which is not the same as two pins with no route between them.`

type traceInput struct {
	Design string `json:"design" jsonschema:"required" jsonschema_description:"The design, as mount/path (tut/designs/gateway)."`
	From   string `json:"from" jsonschema:"required" jsonschema_description:"The pin to start at, as ref.pin: U7.3."`
	To     string `json:"to" jsonschema:"required" jsonschema_description:"The pin to reach, as ref.pin: J2.1."`
	Hops   int32  `json:"hops,omitempty" jsonschema_description:"The most series parts to cross; the server's default when 0."`
}

func (s *srv) trace(ctx context.Context, in traceInput) (core.ToolResult, error) {
	from, err := endpoint(in.From, "from")
	if err != nil {
		return failed(err), nil
	}
	to, err := endpoint(in.To, "to")
	if err != nil {
		return failed(err), nil
	}
	var resp *webapi.TraceDesignResponse
	var snap *webapi.Snapshot
	u, err := s.on(ctx, in.Design, func(sn *webapi.Snapshot) (err error) {
		snap = sn
		resp, err = s.api.Design.TraceDesign(ctx, &webapi.TraceDesignRequest{Snapshot: sn.GetName(), From: from, To: to, Hops: in.Hops})
		return err
	})
	if err != nil {
		return failed(err), nil
	}
	t := service.TraceFromProto(resp.GetTrace())
	var b strings.Builder
	fmt.Fprintf(&b, "design: %s\ntrace: %s --> %s\n\n", u, in.From, in.To)
	rpt.WriteTraceText(&b, t)
	meta, why := s.linkMeta(snap)
	link := rpt.TraceURL(meta, in.From, in.To, int(in.Hops))
	b.WriteString("\n")
	writeLink(&b, link, why)
	res, err := answer(b.String(), map[string]any{"design": u.String(), "from": in.From, "to": in.To, "link": link, "page": linkAction(link), "no_link": why, "answer": protoJSON(resp)})
	// A pin the design lacks is the question's mistake, so the agent hears it as one.
	res.IsError = t.Outcome == check.TraceUnresolved
	return res, err
}

// endpoint is a pin as ref.pin, the spelling `agni trace` takes.
func endpoint(s, arg string) (*webapi.TraceEndpoint, error) {
	ref, pin, ok := strings.Cut(strings.TrimSpace(s), ".")
	if !ok || ref == "" || pin == "" {
		return nil, fmt.Errorf("%s %q: name a pin as <ref-des>.<pin>, such as U7.3", arg, s)
	}
	return &webapi.TraceEndpoint{RefDes: ref, Pin: pin}, nil
}

const diffDesc = `Compare two revisions of a design and get what changed: components added, removed or changed, and nets whose connections changed. design is the earlier revision and against the later one (the designs tool lists each design's revisions). A net that was renamed AND rewired reports as one added and one removed unless rename_approx is set.`

type diffInput struct {
	Design       string `json:"design" jsonschema:"required" jsonschema_description:"The earlier revision, as mount/path."`
	Against      string `json:"against" jsonschema:"required" jsonschema_description:"The later revision, as mount/path."`
	RenameApprox bool   `json:"rename_approx,omitempty" jsonschema_description:"Match a net that was renamed and rewired to its nearest counterpart, as diff --rename-approx does."`
}

func (s *srv) diff(ctx context.Context, in diffInput) (core.ToolResult, error) {
	var resp *webapi.DiffDesignsResponse
	var bu artifact.URI
	var snaps [2]*webapi.Snapshot
	au, err := s.on(ctx, in.Design, func(a *webapi.Snapshot) error {
		var err error
		bu, err = s.on(ctx, in.Against, func(b *webapi.Snapshot) (err error) {
			snaps = [2]*webapi.Snapshot{a, b}
			req := &webapi.DiffDesignsRequest{ASnapshot: a.GetName(), BSnapshot: b.GetName()}
			if in.RenameApprox {
				req.NearRenames = &webapi.NearRenameOptions{}
			}
			resp, err = s.api.Diff.DiffDesigns(ctx, req)
			return err
		})
		return err
	})
	if err != nil {
		return failed(err), nil
	}
	tables, err := tabulate(ctx, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Diff{Diff: resp}})
	if err != nil {
		return failed(err), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff: %s --> %s\n\n", au, bu)
	for _, t := range tables {
		writeTable(&b, t)
	}
	// One link opens the earlier revision with the viewer's Compare started against the later, so the
	// reader lands on the same change table (agni issue 1126).
	var link string
	meta, why := s.linkMeta(snaps[0])
	if why == "" {
		var bmeta rpt.Report
		bmeta, why = s.linkMeta(snaps[1])
		link = rpt.DiffURL(meta, bmeta, in.RenameApprox)
	}
	writeLink(&b, link, why)
	return answer(b.String(), map[string]any{"design": au.String(), "against": bu.String(), "rename_approx": in.RenameApprox,
		"link": link, "page": linkAction(link), "no_link": why, "tables": tablesJSON(tables), "answer": protoJSON(resp)})
}

const designDesc = `Read a design's IR: its components, nets and pins as agni read them, narrowed by read_mask (design.nets, design.components.mpn, ...) and by nets and ref_des. Prefer query for a question; this is for reading what a part or net is.`

type designInput struct {
	Design   string   `json:"design" jsonschema:"required" jsonschema_description:"The design, as mount/path."`
	ReadMask []string `json:"read_mask,omitempty" jsonschema_description:"The fields to read, such as design.nets or design.components.mpn; * for everything. Empty reads design.nets and design.components."`
	Nets     []string `json:"nets,omitempty" jsonschema_description:"Only these nets."`
	RefDes   []string `json:"ref_des,omitempty" jsonschema_description:"Only these components."`
}

func (s *srv) design(ctx context.Context, in designInput) (core.ToolResult, error) {
	mask := in.ReadMask
	if len(mask) == 0 {
		mask = []string{"design.nets", "design.components"}
	}
	var resp *webapi.GetDesignResponse
	u, err := s.on(ctx, in.Design, func(sn *webapi.Snapshot) (err error) {
		resp, err = s.api.Design.GetDesign(ctx, &webapi.GetDesignRequest{Snapshot: sn.GetName(), ReadMask: &fieldmaskpb.FieldMask{Paths: mask}, Nets: in.Nets, RefDes: in.RefDes})
		return err
	})
	if err != nil {
		return failed(err), nil
	}
	data := protoJSON(resp)
	var indented bytes.Buffer
	if json.Indent(&indented, data, "", "  ") != nil {
		indented.Write(data)
	}
	return answer(fmt.Sprintf("design: %s\nread_mask: %s\n\n%s\n", u, strings.Join(mask, ", "), indented.String()), map[string]any{"design": u.String(), "answer": data})
}

const checklistsDesc = `List the review checklists a design has, its project's and any it inherits, each with its items. Run one with the review tool.`

type checklistsInput struct {
	Design string `json:"design" jsonschema:"required" jsonschema_description:"The design, as mount/path."`
}

func (s *srv) checklists(ctx context.Context, in checklistsInput) (core.ToolResult, error) {
	resp, u, err := s.listChecklists(ctx, in.Design)
	if err != nil {
		return failed(err), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "design: %s\n\n", u)
	if len(resp.GetChecklists()) == 0 {
		b.WriteString("this design has no checklist\n")
	}
	for _, c := range resp.GetChecklists() {
		fmt.Fprintf(&b, "%s\n", c.GetName())
		for _, area := range c.GetManifest().GetAreas() {
			fmt.Fprintf(&b, "  %s\n", area.GetName())
			for _, it := range area.GetItems() {
				fmt.Fprintf(&b, "    %s: %s\n", it.GetId(), it.GetTitle())
			}
		}
	}
	return answer(b.String(), map[string]any{"design": u.String(), "answer": protoJSON(resp)})
}

func (s *srv) listChecklists(ctx context.Context, design string) (*webapi.ListChecklistsResponse, artifact.URI, error) {
	var resp *webapi.ListChecklistsResponse
	u, err := s.on(ctx, design, func(sn *webapi.Snapshot) (err error) {
		resp, err = s.api.Review.ListChecklists(ctx, &webapi.ListChecklistsRequest{Snapshot: sn.GetName()})
		return err
	})
	return resp, u, err
}

const reviewDesc = `Score a design against one of its review checklists and get each item's answer (pass, fail, not applicable) with the findings behind it. Nothing is saved. checklist is a name the checklists tool lists; empty runs the design's first.`

type reviewInput struct {
	Design    string `json:"design" jsonschema:"required" jsonschema_description:"The design, as mount/path."`
	Checklist string `json:"checklist,omitempty" jsonschema_description:"A checklist's name, as the checklists tool lists it. Empty runs the first."`
}

func (s *srv) review(ctx context.Context, in reviewInput) (core.ToolResult, error) {
	list, u, err := s.listChecklists(ctx, in.Design)
	if err != nil {
		return failed(err), nil
	}
	var named *webapi.NamedChecklist
	var names []string
	for _, c := range list.GetChecklists() {
		names = append(names, c.GetName())
		if named == nil && (in.Checklist == "" || c.GetName() == in.Checklist) {
			named = c
		}
	}
	if named == nil {
		if len(names) == 0 {
			return failed(fmt.Errorf("%s has no checklist", u)), nil
		}
		return failed(fmt.Errorf("checklist %q: %s has %s", in.Checklist, u, strings.Join(names, ", "))), nil
	}
	var resp *webapi.Review
	if _, err := s.on(ctx, in.Design, func(sn *webapi.Snapshot) (err error) {
		resp, err = s.api.Review.CreateReview(ctx, &webapi.CreateReviewRequest{Snapshot: sn.GetName(), Manifest: named.GetManifest(), ValidateOnly: true})
		return err
	}); err != nil {
		return failed(err), nil
	}
	tables, err := tabulate(ctx, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Review{Review: resp}})
	if err != nil {
		return failed(err), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "design: %s\nchecklist: %s\n\n", u, named.GetName())
	for _, t := range tables {
		writeTable(&b, t)
	}
	return answer(b.String(), map[string]any{"design": u.String(), "checklist": named.GetName(), "tables": tablesJSON(tables), "answer": protoJSON(resp)})
}

const querySetDesc = `Run one of a design's saved query sets, the tables its project keeps for every design (a power review, say), and get each query's rows with a link per query. set is a name the set lists; empty lists the sets.`

type querySetInput struct {
	Design string `json:"design" jsonschema:"required" jsonschema_description:"The design, as mount/path."`
	Set    string `json:"set,omitempty" jsonschema_description:"The set's name. Empty lists the design's sets."`
}

func (s *srv) querySet(ctx context.Context, in querySetInput) (core.ToolResult, error) {
	var sets *webapi.ListQuerySetsResponse
	var resp *webapi.RunQueriesResponse
	var snap *webapi.Snapshot
	var named *webapi.NamedQuerySet
	u, err := s.on(ctx, in.Design, func(sn *webapi.Snapshot) (err error) {
		snap = sn
		if sets, err = s.api.Query.ListQuerySets(ctx, &webapi.ListQuerySetsRequest{Snapshot: sn.GetName()}); err != nil || in.Set == "" {
			return err
		}
		for _, qs := range sets.GetQuerySets() {
			if qs.GetName() == in.Set {
				named = qs
			}
		}
		if named == nil {
			return nil
		}
		resp, err = s.api.Query.RunQueries(ctx, &webapi.RunQueriesRequest{Snapshot: sn.GetName(), Set: named.GetSet()})
		return err
	})
	if err != nil {
		return failed(err), nil
	}
	var names []string
	for _, qs := range sets.GetQuerySets() {
		names = append(names, qs.GetName())
	}
	if in.Set == "" || named == nil {
		text := fmt.Sprintf("design: %s\nquery sets: %s\n", u, strings.Join(names, ", "))
		if len(names) == 0 {
			text = fmt.Sprintf("design: %s\nthis design has no query set\n", u)
		}
		res, err := answer(text, map[string]any{"design": u.String(), "sets": names})
		res.IsError = in.Set != ""
		return res, err
	}
	tables, err := tabulate(ctx, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_QuerySet{QuerySet: resp}})
	if err != nil {
		return failed(err), nil
	}
	meta, why := s.linkMeta(snap)
	var b strings.Builder
	fmt.Fprintf(&b, "design: %s\nquery set: %s\n\n", u, in.Set)
	links := map[string]string{}
	pages := map[string]*pageAction{}
	for i, q := range named.GetSet().GetQueries() {
		fmt.Fprintf(&b, "%s: %s\n", q.GetName(), q.GetQuery())
		if i < len(tables) {
			writeTable(&b, tables[i])
		}
		if url := rpt.QuerySetURL(meta, in.Set, q.GetName()); url != "" {
			links[q.GetName()] = url
			pages[q.GetName()] = linkAction(url)
			writeQueryLink(&b, url, "")
		}
	}
	for _, r := range resp.GetResults() {
		if r.GetError() != "" {
			fmt.Fprintf(&b, "%s failed: %s\n", r.GetName(), r.GetError())
		}
	}
	writeLink(&b, "", why)
	return answer(b.String(), map[string]any{"design": u.String(), "set": in.Set, "links": links, "pages": pages, "no_link": why,
		"tables": tablesJSON(tables), "answer": protoJSON(resp)})
}

// tablesJSON is tables as they travel inside structured content.
func tablesJSON(tables []*webapi.Table) []json.RawMessage {
	out := make([]json.RawMessage, len(tables))
	for i, t := range tables {
		out[i] = protoJSON(t)
	}
	return out
}

// contentHash is the revision snap's design was read at, which a verdict link carries so the viewer
// can say when the design has changed since (agni issue 392). "" when it can't be read, and the link
// then carries none.
func (s *srv) contentHash(ctx context.Context, snap *webapi.Snapshot) string {
	resp, err := s.api.Design.GetDesign(ctx, &webapi.GetDesignRequest{Snapshot: snap.GetName()})
	if err != nil {
		return ""
	}
	return resp.GetContentHash()
}

// listed is what the designs tool and resource say: each design a project declares, by the name the
// tools take, and the mounts a file outside a project is named under.
type listed struct {
	Designs []listedDesign `json:"designs"`
	Mounts  []string       `json:"mounts"`
}

type listedDesign struct {
	Design    string   `json:"design"`
	Title     string   `json:"title,omitempty"`
	Project   string   `json:"project"`
	Revisions []string `json:"revisions,omitempty"`
}

func (s *srv) listDesigns(ctx context.Context) (listed, error) {
	out := listed{Designs: []listedDesign{}, Mounts: []string{}}
	if s.api.Workspace != nil {
		mounts, err := s.api.Workspace.ListMounts(ctx, &webapi.ListMountsRequest{})
		if err != nil {
			return out, err
		}
		// A mount's root is a host path, which an agent has no use for and a remote one should not see.
		for _, m := range mounts.GetMounts() {
			out.Mounts = append(out.Mounts, m.GetName())
		}
	}
	if s.api.Project == nil {
		return out, nil
	}
	projects, err := s.api.Project.ListProjects(ctx, &webapi.ListProjectsRequest{})
	if err != nil {
		return out, err
	}
	for _, p := range projects.GetProjects() {
		ds, err := s.api.Project.ListDesigns(ctx, &webapi.ListProjectDesignsRequest{Parent: p.GetName()})
		if err != nil {
			return out, err
		}
		for _, d := range ds.GetDesigns() {
			l := listedDesign{Design: bare(d.GetUri()), Title: d.GetTitle(), Project: p.GetName()}
			for _, r := range d.GetRevisions() {
				l.Revisions = append(l.Revisions, bare(r.GetEntryUri()))
			}
			out.Designs = append(out.Designs, l)
		}
	}
	return out, nil
}

// bare is a mount:// URI as the tools take it, mount/path.
func bare(uri string) string { return strings.TrimPrefix(uri, artifact.Scheme+"://") }
