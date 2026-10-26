// Package mcpserver serves agni's analysis to an LLM client over the Model Context Protocol (agni
// issue 910). It is a transport adapter beside internal/server's Connect one (C13), so it asks the
// same services and adds no analysis.
//
// The query language's tools come from jaala's MCP server (frontends/mcp): query, explain, schema,
// look and values, answering over the fact base of the design a call names. This package adds the
// rest of agni's questions as tools of its own (check, trace, diff, design, checklists, review,
// query_set and designs) and the catalogs as resources. Every answer carries the question that
// produced it, its rows as TableService projects them (C35), the citations they rest on and a link
// that opens the answer in the viewer, so a client shows evidence rather than the model's say-so.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/frontends/mcp"
	"github.com/panyam/jaala/frontends/nl"
	"github.com/panyam/jaala/ns"
	"github.com/panyam/mcpkit/core"
	mcpkit "github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/server/stateless"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Options configure New.
type Options struct {
	// Version is the server's version, as initialize reports it.
	Version string
	// URLBase is where the viewer is served, such as "http://localhost:8080", so an answer carries a
	// link that opens it there. Empty withholds every link, and each answer says why.
	URLBase string
	// NoLinks says why URLBase is empty, for the answers to repeat. Empty says the server was started
	// with no viewer to link to.
	NoLinks string
	// Budget caps the work of every query and query-backed rule a call evaluates, as `agni serve
	// --query-budget` caps the Connect API's (agni issue 792). Zero caps nothing.
	Budget int64
	// Timeout is the longest one call may take; zero is no limit.
	Timeout time.Duration
}

// New is the MCP server over api's services. api is the same value `agni serve` registers on its
// Connect mux, so the two transports ask the same services and hold the same snapshots.
func New(api server.API, o Options) (*mcpkit.Server, error) {
	if api.Snapshot == nil || api.Query == nil {
		return nil, errors.New("mcpserver: the API needs its snapshot and query services")
	}
	s := &srv{api: api, o: o, designs: newDesigns(api.Snapshot)}
	s.pack = mcp.Pack{
		Pack: nl.Pack{
			About:    about,
			Card:     card,
			Examples: examples,
		},
		Name:    "agni",
		Version: o.Version,
	}
	srv, err := mcp.NewWith(nil, s.pack, mcp.Options{
		Base: func(context.Context) (*datalog.Base, error) {
			return nil, errors.New("name a design: pass design, such as tut/designs/gateway (the designs tool lists them)")
		},
		Vocabulary: func(ctx context.Context) (*ns.Vocabulary, error) { return api.Query.Vocabulary(ctx, "") },
		MaxWork:    o.Budget,
		Timeout:    o.Timeout,
		Server:     []mcpkit.Option{mcpkit.WithListTTLMs(listTTLMs)},
	}, mcp.Extension[designArg]{Prepare: s.prepare})
	if err != nil {
		return nil, err
	}
	s.registerTools(srv)
	s.registerResources(srv)
	return srv, nil
}

// HTTPHandler is srv served over streamable HTTP at prefix, the path it is mounted at on a mux. Only
// a localhost Origin is accepted, so a page on another site can't drive a local server (DNS
// rebinding); origins adds others.
//
// It accepts both MCP wires on one URL (Wire), so each host speaks the one it prefers: the stateless
// 2026-07-28 wire, or the session wire a host that never asked for it gets.
func HTTPHandler(srv *mcpkit.Server, prefix string, origins ...string) http.Handler {
	opts := []mcpkit.TransportOption{
		mcpkit.WithStreamableHTTP(true), mcpkit.WithSSE(false), mcpkit.WithPrefix(prefix),
		mcpkit.WithStatelessMode(Wire()),
	}
	if len(origins) > 0 {
		opts = append(opts, mcpkit.WithAllowedOrigins(origins...))
	}
	return srv.Handler(opts...)
}

// listTTLMs is how long a client may keep the tool, resource and prompt lists before asking again:
// an hour, since agni's tools are fixed for a server's life and nothing announces a change (agni
// issue 1172). It saves every stateless client a tools/list per conversation.
const listTTLMs = 60 * 60 * 1000

// Wire is the MCP wires agni's HTTP endpoint accepts: both (stateless.ModeDual), set here rather
// than left to mcpkit's default so a change of that default can't drop the wire a host speaks (agni
// issue 1172). MCPKIT_STATELESS_MODE still overrides it, to force one wire while debugging a host.
func Wire() stateless.Mode {
	if m, ok := stateless.ParseMode(os.Getenv(stateless.ModeEnvVar)); ok {
		return m
	}
	return stateless.ModeDual
}

const about = `Electronic designs (schematics, netlists and boards) that agni reads: their parts, pins, nets and what connects them. Every board fact in an answer is a row agni returned, with the file and place it was read from.`

const card = `# A design is named by its mount and path, such as tut/designs/gateway: the folder, its entry file or a declared companion all name it. The designs tool lists them, and the revisions each one declares.
# A net is named as the design names it ("I2C_SDA"), a component by its reference designator ("U7"), and a pin by its number or name on that component ("3").
# Lengths are metres and voltages volts; write a unit on a threshold (?w < 0.2mm).
# A value the user gave, a net or a part, goes in bind rather than into the program.`

// examples are the questions the instructions show, from the tutorial project's own query set.
var examples = []nl.Example{
	{Question: "which rails have no test point?", Program: `net.rail(?net), not net.has_test_point(?net) => ?net`},
	{Question: "what does a net connect to?", Program: `pin.net(?ref, ?pin, ?net) => ?ref, ?pin`},
}

// A srv is one MCP server's state: the services it asks and the snapshots it holds.
type srv struct {
	api     server.API
	o       Options
	pack    mcp.Pack
	designs *designs
}

// designArg is the argument agni adds to every one of jaala's tools.
type designArg struct {
	Design string `json:"design,omitempty" jsonschema_description:"The design to ask about, as mount/path (tut/designs/gateway): its folder, entry file or a declared companion. The designs tool lists them. Omit only for schema, look and the shipped vocabulary."`
}

// prepare is how one of jaala's tools runs for the design it names: over that design's fact base,
// read for the program asked (so the board is read only for a program reading board.*), with the
// project's vocabulary, and answering in agni's shape.
func (s *srv) prepare(ctx context.Context, tool string, a designArg) (mcp.Run, error) {
	if strings.TrimSpace(a.Design) == "" {
		return mcp.Run{}, nil
	}
	uri, err := designURI(a.Design)
	if err != nil {
		return mcp.Run{}, err
	}
	snap, err := s.designs.snapshot(ctx, uri)
	if err != nil {
		return mcp.Run{}, err
	}
	run := mcp.Run{}
	switch tool {
	case "schema", "look", "values", "ask", "cypher":
		v, err := s.api.Query.Vocabulary(ctx, snap.GetName())
		if err != nil {
			return mcp.Run{}, err
		}
		run.Vocabulary = v
	}
	a2 := &answering{s: s, design: uri, snap: snap}
	run.BaseFor = a2.baseFor
	p := s.pack
	p.Format = a2.format
	p.Data = a2.data
	run.Pack = &p
	return run, nil
}

// designURI is a design as an agent names it, mount/path or a mount:// URI, as the artifact URI the
// services take.
func designURI(design string) (artifact.URI, error) {
	d := strings.TrimSpace(design)
	if !strings.HasPrefix(d, artifact.Scheme+"://") {
		d = artifact.Scheme + "://" + strings.TrimPrefix(d, "/")
	}
	u, err := artifact.Parse(d)
	if err != nil {
		return artifact.URI{}, fmt.Errorf("design %q: %v; name one as mount/path, such as tut/designs/gateway", design, err)
	}
	return u, nil
}

// withBudget is ctx carrying the server's work budget, which every query and query-backed rule a
// service evaluates reads, as the Connect interceptor sets it.
func (s *srv) withBudget(ctx context.Context) context.Context {
	return query.WithBudget(ctx, s.o.Budget)
}

// protoJSON is m as protojson, unpopulated fields included, ready to sit inside structured content.
func protoJSON(m proto.Message) json.RawMessage {
	b, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(m)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// failed is err as a tool error an agent can act on: the service's own message, which already names
// the argument at fault.
func failed(err error) core.ToolResult {
	return core.ErrorResult(err.Error())
}
