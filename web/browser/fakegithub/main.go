// Command fakegithub serves goapplib's fake GitHub for the browser walk that links a design from a
// repo (agni issue 1727). The browser suite can't reach GitHub, so the walk's agni serve reads this
// instead, through --github-api and --github-raw.
//
// Each --commit tag=dir commits dir's files to the repo's main branch, replacing the files before,
// and tags the commit. So two folders of one board become two tagged revisions, as a repo's history
// would hold them. --addr serves a small JSON answer once the fake is up, naming the fake's API
// address, its raw-file address, and each tag's commit:
//
//	{"api": "http://127.0.0.1:N", "raw": "http://127.0.0.1:N/raw", "commits": {"1.1.2": "<sha>"}}
//
// It is a test tool, built and started by web/browser/linkdesign.spec.ts, and never shipped.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// commits is the repeated --commit flag, in the order given.
type commits []string

func (c *commits) String() string     { return strings.Join(*c, ",") }
func (c *commits) Set(v string) error { *c = append(*c, v); return nil }

// process stands in for the testing.TB that githubtest.New takes. The fake only registers a
// cleanup, and this process ends without running it.
type process struct{ testing.TB }

func (process) Helper()                   {}
func (process) Cleanup(func())            {}
func (process) Fatalf(f string, a ...any) { log.Fatalf(f, a...) }

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "where to serve the JSON answer naming the fake's addresses")
	repo := flag.String("repo", "acme/board", "the fake repo, owner/name")
	var cs commits
	flag.Var(&cs, "commit", "tag=dir: commit dir's files and tag the commit; repeat in order")
	flag.Parse()
	owner, name, ok := strings.Cut(*repo, "/")
	if !ok || len(cs) == 0 {
		log.Fatal("want --repo owner/name and at least one --commit tag=dir")
	}

	gh := githubtest.New(process{})
	shas := map[string]string{}
	var r *githubtest.Repo
	before := map[string]string{}
	for _, c := range cs {
		tag, dir, ok := strings.Cut(c, "=")
		if !ok {
			log.Fatalf("--commit %q: want tag=dir", c)
		}
		files, err := read(dir)
		if err != nil {
			log.Fatal(err)
		}
		if r == nil {
			r = gh.AddRepo(owner, name, false, files)
		} else {
			var gone []string
			for p := range before {
				if _, kept := files[p]; !kept {
					gone = append(gone, p)
				}
			}
			r.Commit("main", files, gone)
		}
		before = files
		shas[tag] = r.Head("main")
		r.Tag(tag, shas[tag], false)
	}

	answer, err := json.Marshal(map[string]any{"api": gh.URL, "raw": gh.RawURL(), "commits": shas})
	if err != nil {
		log.Fatal(err)
	}
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "fakegithub: %s on %s, answering at http://%s\n", *repo, gh.URL, l.Addr())
	srv := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(answer)
	})}
	log.Fatal(srv.Serve(l))
}

// read returns every file under dir by its slash path relative to dir.
func read(dir string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) // #nosec G304 -- a fixture folder the spec names
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return files, err
}
