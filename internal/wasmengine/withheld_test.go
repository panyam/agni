package wasmengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// The tests here cover a board the page held back from a worker (agni issue 992): a question that
// reads no board is answered without it, and every question that reads it is refused with the
// file's hash, never answered from a design missing its copper.

const gatewayBoard = "gateway.kicad_pcb"

// heldBackGateway serves the gateway with its board replaced by a placeholder, and returns the
// board's hash, which the placeholder names.
func heldBackGateway(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	var hash string
	eng := gatewayEngine(t, nil, "", func(files map[string][]byte) {
		sum := sha256.Sum256(files[gatewayBoard])
		hash = "sha256:" + hex.EncodeToString(sum[:])
		files[gatewayBoard] = WithheldPlaceholder(hash)
	})
	srv := httptest.NewServer(eng.Handler)
	t.Cleanup(srv.Close)
	return srv, hash
}

func runQuery(srv *httptest.Server, snap, q string) (*webapi.RunQueryResponse, error) {
	r, err := webapiconnect.NewQueryServiceClient(srv.Client(), srv.URL).RunQuery(context.Background(), connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snap, Query: q}))
	if err != nil {
		return nil, err
	}
	return r.Msg, nil
}

// wantWithheld fails unless err is the refusal naming hash.
func wantWithheld(t *testing.T, what string, err error, hash string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("%s: got %v, want failed_precondition naming the held-back board", what, err)
	}
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if bm, ok := v.(*webapi.BlobsMissing); derr == nil && ok {
			if !slices.Equal(bm.GetSha256(), []string{hash}) {
				t.Fatalf("%s: names %v, want [%s]", what, bm.GetSha256(), hash)
			}
			return
		}
	}
	t.Fatalf("%s: %v carries no BlobsMissing detail", what, err)
}

func TestThePageAndTheEngineSpellAPlaceholderAlike(t *testing.T) {
	b, err := os.ReadFile("testdata/withheld_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct{ Sha256, Placeholder string }
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if got := string(WithheldPlaceholder(twin.Sha256)); got != twin.Placeholder {
		t.Errorf("placeholder %q, the page writes %q", got, twin.Placeholder)
	}
}

// TestAPlaceholderStatsAsItsFileAndRefusesToOpen is the mount as the engine sees it: the placeholder
// lists and stats where the file is, so the design resolves as it would with the file in place, and
// opening it fails and is recorded. control: a file that is not a placeholder, small enough to be
// looked inside, opens with its bytes intact.
func TestAPlaceholderStatsAsItsFileAndRefusesToOpen(t *testing.T) {
	const hash = "sha256:abc"
	w := &withholding{}
	fsys := w.files(fshost.MemFS(map[string][]byte{"d/b.kicad_pcb": WithheldPlaceholder(hash), "d/design.yaml": []byte("entry: a.kicad_sch\n")}))
	if st, err := fs.Stat(fsys, "d/b.kicad_pcb"); err != nil || st.IsDir() {
		t.Fatalf("stat: %v %v", st, err)
	}
	if des, err := fs.ReadDir(fsys, "d"); err != nil || len(des) != 2 {
		t.Fatalf("readdir: %v %v", des, err)
	}
	if _, err := fs.ReadFile(fsys, "d/b.kicad_pcb"); !errors.Is(err, errWithheld) {
		t.Fatalf("reading the placeholder: %v, want it held back", err)
	}
	if got := w.opened(); !slices.Equal(got, []string{hash}) {
		t.Fatalf("opened %v, want [%s]", got, hash)
	}
	if b, err := fs.ReadFile(fsys, "d/design.yaml"); err != nil || string(b) != "entry: a.kicad_sch\n" {
		t.Fatalf("a small file read back as %q, %v", b, err)
	}
}

func TestAQuestionThatReadsTheBoardAsksForIt(t *testing.T) {
	const boardQuery = "board.layer(?n, ?l) => ?n, ?l"
	// control: with the board in place the query answers from it, so a refusal below is the board
	// being read rather than a query that reads nothing.
	full := httptest.NewServer(gatewayEngine(t, nil, "", nil).Handler)
	defer full.Close()
	if r, err := runQuery(full, snapAt(t, full), boardQuery); err != nil || len(r.GetRows()) == 0 {
		t.Fatalf("with the board in place: %v, %d rows", err, len(r.GetRows()))
	}

	asks := map[string]func(srv *httptest.Server, snap string) error{
		"a board query": func(srv *httptest.Server, snap string) error {
			_, err := runQuery(srv, snap, boardQuery)
			return err
		},
		"the Board sheet": func(srv *httptest.Server, snap string) error {
			_, err := webapiconnect.NewDesignServiceClient(srv.Client(), srv.URL).GetSheet(context.Background(), connect.NewRequest(&webapi.GetSheetRequest{Snapshot: snap, Sheet: "board"}))
			return err
		},
		"a checks run": func(srv *httptest.Server, snap string) error {
			_, err := webapiconnect.NewCheckServiceClient(srv.Client(), srv.URL).CheckDesign(context.Background(), connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snap}))
			return err
		},
	}
	for what, ask := range asks {
		t.Run(what, func(t *testing.T) {
			srv, hash := heldBackGateway(t)
			snap := snapAt(t, srv)
			// The open and a netlist query read no board, so they are answered with it held back, and
			// the design still offers its Board sheet.
			d, err := webapiconnect.NewDesignServiceClient(srv.Client(), srv.URL).GetDesign(context.Background(), connect.NewRequest(&webapi.GetDesignRequest{Snapshot: snap}))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(d.Msg.GetSheets(), func(s *webapi.SheetRef) bool { return s.GetId() == "board" }) {
				t.Errorf("the design lost its Board sheet: %v", d.Msg.GetSheets())
			}
			if r, err := runQuery(srv, snap, "net.rail(?n) => ?n"); err != nil || len(r.GetRows()) == 0 {
				t.Fatalf("a netlist query with the board held back: %v", err)
			}
			wantWithheld(t, what, ask(srv, snap), hash)
			// Whatever the engine kept from a read around the placeholder is not trusted either, so a
			// question that reads no board is refused too until the files arrive.
			_, err = runQuery(srv, snap, "net.rail(?n) => ?n")
			wantWithheld(t, "a netlist query after "+what, err, hash)
		})
	}
}
