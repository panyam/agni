package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
)

func TestPartSpecSibling(t *testing.T) {
	if got := partSpecSibling("ti/LM1117.pdf"); got != "ti/LM1117.partspec.json" {
		t.Errorf("partSpecSibling = %q, want ti/LM1117.partspec.json", got)
	}
}

func TestOsPartSpecStoreCAS(t *testing.T) {
	dir := t.TempDir()
	st := &osPartSpecStore{mounts: []mounts.Mount{{Name: "m", Root: dir}}}
	ctx := context.Background()

	// Absent: not found, empty version, no error.
	spec, ver, found, err := st.Get(ctx, mustURI("m", "d.pdf"))
	if err != nil || found || spec != nil || ver != "" {
		t.Fatalf("absent => spec=%v ver=%q found=%v err=%v", spec, ver, found, err)
	}

	// A first write must assert absence (empty base); a non-empty base conflicts.
	if _, err := st.Save(ctx, mustURI("m", "d.pdf"), &parampb.PartSpec{Mpn: "LM1117"}, "stale"); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("first write with non-empty base should conflict, got %v", err)
	}
	v1, err := st.Save(ctx, mustURI("m", "d.pdf"), &parampb.PartSpec{Mpn: "LM1117"}, "")
	if err != nil || v1 == "" {
		t.Fatalf("first write: v=%q err=%v", v1, err)
	}

	// Read back matches and returns the same version token.
	got, ver2, found, err := st.Get(ctx, mustURI("m", "d.pdf"))
	if err != nil || !found || got.GetMpn() != "LM1117" || ver2 != v1 {
		t.Fatalf("readback: found=%v mpn=%q ver=%q (want %q) err=%v", found, got.GetMpn(), ver2, v1, err)
	}

	// The file now exists, so an empty base (asserting absence) conflicts; the current base wins.
	if _, err := st.Save(ctx, mustURI("m", "d.pdf"), &parampb.PartSpec{Mpn: "X"}, ""); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("empty base against an existing file should conflict, got %v", err)
	}
	v2, err := st.Save(ctx, mustURI("m", "d.pdf"), &parampb.PartSpec{Mpn: "LM1117I"}, v1)
	if err != nil || v2 == v1 {
		t.Fatalf("cas update: v2=%q v1=%q err=%v", v2, v1, err)
	}

	// The sibling landed next to the datasheet path.
	if _, err := os.Stat(filepath.Join(dir, "d.partspec.json")); err != nil {
		t.Fatalf("sibling not written: %v", err)
	}
}
