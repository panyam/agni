package projects

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/service"
)

// A malformed descriptor is now fatal for the design it governs, so these pin the boundary of that:
// which failures are the design's own, and which belong to someone else's folder.

// TestResolveDesign_MalformedOwnDescriptor: a design whose OWN design.yaml does not parse returns the
// parse error. Callers use this to refuse rather than compose against the built-in vocabulary, which
// answers a different question while looking like an answer.
func TestResolveDesign_MalformedOwnDescriptor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "proj/project.yaml", "name: gw\n")
	writeFile(t, root, "proj/designs/board/design.yaml", "name: My Board\nentry: board.edn\n") // spaces: not an id
	writeFile(t, root, "proj/designs/board/board.edn", "x")
	s := NewFSStore(Tree{Mount: "m", FS: os.DirFS(root)})

	_, _, err := s.ResolveDesign(context.Background(), artifact.URI{Mount: "m", Path: "proj/designs/board/board.edn"})
	if err == nil {
		t.Fatal("a design.yaml that does not parse resolved cleanly; the run would silently use the built-in config")
	}
	if !strings.Contains(err.Error(), "not a valid id") {
		t.Errorf("error %q does not say what is wrong with the descriptor", err)
	}
}

// TestResolveDesign_MalformedSiblingIsNotOurProblem is the regression guard for the narrowing. The
// tolerance being narrowed exists for a real reason — one team's broken folder must not make another
// team's design unreadable — and only the descriptors on THIS design's path are its own.
func TestResolveDesign_MalformedSiblingIsNotOurProblem(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "proj/project.yaml", "name: gw\n")
	writeFile(t, root, "proj/designs/good/design.yaml", "name: good\nentry: good.edn\n")
	writeFile(t, root, "proj/designs/good/good.edn", "x")
	writeFile(t, root, "proj/designs/broken/design.yaml", "name: Not An Id\nentry: broken.edn\n")
	s := NewFSStore(Tree{Mount: "m", FS: os.DirFS(root)})

	d, p, err := s.ResolveDesign(context.Background(), artifact.URI{Mount: "m", Path: "proj/designs/good/good.edn"})
	if err != nil {
		t.Fatalf("a broken descriptor in a SIBLING folder made this design unresolvable: %v", err)
	}
	if d == nil || p == nil {
		t.Fatalf("design/project = %v/%v, want both resolved", d, p)
	}
}

// TestResolveDesign_UnknownMountIsNotFound: an unresolvable mount is ErrNotFound, which callers treat
// as "no project" rather than as a broken descriptor. Keeping these distinguishable is what lets
// Overlay refuse one and tolerate the other.
func TestResolveDesign_UnknownMountIsNotFound(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: os.DirFS(t.TempDir())})
	_, _, err := s.ResolveDesign(context.Background(), artifact.URI{Mount: "other", Path: "x.edn"})
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("unknown mount gave %v, want ErrNotFound so callers can tell it from a parse failure", err)
	}
}
