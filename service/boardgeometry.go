package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// The page budget GetBoardGeometry uses when a request names none, and the most it allows. The cap
// keeps a page under the 4 MB that grpc-go, grpc-java and gRPC C++ accept by default.
const (
	DefaultBoardPageBytes = 1 << 20
	MaxBoardPageBytes     = 3 << 20
)

// GetBoardGeometry answers a page of the snapshot's board (agni issue 1634). The first page holds the
// header, which is every field but the five lists, and then as many whole list elements as fit.
// Each later page continues with the next element, through placements, nets, zones, texts and
// graphics in that order. Merging the pages with proto.Merge gives back the board boardFor reads.
//
// A page holds at least one element. So an element larger than the budget makes a page of its own,
// and a header larger than the budget makes the first page alone.
func (s *DesignService) GetBoardGeometry(ctx context.Context, req *webapi.GetBoardGeometryRequest) (*webapi.GetBoardGeometryResponse, error) {
	budget := req.GetMaxBytes()
	switch {
	case budget < 0:
		return nil, fmt.Errorf("%w: max_bytes is %d, and must be 0 or more", ErrInvalidArgument, budget)
	case budget == 0:
		budget = DefaultBoardPageBytes
	case budget > MaxBoardPageBytes:
		budget = MaxBoardPageBytes
	}
	ctx, snap, err := s.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	at := boardCursor{}
	if t := req.GetPageToken(); t != "" {
		if at, err = parseBoardToken(t, snap.Revision); err != nil {
			return nil, err
		}
	}
	b, err := s.boardFor(ctx, snap)
	if err != nil {
		return nil, err
	}
	page, next := boardPage(b, at, int(budget))
	resp := &webapi.GetBoardGeometryResponse{Board: page}
	if next.section < len(boardSections) {
		resp.NextPageToken = next.token(snap.Revision)
	}
	return resp, nil
}

// boardSection is one of BoardGeometry's lists, which pages carry element by element.
type boardSection struct {
	field protowire.Number
	n     func(b *geom.BoardGeometry) int
	get   func(b *geom.BoardGeometry, i int) proto.Message
	add   func(page, b *geom.BoardGeometry, i int)
}

// boardSections lists the five lists in the order pages carry them.
var boardSections = []boardSection{
	{5, func(b *geom.BoardGeometry) int { return len(b.GetPlacements()) },
		func(b *geom.BoardGeometry, i int) proto.Message { return b.GetPlacements()[i] },
		func(p, b *geom.BoardGeometry, i int) { p.Placements = append(p.Placements, b.GetPlacements()[i]) }},
	{6, func(b *geom.BoardGeometry) int { return len(b.GetNets()) },
		func(b *geom.BoardGeometry, i int) proto.Message { return b.GetNets()[i] },
		func(p, b *geom.BoardGeometry, i int) { p.Nets = append(p.Nets, b.GetNets()[i]) }},
	{7, func(b *geom.BoardGeometry) int { return len(b.GetZones()) },
		func(b *geom.BoardGeometry, i int) proto.Message { return b.GetZones()[i] },
		func(p, b *geom.BoardGeometry, i int) { p.Zones = append(p.Zones, b.GetZones()[i]) }},
	{8, func(b *geom.BoardGeometry) int { return len(b.GetTexts()) },
		func(b *geom.BoardGeometry, i int) proto.Message { return b.GetTexts()[i] },
		func(p, b *geom.BoardGeometry, i int) { p.Texts = append(p.Texts, b.GetTexts()[i]) }},
	{9, func(b *geom.BoardGeometry) int { return len(b.GetGraphics()) },
		func(b *geom.BoardGeometry, i int) proto.Message { return b.GetGraphics()[i] },
		func(p, b *geom.BoardGeometry, i int) { p.Graphics = append(p.Graphics, b.GetGraphics()[i]) }},
}

// boardCursor is where a page starts. A first page starts at the header.
type boardCursor struct {
	header  bool // the header is already sent
	section int  // index into boardSections
	index   int  // element within that section
}

// boardPage cuts one page of b from at, and says where the next page starts. The page shares b's
// elements, so a caller must not change them. That is safe for the cached board boardFor returns,
// since a page is only marshalled.
//
// A page stops before the element that would take it past budget. The page that carries the header
// may stop before its first element. Any other page takes its first element whatever its size, so
// every page moves the cursor on.
func boardPage(b *geom.BoardGeometry, at boardCursor, budget int) (*geom.BoardGeometry, boardCursor) {
	page := &geom.BoardGeometry{}
	withHeader := !at.header
	if withHeader {
		page.DesignRef, page.UnitNm, page.Layers = b.GetDesignRef(), b.GetUnitNm(), b.GetLayers()
		page.Outline, page.Padstacks, page.Stackup, page.Prov = b.GetOutline(), b.GetPadstacks(), b.GetStackup(), b.GetProv()
		at.header = true
	}
	size, elements := proto.Size(page), 0
	for at.section < len(boardSections) {
		sec := boardSections[at.section]
		if at.index >= sec.n(b) {
			at.section, at.index = at.section+1, 0
			continue
		}
		// An element costs its tag, its length and its bytes.
		cost := protowire.SizeTag(sec.field) + protowire.SizeBytes(proto.Size(sec.get(b, at.index)))
		if size+cost > budget && (elements > 0 || withHeader) {
			break
		}
		sec.add(page, b, at.index)
		size, elements, at.index = size+cost, elements+1, at.index+1
	}
	// Step past lists the page finished, so a page that ends the board says so, and no empty page
	// follows it.
	for at.section < len(boardSections) && at.index >= boardSections[at.section].n(b) {
		at.section, at.index = at.section+1, 0
	}
	return page, at
}

// token encodes the cursor with the revision it belongs to. Clients treat it as opaque.
func (c boardCursor) token(revision string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("b1|%s|%d|%d", revision, c.section, c.index)))
}

// parseBoardToken reads a token token wrote, and refuses one from another revision or another
// format.
func parseBoardToken(t, revision string) (boardCursor, error) {
	bad := func(why string) (boardCursor, error) {
		return boardCursor{}, fmt.Errorf("%w: page_token %q %s; start again with an empty page_token", ErrInvalidArgument, t, why)
	}
	raw, err := base64.RawURLEncoding.DecodeString(t)
	if err != nil {
		return bad("is not a token this server wrote")
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 || parts[0] != "b1" {
		return bad("is not a token this server wrote")
	}
	if parts[1] != revision {
		return bad("belongs to another revision of the design")
	}
	section, err1 := strconv.Atoi(parts[2])
	index, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil || section < 0 || section >= len(boardSections) || index < 0 {
		return bad("is not a token this server wrote")
	}
	return boardCursor{header: true, section: section, index: index}, nil
}
