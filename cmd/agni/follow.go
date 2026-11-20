package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/github"
	"github.com/panyam/oneauth/outbound"

	"github.com/panyam/agni/service"
)

// githubHookPath is where GitHub sends a repo's push webhook (agni issue 1726). The auth gate lets
// it through without a token, since the delivery's signature proves it came from GitHub.
const githubHookPath = "/hooks/github"

// follower moves the designs that follow a branch or a folder when the source moves, from a push
// webhook or a poll (agni issue 1726). It runs one move at a time, so a push and a poll never sync
// one design at once.
type follower struct {
	designs *service.DesignService
	// life ends with the server. A move runs on it rather than on the request, since GitHub hangs up
	// after ten seconds and a sync of a large board takes longer.
	life context.Context
	log  io.Writer
	mu   sync.Mutex
	// wg counts the moves still running, for a test to wait on.
	wg sync.WaitGroup
}

// secretFlag reads the secret flag names, given as a secret reference such as env:NAME. An empty
// secret is an error.
func secretFlag(flag, ref string) ([]byte, error) {
	r, err := outbound.ParseSecretRef(ref)
	if err != nil {
		return nil, fmt.Errorf("%s %q: want env:NAME or file:/abs/path: %w", flag, ref, err)
	}
	s, err := r.Resolve()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", flag, err)
	}
	if s = strings.TrimSpace(s); s == "" {
		return nil, fmt.Errorf("%s %s is empty", flag, ref)
	}
	return []byte(s), nil
}

// githubHook answers a GitHub webhook delivery. A bad signature is 401, a ping or another event 200,
// and a push 202, after which the designs following the pushed ref move in the background.
func (f *follower) githubHook(secret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "a webhook delivery is a POST", http.StatusMethodNotAllowed)
			return
		}
		push, err := github.ParsePush(r, secret)
		switch {
		case errors.Is(err, github.ErrBadSignature):
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		case errors.Is(err, github.ErrPing), errors.Is(err, github.ErrNotPush):
			w.WriteHeader(http.StatusOK)
			return
		case errors.Is(err, mounts.ErrTooLarge):
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.run(fmt.Sprintf("push to %s@%s", push.Source, push.Ref), func(ctx context.Context) ([]service.SourceMove, error) {
			return f.designs.SourceChanged(ctx, push.Change)
		})
		w.WriteHeader(http.StatusAccepted)
	})
}

// poll runs PollSources every interval until the server stops.
func (f *follower) poll(every time.Duration) {
	t := time.NewTicker(every)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-f.life.Done():
				return
			case <-t.C:
				f.mu.Lock()
				moves, err := f.designs.PollSources(f.life)
				f.mu.Unlock()
				f.report("poll", moves, err)
			}
		}
	}()
}

// run starts move in the background, after any move still running.
func (f *follower) run(what string, move func(context.Context) ([]service.SourceMove, error)) {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		f.mu.Lock()
		moves, err := move(f.life)
		f.mu.Unlock()
		f.report(what, moves, err)
	}()
}

// report logs one line per design a move touched, and the error that stopped it.
func (f *follower) report(what string, moves []service.SourceMove, err error) {
	for _, m := range moves {
		switch {
		case m.Synced:
			fmt.Fprintf(f.log, "%s: synced %s (%s)\n", what, m.Design, strings.Join(m.Artifacts, ", "))
		case m.Err != nil:
			fmt.Fprintf(f.log, "%s: marked %s (%s) newer at the source, since the sync failed: %v\n", what, m.Design, strings.Join(m.Artifacts, ", "), m.Err)
		default:
			fmt.Fprintf(f.log, "%s: marked %s (%s) newer at the source\n", what, m.Design, strings.Join(m.Artifacts, ", "))
		}
	}
	if err != nil {
		fmt.Fprintf(f.log, "%s: %v\n", what, err)
	}
}
