package service

import (
	"context"

	"github.com/panyam/agni/core/num"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/jaala/ns"
)

// A QueryRead is one snapshot read for a program that something other than RunQuery evaluates, such
// as the MCP server, whose query tool is jaala's (agni issue 910). It holds what RunQuery reads, the
// fact base, the vocabulary with the project's library and the model, so the evaluator answers over
// the same facts, and Present turns its rows into the response RunQuery would give.
type QueryRead struct {
	s        *QueryService
	snapshot string
	d        *designRead
}

// ReadQuery reads the named snapshot for q, with the board tier only when q reads a board.*
// relation, as RunQuery does (agni issue 987). A snapshot the host does not hold, or one whose files
// changed, is a *SnapshotError, as it is for RunQuery.
func (s *QueryService) ReadQuery(ctx context.Context, snapshot string, q query.Query) (*QueryRead, error) {
	ctx, snap, err := s.projects.Snapshot(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	d, err := s.read(ctx, snap, q)
	if err != nil {
		return nil, err
	}
	return &QueryRead{s: s, snapshot: snapshot, d: d}, nil
}

// Base is the fact base a program is evaluated over. Evaluate with query.EvalOptions(ctx), as
// RunQuery does, so a served work budget reaches it.
func (r *QueryRead) Base() *query.Base { return r.d.base }

// Vocabulary is the vocabulary Base answers over, the project's library included.
func (r *QueryRead) Vocabulary() *ns.Vocabulary { return r.d.reg.Vocabulary() }

// Present is the response for rows [lo, hi) of table, which q evaluated over Base with bind, echoing
// queryText. It is RunQuery's own projection, so a row's cells, kinds, entities and citations are what
// RunQuery gives for the same query. Work is left zero, since the evaluation was not RunQuery's, and
// Total is the table's length when the rows are a page of it.
func (r *QueryRead) Present(ctx context.Context, q query.Query, queryText string, table *query.Table, lo, hi int, bind map[string]query.Value) (*webapi.RunQueryResponse, error) {
	// Answering under the snapshot's context records the geometry the cells are located on, as
	// RunQuery's reads are.
	ctx, _, err := r.s.projects.Snapshot(ctx, r.snapshot)
	if err != nil {
		return nil, err
	}
	lo, hi = max(0, lo), min(hi, len(table.Rows))
	resp := r.d.present(ctx, q, queryText, table, lo, max(lo, hi), bind, false)
	if lo > 0 || hi < len(table.Rows) {
		resp.Total = num.Int32(len(table.Rows))
	}
	return resp, nil
}

// Vocabulary is the vocabulary a question about the named snapshot is checked against, the project's
// library included, composed without reading the design, so describing it costs no read. With no
// snapshot it is the shipped vocabulary.
func (s *QueryService) Vocabulary(ctx context.Context, snapshot string) (*ns.Vocabulary, error) {
	reg, err := s.catalogRegistry(ctx, snapshot, nil)
	if err != nil {
		return nil, err
	}
	return reg.Vocabulary(), nil
}
