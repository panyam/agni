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

	"github.com/panyam/agni/mountset"
	"github.com/panyam/goapplib/storage/drivers/msgraph"

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
	// packs, when set, moves the linked packs a push or a poll names too (agni issue 1801).
	packs *service.PackService
	// life ends with the server. A move runs on it rather than on the request, since GitHub hangs up
	// after ten seconds and a sync of a large board takes longer.
	life context.Context
	log  io.Writer
	mu   sync.Mutex
	// wg counts the moves and the poll loop still running. The server waits on it after life ends,
	// so it never exits with a sync half-written to its store (agni issue 1867).
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
		what := fmt.Sprintf("push to %s@%s", push.Source, push.Ref)
		f.run(what, func(ctx context.Context) ([]service.SourceMove, error) {
			if f.packs != nil {
				moves, err := f.packs.SourceChanged(ctx, push.Change)
				f.reportPacks(what, moves, err)
			}
			return f.designs.SourceChanged(ctx, push.Change)
		})
		w.WriteHeader(http.StatusAccepted)
	})
}

// graphHookPath is where Microsoft Graph sends a SharePoint mount's drive notifications (agni issue
// 1729). The auth gate lets it through without a token, since each notification's clientState
// proves it came from a subscription the server made.
const graphHookPath = "/hooks/msgraph"

// graphHook answers Microsoft Graph. A validation request, which Graph sends when a subscription is
// made, gets its token back. A notification gets 202 whatever it holds, as Graph asks, and one whose
// clientState is the server's moves the designs and packs that follow a folder in the changed drive.
func (f *follower) graphHook(clientState []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if msgraph.AnswerValidation(w, r) {
			return
		}
		changes, err := msgraph.ParseNotification(r, string(clientState))
		switch {
		case errors.Is(err, msgraph.ErrBadClientState):
			w.WriteHeader(http.StatusAccepted)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, c := range changes {
			what := "change in " + c.Source
			f.run(what, func(ctx context.Context) ([]service.SourceMove, error) {
				if f.packs != nil {
					moves, err := f.packs.SourceChanged(ctx, c)
					f.reportPacks(what, moves, err)
				}
				return f.designs.SourceChanged(ctx, c)
			})
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

// renewEvery is how often the server looks for Graph subscriptions near their end, and renewWithin
// is how near. Graph keeps a drive subscription about 29 days.
var (
	renewEvery  = 6 * time.Hour
	renewWithin = 3 * 24 * time.Hour
)

// renewSubscriptions renews the SharePoint mounts' Graph subscriptions now and every renewEvery
// until ctx ends (agni issue 1729).
func renewSubscriptions(ctx context.Context, t *mountset.Table, clientState []byte, log io.Writer) {
	renew := func() {
		if n, err := t.RenewSubscriptions(ctx, renewWithin, clientState, log); err != nil {
			fmt.Fprintf(log, "note: renewing Graph subscriptions: %v\n", err)
		} else if n > 0 {
			fmt.Fprintf(log, "renewed %d Graph subscriptions\n", n)
		}
	}
	renew()
	tick := time.NewTicker(renewEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			renew()
		}
	}
}

// poll runs PollSources every interval until the server stops.
func (f *follower) poll(every time.Duration) {
	t := time.NewTicker(every)
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer t.Stop()
		for {
			select {
			case <-f.life.Done():
				return
			case <-t.C:
				f.mu.Lock()
				if f.packs != nil {
					moves, err := f.packs.PollSources(f.life)
					f.reportPacks("poll", moves, err)
				}
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

// reportPacks logs one line per pack a move touched, and the error that stopped it.
func (f *follower) reportPacks(what string, moves []service.PackMove, err error) {
	for _, m := range moves {
		if m.Err != nil {
			fmt.Fprintf(f.log, "%s: could not move %s: %v\n", what, m.Pack, m.Err)
			continue
		}
		fmt.Fprintf(f.log, "%s: moved %s from %s to %s\n", what, m.Pack, shortVersion(m.From), shortVersion(m.To))
	}
	if err != nil {
		fmt.Fprintf(f.log, "%s: %v\n", what, err)
	}
}
