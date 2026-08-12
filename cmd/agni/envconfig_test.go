package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnvCfg(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, envConfigName)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func noEnv(string) string { return "" }

// TestEnvConfigLoadsMountsAndSymbolPaths is the point: a repo can carry its own mount table so a
// contributor's commands address designs the same way a server does, without a flag list.
func TestEnvConfigLoadsMountsAndSymbolPaths(t *testing.T) {
	root := t.TempDir()
	writeEnvCfg(t, root, "mounts:\n  boards: /srv/boards\n  shared: /srv/shared\nsymbol_paths:\n  - /usr/share/syms\n")
	cfg, path, err := loadEnvConfig(root, noEnv)
	if err != nil {
		t.Fatalf("loadEnvConfig: %v", err)
	}
	if path == "" {
		t.Fatal("the file that was used must be reported, so a run can say where its mounts came from")
	}
	// Rendered sorted, so the table a run builds does not depend on map iteration order.
	got := cfg.mountSpecs()
	want := []string{"boards=/srv/boards", "shared=/srv/shared"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("mountSpecs = %v, want %v", got, want)
	}
	if len(cfg.SymbolPaths) != 1 {
		t.Errorf("symbol paths = %v", cfg.SymbolPaths)
	}
}

// TestEnvConfigNearestWins: the first hit walking up wins OUTRIGHT rather than merging. Merging two
// tables would make the effective set depend on which directory a command ran from.
func TestEnvConfigNearestWins(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	writeEnvCfg(t, root, "mounts:\n  far: /far\n")
	writeEnvCfg(t, deep, "mounts:\n  near: /near\n")
	cfg, _, err := loadEnvConfig(deep, noEnv)
	if err != nil {
		t.Fatalf("loadEnvConfig: %v", err)
	}
	if len(cfg.Mounts) != 1 || cfg.Mounts["near"] != "/near" {
		t.Errorf("the nearest file should win outright, got %v", cfg.Mounts)
	}
}

// TestEnvConfigAbsenceIsNotAnError: running anywhere without one is the ordinary case.
func TestEnvConfigAbsenceIsNotAnError(t *testing.T) {
	cfg, path, err := loadEnvConfig(t.TempDir(), noEnv)
	if err != nil || path != "" || len(cfg.Mounts) != 0 {
		t.Errorf("no file should yield the zero value and no error, got %v %q %v", cfg, path, err)
	}
}

// TestEnvConfigRejectsMalformedAndUnknown. A malformed file is an error rather than a skip: an
// operator who wrote a mount table and silently got none would see every path resolve through a
// minted mount instead, which reads as working.
//
// Unknown fields are rejected on the same reasoning the descriptors use — a misspelled key that
// silently does nothing is the failure worth spending strictness on. It also guards the boundary this
// file exists to hold: someone reaching for `conventions:` here gets told no rather than getting a
// machine-wide analysis tier.
func TestEnvConfigRejectsMalformedAndUnknown(t *testing.T) {
	for name, body := range map[string]string{
		"malformed":     "mounts: [this is not a map\n",
		"analysis tier": "conventions: house.yaml\n",
		"typo":          "symbolpaths:\n  - /x\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeEnvCfg(t, dir, body)
			if _, _, err := loadEnvConfig(dir, noEnv); err == nil {
				t.Errorf("%s should be an error, not a silent empty config", name)
			}
		})
	}
}

// TestEnvConfigFallsBackToTheUserDirectory, so a machine-wide default works when the work tree has no
// file of its own.
func TestEnvConfigFallsBackToTheUserDirectory(t *testing.T) {
	home := t.TempDir()
	writeEnvCfg(t, filepath.Join(home, "agni"), "mounts:\n  global: /g\n")
	getenv := func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return home
		}
		return ""
	}
	cfg, path, err := loadEnvConfig(t.TempDir(), getenv)
	if err != nil {
		t.Fatalf("loadEnvConfig: %v", err)
	}
	if cfg.Mounts["global"] != "/g" || !strings.Contains(path, "agni") {
		t.Errorf("the user config directory should be searched after the work tree, got %v %q", cfg.Mounts, path)
	}
}

// TestApplyEnvConfigYieldsToFlags: a passed flag wins outright. An operator who named a mount table is
// answering for the whole table, and a file quietly adding one they did not ask for is the
// ambient-config failure this tier is only allowed because it cannot change an answer.
func TestApplyEnvConfigYieldsToFlags(t *testing.T) {
	dir := t.TempDir()
	writeEnvCfg(t, dir, "mounts:\n  fromfile: /f\n")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	defer func() { _ = os.Chdir(cwd) }()

	saved := cliMountSpecs
	defer func() { cliMountSpecs = saved }()

	cliMountSpecs = []string{"typed=/t"}
	var note strings.Builder
	if err := applyEnvConfig(&note, noEnv); err != nil {
		t.Fatalf("applyEnvConfig: %v", err)
	}
	if len(cliMountSpecs) != 1 || cliMountSpecs[0] != "typed=/t" {
		t.Errorf("an explicit --mount must win outright, got %v", cliMountSpecs)
	}
	if note.Len() != 0 {
		t.Errorf("nothing was taken from the file, so nothing should be announced, got %q", note.String())
	}

	// With no flag, the file fills in and SAYS so: a mount table nobody typed is not recoverable from
	// the output of a run that used it.
	cliMountSpecs = nil
	note.Reset()
	if err := applyEnvConfig(&note, noEnv); err != nil {
		t.Fatalf("applyEnvConfig: %v", err)
	}
	if len(cliMountSpecs) != 1 || cliMountSpecs[0] != "fromfile=/f" {
		t.Errorf("the file should fill in an unpassed flag, got %v", cliMountSpecs)
	}
	if !strings.Contains(note.String(), envConfigName) {
		t.Errorf("the run should name the file it took config from, got %q", note.String())
	}
}
