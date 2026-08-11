package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/readers/formats"
)

// TestLocalLoaderNotesOncePerPath: one BuildModel asks this loader for the netlist, the board, and
// the geometry of the SAME path, so a note emitted per call would tell the user three times which
// file was read.
func TestLocalLoaderNotesOncePerPath(t *testing.T) {
	dir := designFolder(t)
	pcb := filepath.Join(dir, "gateway.kicad_pcb")

	var notes bytes.Buffer
	l := &localLoader{loader: &formats.Loader{}, notes: &notes}
	ctx := context.Background()

	// Each of these resolves the same path. The reads themselves fail (the fixture files hold
	// throwaway bytes), which is fine: the note is written during resolution, before any parse.
	l.Design(ctx, "", pcb)
	l.Board(ctx, "", pcb)
	l.Geometry(ctx, "", pcb, "", false)

	if got := strings.Count(notes.String(), "companion view"); got != 1 {
		t.Fatalf("note written %d times, want once:\n%s", got, notes.String())
	}
}

// TestLocalLoaderResolvesEachTier pins the split that makes a design's tiers come from different
// files: connectivity from the netlist entry, copper from the declared board companion.
func TestLocalLoaderResolvesEachTier(t *testing.T) {
	dir := designFolder(t)
	l := &localLoader{loader: &formats.Loader{}, notes: &bytes.Buffer{}}

	src, err := l.resolve(context.Background(), filepath.Join(dir, "gateway.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	if src.NetlistRef != filepath.Join(dir, "gateway.edn") {
		t.Errorf("netlist = %q, want the declared entry", src.NetlistRef)
	}
	if src.BoardRef != filepath.Join(dir, "gateway.kicad_pcb") {
		t.Errorf("board = %q, want the named companion", src.BoardRef)
	}
}

// TestLocalLoaderDesignHashFollowsTheEntry: a run recorded against a companion and one recorded
// against the design folder describe the same bytes, so they must record the same revision identity.
func TestLocalLoaderDesignHashFollowsTheEntry(t *testing.T) {
	dir := designFolder(t)
	l := &localLoader{loader: &formats.Loader{}, notes: &bytes.Buffer{}}
	ctx := context.Background()

	viaCompanion, err := l.DesignHash(ctx, "", filepath.Join(dir, "gateway.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	viaEntry, err := l.DesignHash(ctx, "", filepath.Join(dir, "gateway.edn"))
	if err != nil {
		t.Fatal(err)
	}
	if viaCompanion == "" || viaCompanion != viaEntry {
		t.Fatalf("hash via companion = %q, via entry = %q; want the same non-empty hash", viaCompanion, viaEntry)
	}
}
