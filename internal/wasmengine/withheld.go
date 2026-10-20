package wasmengine

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"

	"connectrpc.com/connect"

	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/service"
)

// The page holds a design's board file back from its serve worker until a question there reads the
// board (agni issue 992), since a query, a coverage or a trace builds no board and the bytes are most
// of what that worker would hold. It puts a placeholder at the file's path instead, so the folder
// lists, stats and resolves exactly as it does with the file in place.
//
// Opening a placeholder fails, and an engine that has opened one answers every request from then on
// as FilesWithheldError, whatever its service made of the failed read. A reader that treats an
// unreadable sibling as an absent one would otherwise answer with less than the design, and a cache
// would keep that answer for the next question. The page sends the files and asks again, and the
// files arriving rebuild the engine, which starts with nothing read around a placeholder.

// withheldPrefix starts every placeholder. Its first byte is NUL, which no design format's text
// starts with. testdata/withheld_twin.json holds it to the page's copy in web/src/wasm/client.ts.
const withheldPrefix = "\x00agni withheld\n"

// maxPlaceholder bounds the files Open looks inside: a prefix and a "sha256:<hex>" line.
const maxPlaceholder = 256

// WithheldPlaceholder is the placeholder for a held-back file whose content hash is sha256, as
// ListDesignFiles spells it.
func WithheldPlaceholder(sha256 string) []byte {
	return []byte(withheldPrefix + sha256 + "\n")
}

var errWithheld = errors.New("held back from this engine")

// withholding records the held-back files an engine has opened, by content hash.
type withholding struct {
	mu   sync.Mutex
	read map[string]bool
}

func (w *withholding) note(hash string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.read == nil {
		w.read = map[string]bool{}
	}
	w.read[hash] = true
}

// opened is the hashes of every held-back file opened so far, sorted, and nil when there are none.
func (w *withholding) opened() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for h := range w.read {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// files wraps a mount so that opening a placeholder fails and is recorded. Stat and ReadDir see the
// placeholder as the file it stands for.
func (w *withholding) files(fsys fs.FS) fs.FS { return withholdingFS{fsys, w} }

type withholdingFS struct {
	fs.FS
	w *withholding
}

func (f withholdingFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := file.Stat()
	if err != nil || st.IsDir() || st.Size() < int64(len(withheldPrefix)) || st.Size() > maxPlaceholder {
		return file, nil
	}
	// A small file is read to tell, so it is opened again for the caller.
	b, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		return nil, err
	}
	if rest, ok := bytes.CutPrefix(b, []byte(withheldPrefix)); ok {
		f.w.note(strings.TrimSpace(string(rest)))
		return nil, &fs.PathError{Op: "open", Path: name, Err: errWithheld}
	}
	return f.FS.Open(name)
}

func (f withholdingFS) Stat(name string) (fs.FileInfo, error) { return fs.Stat(f.FS, name) }

func (f withholdingFS) ReadDir(name string) ([]fs.DirEntry, error) { return fs.ReadDir(f.FS, name) }

// guard answers a request as FilesWithheldError once the engine has opened a held-back file. The
// decision is taken when the handler first writes, which a Connect handler does only once it has its
// answer, so nothing it wrote reaches the page.
func (w *withholding) guard(h http.Handler) http.Handler { return &guarded{next: h, w: w} }

type guarded struct {
	next http.Handler
	w    *withholding
}

func (h *guarded) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	g := &guardedWriter{rw: rw, r: r, w: h.w, header: http.Header{}}
	h.next.ServeHTTP(g, r)
	if !g.decided {
		g.decide(http.StatusOK)
	}
}

type guardedWriter struct {
	rw     http.ResponseWriter
	r      *http.Request
	w      *withholding
	header http.Header
	// decided is set once the response's status is chosen, and replaced when it is the error.
	decided, replaced bool
}

func (g *guardedWriter) Header() http.Header { return g.header }

func (g *guardedWriter) WriteHeader(code int) {
	if !g.decided {
		g.decide(code)
	}
}

func (g *guardedWriter) Write(b []byte) (int, error) {
	if !g.decided {
		g.decide(http.StatusOK)
	}
	if g.replaced {
		return len(b), nil
	}
	return g.rw.Write(b)
}

func (g *guardedWriter) decide(code int) {
	g.decided = true
	if hashes := g.w.opened(); len(hashes) > 0 {
		g.replaced = true
		_ = connect.NewErrorWriter().Write(g.rw, g.r, server.ConnectError(&service.FilesWithheldError{Hashes: hashes}))
		return
	}
	for k, v := range g.header {
		g.rw.Header()[k] = v
	}
	g.rw.WriteHeader(code)
}
