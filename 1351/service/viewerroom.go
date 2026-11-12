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

// RoomTokens mints relay tokens, and a viewer page joins its design's room with one. The host that
// runs the relay implements RoomTokens, since only that host holds the relay's key
// (internal/viewerrooms).
type RoomTokens interface {
	// Mint returns a token admitting its holder to room alone, and the time it stops admitting a join.
	Mint(room string) (token string, expires time.Time, err error)
	// Bridges returns the relay subjects that the host's room bridges join as. A page answers these
	// subjects alone.
	Bridges() []string
}

// ViewerRoomService answers an open viewer page before the page offers its tools to agents through
// its design's relay room (agni issue 1154). It tells the page which room to join, and gives it a
// token to join with.
type ViewerRoomService struct {
	projects *ProjectResolver
	tokens   RoomTokens
	relayURL string
}

// NewViewerRoomService answers JoinViewerRoom for a relay served at relayURL ("/relay/"), with
// tokens minted by tokens. Every other service holds the same resolver as projects. So a design names
// the same room here as in the bridge's WithRoomOf, which calls Room.
func NewViewerRoomService(projects *ProjectResolver, tokens RoomTokens, relayURL string) *ViewerRoomService {
	return &ViewerRoomService{projects: projects, tokens: tokens, relayURL: relayURL}
}

// Room returns the room for a design named as mount/path or a mount:// URI. The room is "design:"
// followed by the design's declared entry. linkMountPath resolves the entry, as it does for a
// report's links (C32), so the folder, the entry and a declared companion name one room. An empty
// design is an invalid argument.
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

// JoinViewerRoom names the design's room, mints a token for it, and names the bridges the page
// should answer. It stores nothing.
func (s *ViewerRoomService) JoinViewerRoom(ctx context.Context, req *webapi.JoinViewerRoomRequest) (*webapi.JoinViewerRoomResponse, error) {
	room, err := s.Room(ctx, req.GetDesign())
	if err != nil {
		return nil, err
	}
	token, expires, err := s.tokens.Mint(room)
	if err != nil {
		return nil, err
	}
	return &webapi.JoinViewerRoomResponse{Room: room, RelayUrl: s.relayURL, Token: token, ExpireTime: timestamppb.New(expires), BridgeSubjects: s.tokens.Bridges()}, nil
}
