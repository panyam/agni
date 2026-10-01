package opscmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/version"
)

// TestVersionCmd pins that the version a human is told is the
// SAME string internal/version stamps into a results document's provenance. A build that reported
// one version at the prompt and another into an archived report would be worse than no command,
// because the report is the artifact someone reads months later and cannot re-derive.
func TestVersionCmd(t *testing.T) {
	var out bytes.Buffer
	cmd := Version("agni")
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()

	// Matched exactly, not with Contains, because a decorated version ("9.9.9-" + the real one) still
	// contains the real one, so a substring check would pass a command that reports the wrong
	// build. The first line must BE the version and nothing else.
	wantFirst := "agni " + version.Version()
	if first, _, _ := strings.Cut(got, "\n"); first != wantFirst {
		t.Errorf("first line = %q, want %q", first, wantFirst)
	}
	// The toolchain and platform are the detail provenance has no field for, and the reason the
	// command exists at all rather than just --version.
	for _, want := range []string{runtime.Version(), runtime.GOOS, runtime.GOARCH} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q is missing %q", got, want)
		}
	}
}
