//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// A mount given as a Windows path, with a drive letter and backslashes, reads the designs under it
// (agni issue 1595). The drive letter is given in the other case, since Windows paths match without
// regard to case and a person may type either.
func TestADriveLetterMountReadsItsDesigns(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	vol := filepath.VolumeName(root)
	if len(vol) != 2 || vol[1] != ':' {
		t.Fatalf("the checkout is at %s, which has no drive letter", root)
	}
	other := strings.ToLower(vol)
	if other == vol {
		other = strings.ToUpper(vol)
	}
	root = other + root[len(vol):]
	if !strings.Contains(root, `\`) {
		t.Fatalf("%s holds no backslash", root)
	}
	freshWorkspace(t)
	var got webapi.GetSummaryResponse
	if err := protojson.Unmarshal([]byte(runRoot(t, "stats", "--mount", "tut="+root, "mount://tut/boards/gateway", "--format", "json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.GetComponentCount() == 0 || got.GetNetCount() == 0 {
		t.Errorf("mount tut=%s reads %d components and %d nets from boards/gateway", root, got.GetComponentCount(), got.GetNetCount())
	}
}
