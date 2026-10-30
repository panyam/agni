package main

import (
	"os"
	"path/filepath"
	"testing"
)

// `agni open` on a loose board minted the mount "local" before WS16-003, which is the viewer's mount
// for dropped files, so the Load page read the board as a drop and asked for the browser engine.
func TestAMintedMountNeverTakesTheBrowserMount(t *testing.T) {
	freshWorkspace(t)
	ws, err := workspace()
	if err != nil {
		t.Fatal(err)
	}
	board := filepath.Join(t.TempDir(), "board.kicad_sch")
	if err := os.WriteFile(board, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	u, err := ws.URI(board)
	if err != nil {
		t.Fatal(err)
	}
	if u.Mount != mintedMountName {
		t.Errorf("a loose file minted %q, want %q", u.Mount, mintedMountName)
	}
	// A project whose id is the browser mount's name is suffixed like any other taken name.
	if got := ws.uniqueNameLocked(browserMount, t.TempDir()); got == browserMount {
		t.Errorf("a minted mount was named %q, the browser's own mount", got)
	}
}
