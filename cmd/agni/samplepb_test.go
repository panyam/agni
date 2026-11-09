package main

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// web/sample.pb is the decoder test's interop fixture (web/src/packed.test.ts), the packed sheet
// this command writes for edif/testdata/sample.eds. It holds the fixture to what the packer writes
// today, so the decoder is checked against a sheet the packer still produces (agni issue 1464).
// The fixture drifted for months before anyone looked. Regenerate it with the command in the
// decoder test's header and read the decoder test's diff.
func TestSamplePBIsWhatThePackerWrites(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "..", "readers", "edif", "testdata", "sample.eds"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "sample.pb")
	runRoot(t, "render", src, "--format", "pack", "--sheet", "0", "-o", out)
	var got, want geom.PackedSheet
	for path, msg := range map[string]*geom.PackedSheet{out: &got, filepath.Join("..", "..", "web", "sample.pb"): &want} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := proto.Unmarshal(b, msg); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if !proto.Equal(&got, &want) {
		t.Errorf("web/sample.pb is not what `agni render readers/edif/testdata/sample.eds --format pack --sheet 0` writes; regenerate it and update web/src/packed.test.ts")
	}
}
