package main

import (
	"context"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/panyam/agni/service"
)

// A server that stops while a move is still running waits for it before it returns, so the move
// never writes to a store its host has started to remove (agni issue 1867).
func TestServeWaitsForAMoveStillRunning(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	followers := make(chan *follower, 1)
	o := viewerOpts{
		listener: ln, store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"),
		follow: func(f *follower) { followers <- f },
	}
	cmd := rootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() { done <- runViewer(cmd, o) }()
	var f *follower
	select {
	case f = <-followers:
	case err := <-done:
		t.Fatalf("runViewer stopped before it made its follower: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("runViewer made no follower in 10s")
	}

	started, release := make(chan struct{}), make(chan struct{})
	f.run("test", func(context.Context) ([]service.SourceMove, error) {
		close(started)
		<-release
		return nil, nil
	})
	<-started
	ln.Close()
	select {
	case err := <-done:
		close(release)
		t.Fatalf("runViewer returned (%v) while a move was still running; want it to wait for the move", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runViewer didn't return within 10s of the move finishing")
	}
}
