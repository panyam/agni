package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RoomTokens mints the relay tokens viewer pages join their design's room with. The host that runs
// the relay implements it, since only it holds the key the relay checks (internal/viewerrooms).
type RoomTokens interface {
	// Mint returns a token admitting its holder to room alone, and when it stops admitting a join.
	Mint(room string) (token string, expires time.Time, err error)
}

// ViewerRoomService is what an open viewer page asks before it offers its tools to agents through
// its design's relay room (agni issue 1154): which room, and a token to join it with.
type ViewerRoomService struct {
	projects *ProjectResolver
	tokens   RoomTokens
	relayURL string
}

// NewViewerRoomService answers JoinViewerRoom for a relay served at relayURL ("/relay/"), with
// tokens minted by tokens. projects is the resolver every other service holds, so a design names the
// same room here as in the bridge's WithRoomOf, which calls Room.
func NewViewerRoomService(projects *ProjectResolver, tokens RoomTokens, relayURL string) *ViewerRoomService {
	return &ViewerRoomService{projects: projects, tokens: tokens, relayURL: relayURL}
}

// Room is the room for a design named as mount/path or a mount:// URI: "design:" and the design's
// declared entry, resolved by linkMountPath as a report's links are (C32), so the folder, the entry
// and a declared companion name one room. An empty design is an invalid argument.
func (s *ViewerRoomService) Room(ctx context.Context, design string) (string, error) {
	d := strings.TrimSpace(design)
	if d == "" {
		return "", fmt.Errorf("%w: name a design, as mount/path", ErrInvalidArgument)
	}
	if !strings.HasPrefix(d, artifact.Scheme+"://") {
		d = artifact.Scheme + "://" + strings.TrimPrefix(d, "/")
	}
	p, err := linkMountPath(ctx, s.projects, d)
	if err != nil {
		return "", err
	}
	return "design:" + p, nil
}

// JoinViewerRoom names the design's room and mints a token for it. It stores nothing.
func (s *ViewerRoomService) JoinViewerRoom(ctx context.Context, req *webapi.JoinViewerRoomRequest) (*webapi.JoinViewerRoomResponse, error) {
	room, err := s.Room(ctx, req.GetDesign())
	if err != nil {
		return nil, err
	}
	token, expires, err := s.tokens.Mint(room)
	if err != nil {
		return nil, err
	}
	return &webapi.JoinViewerRoomResponse{Room: room, RelayUrl: s.relayURL, Token: token, ExpireTime: timestamppb.New(expires)}, nil
}
