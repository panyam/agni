package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/mountset"
)

// boardsWithABrokenOne is twoBoards plus boards/broken, whose design.yaml names a symbol folder
// that isn't there, so the server refuses to add it.
func boardsWithABrokenOne(t *testing.T) string {
	t.Helper()
	lab := twoBoards(t)
	b, err := os.ReadFile(gatewayRevA)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(lab, "boards", "broken")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"gateway.edn": b,
		"design.yaml": []byte("name: broken\nentry: gateway.edn\nsymbols: [nowhere]\n"),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return lab
}

// outcomes maps each result's folder to its outcome, with the reason after a refusal.
func outcomes(resp *webapi.AddDesignsResponse) map[string]string {
	out := map[string]string{}
	for _, r := range resp.GetResults() {
		o := strings.TrimPrefix(r.GetOutcome().String(), "ADD_OUTCOME_")
		if r.GetReason() != "" {
			o += ": " + r.GetReason()
		}
		out[r.GetFolder()] = o
	}
	return out
}

func allThree() []*webapi.DesignToAdd {
	return []*webapi.DesignToAdd{{Folder: "boards/broken"}, {Folder: "boards/cut"}, {Folder: "boards/gw"}}
}

// With allow_partial, AddDesigns keeps every proposal it can and reports the refused one, where
// without it one refused proposal keeps nothing (agni issue 1860).
func TestAddDesignsKeepsWhatItCanWithAllowPartial(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", boardsWithABrokenOne(t)))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)

	if _, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://lab/boards", Designs: allThree()})); err == nil || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("AddDesigns over the broken board = %v; want it refused, naming its missing symbol folder", err)
	}
	if list, _ := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{})); len(list.Msg.GetDesigns()) != 0 {
		t.Fatalf("a refused AddDesigns kept %d designs; want none", len(list.Msg.GetDesigns()))
	}

	resp, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://lab/boards", Designs: allThree(), AllowPartial: true}))
	if err != nil {
		t.Fatal(err)
	}
	got := outcomes(resp.Msg)
	if got["boards/cut"] != "ADDED" || got["boards/gw"] != "ADDED" || !strings.HasPrefix(got["boards/broken"], "REFUSED: ") || !strings.Contains(got["boards/broken"], "nowhere") {
		t.Errorf("outcomes = %v; want cut and gw added, broken refused naming its missing symbol folder", got)
	}
	if len(resp.Msg.GetDesigns()) != 2 {
		t.Errorf("designs = %d; want the two kept", len(resp.Msg.GetDesigns()))
	}
}

// A proposal whose design exists fails under ON_EXISTS_FAIL, is left alone under ON_EXISTS_SKIP,
// and under ON_EXISTS_UPDATE is left unchanged until its files change, then rewritten (agni issue
// 1860).
func TestAddDesignsOnAnExistingDesign(t *testing.T) {
	ctx := context.Background()
	lab := twoBoards(t)
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	add := func(on webapi.OnExists) (map[string]string, error) {
		t.Helper()
		resp, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{
			Uri: "mount://lab/boards", Designs: []*webapi.DesignToAdd{{Folder: "boards/gw"}}, OnExists: on, AllowPartial: true,
		}))
		if err != nil {
			return nil, err
		}
		return outcomes(resp.Msg), nil
	}
	if got, err := add(webapi.OnExists_ON_EXISTS_UNSPECIFIED); err != nil || got["boards/gw"] != "ADDED" {
		t.Fatalf("the first add = %v, %v; want gw added", got, err)
	}
	head := func() string {
		t.Helper()
		d, err := designs.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Name: "designs/gw"}))
		if err != nil {
			t.Fatal(err)
		}
		return d.Msg.GetEntryUri()
	}
	before := head()

	if got, err := add(webapi.OnExists_ON_EXISTS_FAIL); err != nil || !strings.HasPrefix(got["boards/gw"], "REFUSED: ") || !strings.Contains(got["boards/gw"], "already exists") {
		t.Errorf("ON_EXISTS_FAIL = %v, %v; want gw refused as already existing", got, err)
	}
	if got, err := add(webapi.OnExists_ON_EXISTS_SKIP); err != nil || got["boards/gw"] != "SKIPPED" {
		t.Errorf("ON_EXISTS_SKIP = %v, %v; want gw skipped", got, err)
	}
	if got, err := add(webapi.OnExists_ON_EXISTS_UPDATE); err != nil || got["boards/gw"] != "UNCHANGED" || head() != before {
		t.Errorf("ON_EXISTS_UPDATE over the same files = %v, %v, entry %s; want gw unchanged at %s", got, err, head(), before)
	}

	edn := filepath.Join(lab, "boards", "gw", "gateway.edn")
	b, err := os.ReadFile(edn)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(edn, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := add(webapi.OnExists_ON_EXISTS_UPDATE); err != nil || got["boards/gw"] != "UPDATED" || head() == before {
		t.Errorf("ON_EXISTS_UPDATE over a changed file = %v, %v, entry %s; want gw updated off %s", got, err, head(), before)
	}
}

// `designs add --only` keeps only the boards it names even with --yes, so a refused board elsewhere
// in the folder doesn't matter. --yes alone keeps every board that validates, names the refused one
// and fails, and a second run with --skip-existing succeeds (agni issue 1860).
func TestDesignsAddKeepsWhatValidatesAndRunsAgain(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", boardsWithABrokenOne(t)))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	kept := func() int {
		t.Helper()
		list, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		return len(list.Msg.GetDesigns())
	}

	if out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--yes", "--only", "boards/cut"); err != nil || kept() != 1 {
		t.Fatalf("designs add --yes --only boards/cut = %q, %v, %d designs; want cut alone kept", out, err, kept())
	}
	out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--yes")
	if err == nil || !strings.Contains(out, "refused\tboards/broken") || !strings.Contains(out, "nowhere") || kept() != 2 {
		t.Fatalf("designs add --yes = %q, %v, %d designs; want gw kept, broken refused naming its symbol folder, and a failed exit", out, err, kept())
	}
	if !strings.Contains(out, "refused\tboards/cut") || !strings.Contains(out, "already exists") {
		t.Errorf("designs add --yes over a kept cut = %q; want cut refused as already existing", out)
	}
	out, err = runDesigns(t, srv.URL, "add", "lab/boards", "--only", "boards/cut", "--only", "boards/gw", "--skip-existing")
	if err != nil || !strings.Contains(out, "skipped\tboards/cut") || !strings.Contains(out, "skipped\tboards/gw") {
		t.Errorf("designs add --skip-existing of kept boards = %q, %v; want both skipped and a clean exit", out, err)
	}
	if out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--yes", "--skip-existing", "--update"); err == nil || !strings.Contains(err.Error(), "--skip-existing") {
		t.Errorf("designs add --skip-existing --update = %q, %v; want them refused together", out, err)
	}
}
