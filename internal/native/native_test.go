package native

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func kicadInstalled() bool {
	_, err := exec.LookPath("kicad-cli")
	return err == nil
}

func TestNativeAvailable(t *testing.T) {
	if Available(".eds", map[string]bool{"kicad-cli": true}) {
		t.Error(".eds has no native renderer; want unavailable")
	}
	if Available(".kicad_sch", map[string]bool{}) {
		t.Error("kicad-cli not enabled; want unavailable")
	}
	if got := Available(".kicad_pcb", map[string]bool{"kicad-cli": true}); got != kicadInstalled() {
		t.Errorf("Available(.kicad_pcb, enabled) = %v, want %v", got, kicadInstalled())
	}
	// With the tool enabled, availability tracks whether the binary is installed.
	got := Available(".kicad_sch", map[string]bool{"kicad-cli": true})
	if got != kicadInstalled() {
		t.Errorf("Available(.kicad_sch, enabled) = %v, want %v (kicad-cli installed=%v)", got, kicadInstalled(), kicadInstalled())
	}
}

// TestXschemArgs pins the xschem export invocation: xschem 2.8.x has no --plotfile and writes
// plot.svg into the working directory (runRender sets that to the temp outDir), so the args are
// just the headless-export flags plus the input. Verified end-to-end via Dockerfile.native-tools.
func TestXschemArgs(t *testing.T) {
	got := xschemNative.args("/abs/design.sch", "/tmp/out", 1)
	want := []string{"--no_x", "--quit", "--svg", "/abs/design.sch"}
	if len(got) != len(want) {
		t.Fatalf("xschem args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("xschem args = %v, want %v", got, want)
		}
	}
}

// The shared .sch extension resolves its native renderer by sniffing the header: xschem uses
// the xschem tool, gEDA uses lepton-cli, and a legacy-KiCad .sch has none.
func TestNativeRendererForSch(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		file, content, wantTool string
		wantOK                  bool
	}{
		{"a.sch", "v {xschem version=3.4.4}\n", "xschem", true},
		{"b.sch", "v 20200319 2\n", "lepton-cli", true},
		{"c.sch", "EESchema Schematic File Version 4\n", "", false}, // legacy KiCad
	}
	for _, c := range cases {
		r, ok := nativeRendererFor(write(c.file, c.content))
		if ok != c.wantOK || r.tool != c.wantTool {
			t.Errorf("%s: nativeRendererFor = (%q,%v), want (%q,%v)", c.file, r.tool, ok, c.wantTool, c.wantOK)
		}
	}
}
