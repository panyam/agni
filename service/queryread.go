package service

import (
	"context"

	"github.com/panyam/agni/core/num"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/jaala/ns"
)

// A QueryRead is one snapshot read for a program that RunQuery does not evaluate. The MCP server is
// one such caller, since its query tool is jaala's (agni issue 910). A QueryRead holds the fact base,
// the vocabulary with the project's library, and the model. RunQuery reads the same three, so the
// evaluator answers over the same facts. Present turns its rows into the response RunQuery would
// give.
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

// Present returns the response for rows [lo, hi) of table, echoing queryText. q produced table by
// evaluating over Base with bind. Present is RunQuery's own projection, so a row's cells, kinds,
// entities and citations match what RunQuery gives for the same query. When the evaluation recorded
// witnesses, each row carries them, as RunQuery's rows do when asked (agni issue 794). Work is left
// zero, since the evaluation was not RunQuery's. Total is the table's length when the rows are a
// page of it.
func (r *QueryRead) Present(ctx context.Context, q query.Query, queryText string, table *query.Table, lo, hi int, bind query.Bindings) (*webapi.RunQueryResponse, error) {
	// Answering under the snapshot's context records the geometry that locates the cells, as it is
	// recorded for RunQuery's reads.
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

// Vocabulary returns the vocabulary that checks a question about the named snapshot, the project's
// library included. It is composed without reading the design, so describing it costs no read. With
// no snapshot it is the shipped vocabulary.
func (s *QueryService) Vocabulary(ctx context.Context, snapshot string) (*ns.Vocabulary, error) {
	reg, err := s.catalogRegistry(ctx, snapshot, nil)
	if err != nil {
		return nil, err
	}
	return reg.Vocabulary(), nil
}
