package service

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/proto"
)

// LocateSubjects answers which sheets draw each subject (agni issue 1325). It reads the snapshot's
// netlist and schematic, and its board only when the request asks. It uses the same index a check
// run's findings use (sheetIndex), so a subject locates here exactly where its finding badges.
//
// A subject drawn nowhere carries the reason a finding would: NOT_IN_DESIGN for a name the netlist
// lacks, a power rail or a virtual symbol for those, and NO_GEOMETRY otherwise. A request naming no
// subjects is an invalid argument, since an empty answer would read as "drawn nowhere". A design
// with no board answers without the board sheet rather than failing, because "board" asks where the
// subject is drawn and a board-less design draws it on no board.
func (s *DesignService) LocateSubjects(ctx context.Context, req *webapi.LocateSubjectsRequest) (*webapi.LocateSubjectsResponse, error) {
	if len(req.GetSubjects()) == 0 {
		return nil, fmt.Errorf("%w: name at least one subject to locate", ErrInvalidArgument)
	}
	ctx, snap, err := s.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	opts := snap.Overlay.ReadOptions()
	loader := LoaderOr(snap, s.loader)
	d, err := loader.Design(ctx, snap.Netlist, opts...)
	if err != nil {
		return nil, ClassifyLoadErr(err)
	}
	m := check.NewModel(d)
	g, unreadable := snapGeometry(ctx, loader, snap, opts...)
	ix := indexSheets(g, m)
	ix.unreadable = unreadable
	// A board-only design locates on its board whatever the request asks, since it lists no other
	// sheet (agni issue 1466).
	if req.GetBoard() || snap.BoardOnly() {
		if ix.board, err = boardIndexFor(ctx, loader, snap.BoardRead()); err != nil {
			return nil, err
		}
	}
	resp := &webapi.LocateSubjectsResponse{Subjects: make([]*webapi.SubjectSheets, len(req.GetSubjects()))}
	for i, sub := range req.GetSubjects() {
		out := &webapi.SubjectSheets{Subject: proto.Clone(sub).(*checkspb.Subject), SheetIds: ix.sheetsFor(sub)}
		if len(out.SheetIds) == 0 {
			out.LocateReason = ix.reasonFor(sub, m)
		}
		resp.Subjects[i] = out
	}
	return resp, nil
}
