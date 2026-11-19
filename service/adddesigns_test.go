package service_test

import (
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

func proposal(folder, name string) *webapi.ProposedDesign {
	return &webapi.ProposedDesign{Folder: folder, Design: &webapi.Design{Name: name}}
}

// Boards of the same names in two folders get ids that differ, so adding all of them doesn't
// collide (agni issue 1756).
func TestDefaultDesignIDsDifferForBoardsOfOneNameInTwoFolders(t *testing.T) {
	all := []*webapi.ProposedDesign{
		proposal("rev-a", "gateway"), proposal("rev-a", "spare"),
		proposal("rev-b", "gateway"), proposal("rev-b", "spare"),
		proposal("labs/solo", "solo"),
		proposal("", "root-board"),
	}
	want := []string{"rev-a-gateway", "rev-a-spare", "rev-b-gateway", "rev-b-spare", "solo", "root-board"}
	for i, p := range all {
		if got := service.DefaultDesignID(p, all); got != want[i] {
			t.Errorf("DefaultDesignID(%s, %s) = %q, want %q", p.GetFolder(), p.GetDesign().GetName(), got, want[i])
		}
	}
	releases := []*webapi.ProposedDesign{proposal("jetson-1.1.2", "jetson"), proposal("jetson-1.1.3", "jetson")}
	if a, b := service.DefaultDesignID(releases[0], releases), service.DefaultDesignID(releases[1], releases); a != "jetson-1.1.2" || b != "jetson-1.1.3" {
		t.Errorf("two releases in two folders = %q and %q; want each folder's name", a, b)
	}
}
