package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// gatewayDescriptor names the tutorial gateway's schematic as its entry and declares the intent the
// tutorial's own design.yaml declares for its nets.
const gatewayDescriptor = `name: gateway
entry: gateway.kicad_sch
companions: [gateway.kicad_pcb]
intent:
  nets:
    PMIC_MAIN_12V0: {nominal: 12.0, domain: main}
    PMIC_CORE_3V3: {nominal: 3.3, domain: io}
    PMIC_IO_1V8: {nominal: 3.3, domain: core}
`

// A design linked from a GitHub repo keeps the intent its design.yaml declares, so a check of it
// gives the findings a check of the same files does (agni issue 1854).
func TestALinkFromGitHubChecksUnderTheDesignYAMLsIntent(t *testing.T) {
	files := map[string]string{"hw/design.yaml": gatewayDescriptor}
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "design.yaml"), []byte(gatewayDescriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"gateway.kicad_sch", "gateway.kicad_pcb", "gateway.kicad_pro"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "examples", "tutorial-project", "boards", "gateway", f))
		if err != nil {
			t.Fatal(err)
		}
		files["hw/"+f] = string(b)
		if err := os.WriteFile(filepath.Join(local, f), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gh, _ := privateGitHubRepo(t)
	gh.AddRepo("acme", "open", false, files)
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runMounts(t, "add", "open", "github://acme/open", "--remote", url))
	must(runDesigns(t, url, "create", "gw"))
	must(runDesigns(t, url, "link", "gw", "schematic", "open/hw/gateway.kicad_sch@main"))
	must(runDesigns(t, url, "link", "gw", "board", "open/hw/gateway.kicad_pcb@main"))

	stored := findingKeys(t, "check", "designs/gw", "--remote", url)
	onDisk := findingKeys(t, "check", local)
	intentRules := slices.ContainsFunc(onDisk, func(k string) bool { return len(k) > 7 && k[:7] == "intent/" })
	if !intentRules {
		t.Fatalf("a check of the files finds nothing from an intent/ rule, so the comparison below proves nothing: %v", onDisk)
	}
	if !slices.Equal(stored, onDisk) {
		t.Errorf("check designs/gw = %d findings, check of its files = %d; want the same set\nonly stored: %v\nonly on disk: %v", len(stored), len(onDisk), minus(stored, onDisk), minus(onDisk, stored))
	}
}
