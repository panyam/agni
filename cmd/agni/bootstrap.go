package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/panyam/agni/service"
)

// readBootstrap reads and checks the --bootstrap file, so a typo stops the server before it listens.
func readBootstrap(path string) (*service.BootstrapPlan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("--bootstrap: %w", err)
	}
	plan, err := service.ParseBootstrap(b)
	if err != nil {
		return nil, fmt.Errorf("--bootstrap %s: %w", path, err)
	}
	return plan, nil
}

// Bootstrap retries a failed entry after bootstrapFirstWait, doubling up to bootstrapMaxWait. A
// first install fails until its connection's App is installed, so the retry has to outlast that.
var (
	bootstrapFirstWait = 5 * time.Second
	bootstrapMaxWait   = 5 * time.Minute
)

// bootstrapper applies the --bootstrap plan until every entry applies, and answers /readyz
// meanwhile (agni issue 1808).
type bootstrapper struct {
	b    service.Bootstrap
	plan *service.BootstrapPlan
	// lock is the follower's, so a push or poll never moves a design while bootstrap links it.
	lock *sync.Mutex
	log  io.Writer

	mu    sync.Mutex
	ready bool
	last  *service.BootstrapResult
}

// run applies the plan, and again after a wait while any entry fails, until life ends.
func (s *bootstrapper) run(life context.Context) {
	wait := bootstrapFirstWait
	for {
		s.lock.Lock()
		r := s.b.Apply(life, s.plan)
		s.lock.Unlock()
		s.report(r)
		s.mu.Lock()
		s.last, s.ready = r, r.OK()
		s.mu.Unlock()
		if r.OK() {
			return
		}
		select {
		case <-life.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, bootstrapMaxWait)
	}
}

// report logs what one pass did.
func (s *bootstrapper) report(r *service.BootstrapResult) {
	if len(r.Applied) > 0 {
		fmt.Fprintf(s.log, "bootstrap: applied %s\n", strings.Join(r.Applied, ", "))
	}
	for _, f := range r.Failed {
		fmt.Fprintf(s.log, "bootstrap: %s failed: %v\n", f.Name, f.Err)
	}
	if len(r.Pruned) > 0 {
		fmt.Fprintf(s.log, "bootstrap: pruned %s, which bootstrap.yaml doesn't name\n", strings.Join(r.Pruned, ", "))
	}
	if kept := unpruned(r); len(kept) > 0 {
		fmt.Fprintf(s.log, "bootstrap: %s %s in the store but not in bootstrap.yaml; set prune: true to delete %s\n",
			strings.Join(kept, ", "), plural(len(kept), "is", "are"), plural(len(kept), "it", "them"))
	}
	if r.OK() {
		fmt.Fprintf(s.log, "bootstrap: ready, %d applied and %d unchanged\n", len(r.Applied), len(r.Unchanged))
	}
}

// unpruned is r's unnamed resources that it didn't delete.
func unpruned(r *service.BootstrapResult) []string {
	var out []string
	for _, n := range r.Unnamed {
		pruned := false
		for _, p := range r.Pruned {
			pruned = pruned || p == n
		}
		if !pruned {
			out = append(out, n)
		}
	}
	return out
}

// readyHandler answers the orchestrator's readiness probe. A server with no --bootstrap is ready
// once it serves. One with it is ready once every entry applied, and until then it answers 503
// naming each entry that failed.
func (s *bootstrapper) readyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if s == nil {
			io.WriteString(w, "ok\n")
			return
		}
		s.mu.Lock()
		ready, last := s.ready, s.last
		s.mu.Unlock()
		if ready {
			io.WriteString(w, "ok\n")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		if last == nil {
			io.WriteString(w, "bootstrap: applying\n")
			return
		}
		for _, f := range last.Failed {
			fmt.Fprintf(w, "bootstrap: %s failed: %v\n", f.Name, f.Err)
		}
	})
}
