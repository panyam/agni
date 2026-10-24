package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// fixedTokens mints a token naming its room, so a test sees which room it was minted for.
type fixedTokens struct{}

func (fixedTokens) Mint(room string) (string, time.Time, error) {
	return "token-for-" + room, time.Unix(1_800_000_000, 0), nil
}

// TestEveryNameOfADesignJoinsOneRoom is C32 for the viewer's rooms (agni issue 1154): a tab opened on
// the design's folder and one opened on its entry are in one room, and an agent naming the design any
// of those ways reaches both. The names are the report links' (linkNames).
func TestEveryNameOfADesignJoinsOneRoom(t *testing.T) {
	svc := NewViewerRoomService(&ProjectResolver{Store: declaringStore{design: withCompanion()}}, fixedTokens{}, "/relay/")
	want := map[string]string{
		"the design's folder":   "design:m/d/board.edn",
		"the entry":             "design:m/d/board.edn",
		"a declared companion":  "design:m/d/board.edn",
		"an undeclared sibling": "design:m/d/other.edn",
	}
	for _, tc := range linkNames {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.JoinViewerRoom(context.Background(), &webapi.JoinViewerRoomRequest{Design: tc.uri[len("mount://"):]})
			if err != nil {
				t.Fatal(err)
			}
			if resp.GetRoom() != want[tc.name] {
				t.Errorf("room %q, want %q", resp.GetRoom(), want[tc.name])
			}
			if resp.GetToken() != "token-for-"+want[tc.name] || resp.GetRelayUrl() != "/relay/" || resp.GetExpireTime().GetSeconds() != 1_800_000_000 {
				t.Errorf("response %v", resp)
			}
			// An agent may also pass the design as a mount:// URI.
			if room, err := svc.Room(context.Background(), tc.uri); err != nil || room != want[tc.name] {
				t.Errorf("Room(%q) = %q, %v", tc.uri, room, err)
			}
		})
	}
}

func TestJoiningNamesADesign(t *testing.T) {
	svc := NewViewerRoomService(&ProjectResolver{Store: declaringStore{design: withCompanion()}}, fixedTokens{}, "/relay/")
	_, err := svc.JoinViewerRoom(context.Background(), &webapi.JoinViewerRoomRequest{Design: "  "})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("an empty design answered %v, want an invalid argument", err)
	}
}
