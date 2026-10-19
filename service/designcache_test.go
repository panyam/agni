package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestAWaiterBuildsAgainWhenTheBuildersCallerGoesAway: a request waiting on another's read gets the
// read, not the other request's cancellation (agni issue 1061). A background LoadDesign the page
// abandons must not fail the check that was waiting on it.
func TestAWaiterBuildsAgainWhenTheBuildersCallerGoesAway(t *testing.T) {
	c := NewDesignCache(8)
	started := make(chan struct{})
	var builds atomic.Int32
	build := func(ctx context.Context) (any, error) {
		if builds.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return "read", nil
	}
	first, cancel := context.WithCancel(context.Background())
	firstErr := make(chan error, 1)
	go func() {
		_, err := c.get(first, "board\x00k", build)
		firstErr <- err
	}()
	<-started
	waited := make(chan struct{})
	var got any
	var err error
	go func() {
		got, err = c.get(context.Background(), "board\x00k", build)
		close(waited)
	}()
	time.Sleep(50 * time.Millisecond) // the waiter is parked on the first build
	cancel()
	if e := <-firstErr; !errors.Is(e, context.Canceled) {
		t.Fatalf("the cancelled builder got %v, want context.Canceled", e)
	}
	<-waited
	if err != nil || got != "read" {
		t.Errorf("the waiter got %v, %v; want the read, built again", got, err)
	}
	if n := builds.Load(); n != 2 {
		t.Errorf("built %d times, want the cancelled build and the waiter's own", n)
	}
}

// TestAWaiterGetsAFailedReadsError: a read that fails for itself is not run twice for one request.
func TestAWaiterGetsAFailedReadsError(t *testing.T) {
	c := NewDesignCache(8)
	bad := errors.New("unreadable")
	release := make(chan struct{})
	var builds atomic.Int32
	build := func(ctx context.Context) (any, error) {
		builds.Add(1)
		<-release
		return nil, bad
	}
	firstErr := make(chan error, 1)
	go func() {
		_, err := c.get(context.Background(), "board\x00k", build)
		firstErr <- err
	}()
	for builds.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	waiterErr := make(chan error, 1)
	go func() {
		_, err := c.get(context.Background(), "board\x00k", build)
		waiterErr <- err
	}()
	time.Sleep(50 * time.Millisecond)
	close(release)
	if e := <-firstErr; !errors.Is(e, bad) {
		t.Fatalf("the builder got %v, want its own error", e)
	}
	if e := <-waiterErr; !errors.Is(e, bad) {
		t.Errorf("the waiter got %v, want the failed read's error", e)
	}
	if n := builds.Load(); n != 1 {
		t.Errorf("built %d times, want one", n)
	}
}
