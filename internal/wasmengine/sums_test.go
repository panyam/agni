package wasmengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/goapplib/wasmhost"
)

// The tests here cover a worker checking a stored read by the hashes the page listed (agni issue
// 1515), so a board held back as a placeholder restores from the store without its bytes.

// withSums adds the page's manifest of every file's listed hash and size.
func withSums(files map[string][]byte) {
	m := sumsManifest{Files: map[string]listedSum{}}
	for p, b := range files {
		sum := sha256.Sum256(b)
		m.Files[p] = listedSum{SHA256: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(b))}
	}
	b, _ := json.Marshal(m)
	files[SumsFile] = b
}

// holdBoard replaces the board with its placeholder, keeping the manifest's row for it.
func holdBoard(files map[string][]byte) {
	sum := sha256.Sum256(files[gatewayBoard])
	files[gatewayBoard] = WithheldPlaceholder("sha256:" + hex.EncodeToString(sum[:]))
}

func boardSheet(srv *httptest.Server, snap string) (*webapi.GetSheetResponse, error) {
	r, err := webapiconnect.NewDesignServiceClient(srv.Client(), srv.URL).GetSheet(context.Background(), connect.NewRequest(&webapi.GetSheetRequest{Snapshot: snap, Sheet: "board"}))
	if err != nil {
		return nil, err
	}
	return r.Msg, nil
}

// TestAHeldBackBoardRestoresByItsListedHash runs the page's shape. The prepare worker reads the
// design with its board and stores it. The serve worker holds the board as a placeholder and draws
// the Board sheet from the store. As a control, the same serve worker without the manifest is refused
// for the board's bytes, as it was before.
func TestAHeldBackBoardRestoresByItsListedHash(t *testing.T) {
	store := &wasmhost.MemCache{}
	loadDesign(t, gatewayEngine(t, store, "v1", withSums))

	serve := httptest.NewServer(gatewayEngine(t, store, "v1", func(f map[string][]byte) { withSums(f); holdBoard(f) }).Handler)
	defer serve.Close()
	r, err := boardSheet(serve, snapAt(t, serve))
	if err != nil {
		t.Fatalf("the Board sheet with the board held back: %v", err)
	}
	if len(r.GetPacked().GetPrimitives()) == 0 {
		t.Fatal("the Board sheet drew nothing")
	}

	var hash string
	control := httptest.NewServer(gatewayEngine(t, store, "v1", func(f map[string][]byte) {
		sum := sha256.Sum256(f[gatewayBoard])
		hash = "sha256:" + hex.EncodeToString(sum[:])
		holdBoard(f)
	}).Handler)
	defer control.Close()
	_, err = boardSheet(control, snapAt(t, control))
	wantWithheld(t, "the Board sheet with no manifest", err, hash)
}

// TestTheManifestAnswersOnlyWhereItAgreesWithTheFile checks when a mount trusts its manifest. A real
// file of the listed size and a placeholder naming the listed hash are summed by it. A file of
// another size, a placeholder naming another hash and an unlisted file are not, so they are read and
// summed as before.
func TestTheManifestAnswersOnlyWhereItAgreesWithTheFile(t *testing.T) {
	const listed = "sha256:aa"
	files := map[string][]byte{
		"same.txt":   []byte("four"),
		"grown.txt":  []byte("eleven bytes"),
		"held.pcb":   WithheldPlaceholder(listed),
		"other.pcb":  WithheldPlaceholder("sha256:bb"),
		"unlisted.x": []byte("x"),
	}
	m := sumsManifest{Files: map[string]listedSum{
		"same.txt":  {SHA256: listed, Size: 4},
		"grown.txt": {SHA256: listed, Size: 4},
		"held.pcb":  {SHA256: listed, Size: 85},
		"other.pcb": {SHA256: listed, Size: 85},
	}}
	b, _ := json.Marshal(m)
	files[SumsFile] = b
	raw := fshost.MemFS(files)
	w := &withholding{}
	fsys := summed(w.files(raw), raw).(summedFS)
	for name, want := range map[string]bool{"same.txt": true, "grown.txt": false, "held.pcb": true, "other.pcb": false, "unlisted.x": false} {
		sum, size, ok := fsys.ContentSum(name)
		if ok != want {
			t.Errorf("%s: answered %v, want %v", name, ok, want)
		}
		if ok && (sum != listed || size != m.Files[name].Size) {
			t.Errorf("%s: answered %s %d, want the listed %s %d", name, sum, size, listed, m.Files[name].Size)
		}
	}
	if got := w.opened(); len(got) != 0 {
		t.Errorf("checking the sums opened placeholders %v through the withholding layer", got)
	}
}

func TestThePageAndTheEngineReadTheSumsAlike(t *testing.T) {
	b, err := os.ReadFile("testdata/sums_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct {
		Listing []struct {
			Path, Sha256 string
			Size         int64
		}
		Manifest string
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	var m sumsManifest
	if err := json.Unmarshal([]byte(twin.Manifest), &m); err != nil {
		t.Fatalf("the page's manifest does not parse: %v", err)
	}
	if len(m.Files) != len(twin.Listing) {
		t.Fatalf("the manifest has %d rows for a listing of %d", len(m.Files), len(twin.Listing))
	}
	for _, f := range twin.Listing {
		if got := m.Files[f.Path]; got.SHA256 != f.Sha256 || got.Size != f.Size {
			t.Errorf("%s reads as %+v, listed %s %d", f.Path, got, f.Sha256, f.Size)
		}
	}
}
