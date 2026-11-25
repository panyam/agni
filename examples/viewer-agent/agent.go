package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/panyam/mcpkit/client"
)

// Agent answers a reader's questions in their viewer tab, the way an LLM agent would through
// agni's /mcp, with the thinking scripted (agni issue 1907). It hears a question through
// next_request, asks the design which parts it has with agni's query tool, shows that answer in the
// tab that asked, and proposes highlighting the first part. When the reader accepts, it highlights
// the part. A real agent decides what to ask and what to propose. The calls it makes are these.
type Agent struct {
	mcp    *client.Client
	design string
	out    io.Writer
	// cursor is where the last next_request left off, so an event between two calls is not missed.
	cursor string
	// proposed holds what each open proposal does once accepted, by proposal id.
	proposed map[string]proposal
}

// proposal is an open proposal: the tab it is on and the part accepting it highlights.
type proposal struct {
	page, part string
}

// event is what next_request hands back: one page event, or none within the wait.
type event struct {
	Name string          `json:"name"`
	Page string          `json:"page"`
	Data json.RawMessage `json:"data"`
}

// The page events the agent acts on. room.ask is the reader's question from the Ask box, and
// page.proposal_answered is their click on a proposal.
const (
	eventAsk      = "room.ask"
	eventAnswered = "page.proposal_answered"
)

// partsQuery lists the design's parts. It reads the same on every design agni reads.
const partsQuery = `component.net(?r, ?n) => ?r`

// NewAgent returns an agent for one design, as mount/path, over a connected /mcp client.
func NewAgent(c *client.Client, design string, out io.Writer) *Agent {
	return &Agent{mcp: c, design: design, out: out, proposed: map[string]proposal{}}
}

// Run answers questions until ctx ends. With once, it returns after the first question's proposal
// is answered, which is how the browser exercise runs it.
func (a *Agent) Run(ctx context.Context, once bool) error {
	if err := a.Listen(ctx); err != nil {
		return err
	}
	for ctx.Err() == nil {
		ev, err := a.next(ctx)
		if err != nil {
			return err
		}
		if ev == nil {
			continue
		}
		switch ev.Name {
		case eventAsk:
			if err := a.answer(ctx, ev); err != nil {
				return err
			}
		case eventAnswered:
			done, err := a.act(ctx, ev)
			if err != nil {
				return err
			}
			if done && once {
				return nil
			}
		}
	}
	return ctx.Err()
}

// Listen marks where the agent starts hearing, so a question asked after Listen returns reaches Run
// even when Run has not yet called next_request. next_request with no cursor hears only what
// happens from then on. Run calls it when nothing else has.
func (a *Agent) Listen(ctx context.Context) error {
	if a.cursor != "" {
		return nil
	}
	_, err := a.nextWithin(ctx, 0)
	return err
}

// next waits up to 30 seconds for the next page event from the design's tabs that agreed to this
// agent. It returns nil when nothing happened.
func (a *Agent) next(ctx context.Context) (*event, error) {
	return a.nextWithin(ctx, 30)
}

func (a *Agent) nextWithin(ctx context.Context, seconds int) (*event, error) {
	args := map[string]any{"design": a.design, "wait_seconds": seconds}
	if a.cursor != "" {
		args["after"] = a.cursor
	}
	var got struct {
		Event  *event `json:"event"`
		Cursor string `json:"cursor"`
	}
	if err := a.call(ctx, "next_request", args, &got); err != nil {
		return nil, err
	}
	a.cursor = got.Cursor
	return got.Event, nil
}

// answer shows the design's parts in the tab that asked, and proposes highlighting the first.
func (a *Agent) answer(ctx context.Context, ev *event) error {
	var ask struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(ev.Data, &ask); err != nil {
		return fmt.Errorf("reading the question: %w", err)
	}
	fmt.Fprintf(a.out, "heard %q from page %s\n", ask.Text, ev.Page)
	parts, err := a.parts(ctx)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return a.call(ctx, "viewer_show_answer", map[string]any{"design": a.design, "page": ev.Page,
			"text": fmt.Sprintf("You asked: %s\nI found no parts in %s.", ask.Text, a.design)}, nil)
	}
	shown := parts[:min(len(parts), 20)]
	entities := make([]map[string]any, len(shown))
	for i, p := range shown {
		entities[i] = map[string]any{"kind": "component", "subject": p}
	}
	var answered struct {
		Answer string `json:"answer"`
	}
	if err := a.call(ctx, "viewer_show_answer", map[string]any{"design": a.design, "page": ev.Page,
		"text":     fmt.Sprintf("You asked: %s\n%s has %d parts.", ask.Text, a.design, len(parts)),
		"entities": entities}, &answered); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "showed %s: %d parts\n", answered.Answer, len(parts))
	var proposed struct {
		Proposal string `json:"proposal"`
	}
	if err := a.call(ctx, "viewer_propose", map[string]any{"design": a.design, "page": ev.Page,
		"action":   "Highlight " + parts[0] + " on the canvas",
		"entities": []map[string]any{{"kind": "component", "subject": parts[0]}}}, &proposed); err != nil {
		return err
	}
	a.proposed[proposed.Proposal] = proposal{page: ev.Page, part: parts[0]}
	fmt.Fprintf(a.out, "proposed %s: highlight %s\n", proposed.Proposal, parts[0])
	return nil
}

// act does what an accepted proposal said. It reports whether the event answered one of this
// agent's proposals.
func (a *Agent) act(ctx context.Context, ev *event) (bool, error) {
	var choice struct {
		Proposal string `json:"proposal"`
		Choice   string `json:"choice"`
	}
	if err := json.Unmarshal(ev.Data, &choice); err != nil {
		return false, fmt.Errorf("reading the choice: %w", err)
	}
	p, ok := a.proposed[choice.Proposal]
	if !ok || p.page != ev.Page {
		// Another agent's proposal on the same tab, which this agent hears too.
		return false, nil
	}
	delete(a.proposed, choice.Proposal)
	if choice.Choice != "accepted" {
		fmt.Fprintf(a.out, "%s %s\n", choice.Proposal, choice.Choice)
		return true, nil
	}
	if err := a.call(ctx, "viewer_locate_entity", map[string]any{"design": a.design, "page": p.page, "kind": "component", "subject": p.part}, nil); err != nil {
		return true, err
	}
	fmt.Fprintf(a.out, "%s accepted: highlighted %s\n", choice.Proposal, p.part)
	return true, nil
}

// parts asks agni's query tool for the design's parts, sorted and without repeats.
func (a *Agent) parts(ctx context.Context) ([]string, error) {
	var got struct {
		Tables []struct {
			Rows []struct {
				Cells []string `json:"cells"`
			} `json:"rows"`
		} `json:"tables"`
	}
	if err := a.call(ctx, "query", map[string]any{"design": a.design, "program": partsQuery}, &got); err != nil {
		return nil, err
	}
	var parts []string
	for _, t := range got.Tables {
		for _, r := range t.Rows {
			if len(r.Cells) > 0 && r.Cells[0] != "" {
				parts = append(parts, r.Cells[0])
			}
		}
	}
	slices.Sort(parts)
	return slices.Compact(parts), nil
}

// call calls one tool and decodes its structured answer into out, when out is not nil. A tool's
// error comes back as a Go error with the tool's text.
func (a *Agent) call(ctx context.Context, tool string, args map[string]any, out any) error {
	res, err := a.mcp.ToolCallFull(ctx, tool, args)
	if err != nil {
		return fmt.Errorf("%s: %w", tool, err)
	}
	if res.IsError {
		var b strings.Builder
		for _, c := range res.Content {
			b.WriteString(c.Text)
		}
		return fmt.Errorf("%s: %s", tool, b.String())
	}
	if out == nil {
		return nil
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s answered %s: %w", tool, b, err)
	}
	return nil
}
