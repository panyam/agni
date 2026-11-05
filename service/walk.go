package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/num"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// WalkProto and WalkFromProto convert a walk between its Go form and its wire form. Each net is a
// TraceNet and each crossing a TraceCross, so a walk and a trace render through the same pieces.
// TestWalkProtoRoundTrip holds the pair to each other (C26).
func WalkProto(w check.Walk) *webapi.Walk {
	p := &webapi.Walk{
		From:   traceEndProto(w.From),
		Kind:   w.Kind,
		Radius: num.Int32(w.Radius),
		Reason: w.Reason,
	}
	for _, n := range w.Nets {
		one := TraceProto(check.Trace{Nets: []check.TraceNet{n.Net}, Crossings: append([]check.TraceCross{n.Entered}, n.AlsoEntered...)})
		p.Nets = append(p.Nets, &webapi.WalkNet{
			Hop: num.Int32(n.Hop), Net: one.GetNets()[0], Entered: one.GetCrossings()[0],
			AlsoEntered: one.GetCrossings()[1:], Ended: n.Ended,
		})
	}
	for _, b := range w.Boundary {
		p.Boundary = append(p.Boundary, &webapi.WalkBoundary{
			RefDes: b.RefDes, Class: b.Class, Hop: num.Int32(b.Hop), Net: b.Net, Reason: b.Reason,
		})
	}
	for _, e := range w.Elided {
		p.Elided = append(p.Elided, &webapi.WalkElided{Hop: num.Int32(e.Hop), Nets: num.Int32(e.Nets)})
	}
	return p
}

// WalkFromProto is WalkProto's inverse.
func WalkFromProto(p *webapi.Walk) check.Walk {
	w := check.Walk{
		From:   traceEndFromProto(p.GetFrom()),
		Kind:   p.GetKind(),
		Radius: int(p.GetRadius()),
		Reason: p.GetReason(),
	}
	for _, n := range p.GetNets() {
		one := TraceFromProto(&webapi.Trace{Nets: []*webapi.TraceNet{n.GetNet()}, Crossings: append([]*webapi.TraceCross{n.GetEntered()}, n.GetAlsoEntered()...)})
		wn := check.WalkNet{Hop: int(n.GetHop()), Net: one.Nets[0], Entered: one.Crossings[0], Ended: n.GetEnded()}
		if len(one.Crossings) > 1 {
			wn.AlsoEntered = one.Crossings[1:]
		}
		w.Nets = append(w.Nets, wn)
	}
	for _, b := range p.GetBoundary() {
		w.Boundary = append(w.Boundary, check.WalkBoundary{
			RefDes: b.GetRefDes(), Class: b.GetClass(), Hop: int(b.GetHop()), Net: b.GetNet(), Reason: b.GetReason(),
		})
	}
	for _, e := range p.GetElided() {
		w.Elided = append(w.Elided, check.WalkElided{Hop: int(e.GetHop()), Nets: int(e.GetNets())})
	}
	return w
}

// WalkDesign walks outwards from one part or pin over the snapshot's netlist (agni issue 1339). It
// calls the same check.WalkFrom the CLI calls. An unknown kind is ErrInvalidArgument, and a start
// that names nothing is an answer with its reason set, as TraceDesign answers an unresolved endpoint.
func (s *DesignService) WalkDesign(ctx context.Context, req *webapi.WalkDesignRequest) (*webapi.WalkDesignResponse, error) {
	ctx, snap, err := s.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	opts, nu, gu := snap.Overlay.ReadOptions(), snap.Netlist, snap.Geometry
	d, err := LoaderOr(snap, s.loader).Design(ctx, nu, opts...)
	if err != nil {
		return nil, err
	}
	m := check.NewModel(d)
	from := check.Endpoint{RefDes: req.GetFrom().GetRefDes(), Pin: req.GetFrom().GetPin()}
	w, err := check.WalkFrom(m, from, req.GetKind(), int(req.GetHops()), req.GetThrough())
	if errors.Is(err, check.ErrUnknownWalkKind) {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	if err != nil {
		return nil, err
	}
	p := WalkProto(w)
	AnnotateWalkSheets(p, BuildGeometry(ctx, LoaderOr(snap, s.loader), gu, opts...), m)
	return &webapi.WalkDesignResponse{Walk: p}, nil
}

// ListWalkKinds lists check.WalkKinds. It reads no design.
func (s *DesignService) ListWalkKinds(context.Context, *webapi.ListWalkKindsRequest) (*webapi.ListWalkKindsResponse, error) {
	resp := &webapi.ListWalkKindsResponse{}
	for _, k := range check.WalkKinds() {
		resp.Kinds = append(resp.Kinds, &webapi.WalkKind{Name: k.Name, Title: k.Title, Summary: k.Summary})
	}
	return resp, nil
}
