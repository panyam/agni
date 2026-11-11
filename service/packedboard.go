package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/proto"
)

// packedBoardVersion names the layout of a kept packed board. Change it when PackBoard or
// BoardLayers writes something new, so a store written by an older build reads as a miss.
const packedBoardVersion = "2"

var packedBoardCodec = &protoCodec{empty: func() proto.Message { return &webapi.GetSheetResponse{} }}

// packedBoard answers GetSheet for the board in the packed format, through the design cache (agni
// issue 1522). Packing restores the board, copies it, restores the netlist's design for net ids and
// packs every primitive, about 240 ms on Jetson in wasm. The answer is kept as a layer of its own,
// persisted like the board, so a reload restores the packed answer instead.
//
// The layer is built inside the cache, so it records the board and netlist files it read through
// their own layers. A change to either makes it stale. Its key also names the render style and
// packedBoardVersion.
//
// The kept answer is shared by every request that hits it, so a caller must not change it.
func (s *DesignService) packedBoard(ctx context.Context, snap Snap) (*webapi.GetSheetResponse, error) {
	build := func(ctx context.Context) (any, error) {
		b, err := s.boardFor(ctx, snap)
		if err != nil {
			return nil, err
		}
		style := render.WithStyle(s.styleOrDefault())
		return &webapi.GetSheetResponse{
			Content:     &webapi.GetSheetResponse_Packed{Packed: render.PackBoard(b, style, s.boardNetIDs(ctx, snap))},
			BoardLayers: render.BoardLayers(b, style),
		}, nil
	}
	l, ok := LoaderOr(snap, s.loader).(*CachingLoader)
	var key string
	if ok {
		ok, key = l.key("packedboard", cacheKey(snap.BoardRead().String(), snap.Netlist.String(), styleKey(s.styleOrDefault()), packedBoardVersion), snap.Overlay.ReadOptions())
	}
	var v any
	var err error
	if ok {
		v, err = l.cache.getPersisted(ctx, key, packedBoardCodec, build)
	} else {
		v, err = build(ctx)
	}
	if err != nil {
		return nil, err
	}
	resp, _ := v.(*webapi.GetSheetResponse)
	if resp == nil {
		return nil, fmt.Errorf("%w: no board geometry for this file", ErrNotFound)
	}
	return resp, nil
}

// styleKey names a render style for a cache key. A style is a comparable value, so its printed form
// names it.
func styleKey(st render.Style) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%#v", st)))
	return hex.EncodeToString(sum[:8])
}
