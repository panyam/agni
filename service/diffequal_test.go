package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TestIncludeEqualAccountsForEveryNet covers agni issue 818. With include_equal, every net of both
// revisions is accounted for exactly once: an unchanged net as equal, a rename as one entry covering
// two names. Without it the response is what it was, and the status maps never list equal nets.
func TestIncludeEqualAccountsForEveryNet(t *testing.T) {
	base := filepath.Join("..", "examples", "tutorial-project", "designs", "gateway")
	svc := NewDiffService(fsQueryLoader{base: base}, nil)
	ask := func(equal bool) *webapi.DiffDesignsResponse {
		t.Helper()
		resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
			AUri: "mount://m/gateway.edn", BUri: "mount://m/gateway-rev-b.edn", IncludeEqual: equal,
		})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	off, on := ask(false), ask(true)
	kinds := map[string]int{}
	for _, n := range on.GetReport().GetNets() {
		kinds[n.GetKind()]++
	}
	for _, n := range off.GetReport().GetNets() {
		if n.GetKind() == string(diff.NetEqual) {
			t.Fatal("an equal net was reported without include_equal")
		}
	}
	if got, want := len(on.GetReport().GetNets())-kinds["equal"], len(off.GetReport().GetNets()); got != want {
		t.Errorf("include_equal changed the %d changes to %d", want, got)
	}
	if kinds["equal"] == 0 {
		t.Fatal("no equal nets on a revision pair that shares most of its nets, so the sum below proves nothing")
	}
	names := map[string]bool{}
	for _, uri := range []string{"gateway.edn", "gateway-rev-b.edn"} {
		r, err := NewQueryService(fsQueryLoader{base: base}, nil, nil).RunQuery(context.Background(),
			&webapi.RunQueryRequest{Uri: "mount://m/" + uri, Query: `entity(?n, "net") => ?n`})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range r.GetRows() {
			names[row.GetCells()[0]] = true
		}
	}
	covered := kinds["equal"] + kinds["hard"] + kinds["soft"] + kinds["new"] + kinds["deleted"] + 2*(kinds["renamed"]+kinds["renamed-approx"])
	if covered != len(names) {
		t.Errorf("the kinds %v cover %d names, want the %d names across both revisions", kinds, covered, len(names))
	}
	for name, status := range on.GetNetStatus() {
		if status == string(diff.NetEqual) {
			t.Errorf("net_status lists %s as equal, which would tint every unchanged net", name)
		}
	}
}
