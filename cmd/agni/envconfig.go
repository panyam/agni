package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file is TIER-1 configuration: where bytes are and what tools exist, as opposed to what a
// design is checked against.
//
// The two tiers are kept apart deliberately, and the boundary is one question: does this change WHAT
// is checked, or only WHERE bytes are found? Naming conventions, interface profiles, seeded
// parameters, design intent and a review checklist all change the answer, so they belong to a PROJECT
// (agni.v1.webapi.AnalysisConfig) where they are scoped to the designs that declared them. Mounts and
// symbol search paths only locate input, and they are properties of a machine rather than of a team,
// so a plain file is the right home and there is no isolation to get wrong.
//
// Putting an analysis tier in here would undo that. A machine-wide conventions file applying to every
// design a CLI opened is precisely the bug per-design config fixed — one team's vocabulary reaching
// another team's board, correct in isolation and aimed at the wrong design. This file must stay
// boring.

// envConfigName is the file, looked for beside the work rather than only in a home directory: a repo
// checked out on two machines wants the same mounts on both, and that makes it a repo artifact.
const envConfigName = "agni.yaml"

// maxEnvConfigWalk bounds the upward search from the working directory, on the same reasoning as the
// project walk: a stray agni.yaml far up someone's home directory should never silently become the
// mount table a command ran against.
const maxEnvConfigWalk = 4

// envConfig is the tier-1 file's shape.
//
// It carries only what it can carry safely. A field here reaches every command in the process with no
// design to scope it to, so anything whose wrong value produces a QUIET wrong answer rather than a
// loud failure does not belong.
type envConfig struct {
	// Mounts expose folders as `name: path`, the file form of --mount.
	Mounts map[string]string `yaml:"mounts"`
	// SymbolPaths are default symbol-library search directories, the file form of --symbol-path.
	//
	// A machine-wide default is the honest scope for a vendor library installed system-wide. A
	// PROJECT's own libraries belong in its descriptor instead (AnalysisConfig.symbol_path_uris),
	// where they travel with the design and reach a served surface too.
	SymbolPaths []string `yaml:"symbol_paths"`
	// WebDir is where the viewer's own assets live, the file form of --web-dir.
	//
	// It passes this tier's test cleanly: it locates BYTES and cannot change what a run concludes. A
	// wrong value fails loudly at startup, because checkWebAssets stats four named files before the
	// listener opens, which is the property that lets this tier hold a value at all.
	WebDir string `yaml:"web_dir"`
}

// loadEnvConfig finds and parses the nearest agni.yaml, returning the zero value when there is none.
//
// The search is nearest-first from the working directory and then the user config directory, and the
// FIRST hit wins outright rather than merging. Merging two mount tables would make the effective set
// depend on which directory a command was run from, which is exactly the ambient-config problem this
// tier is allowed to have only because it cannot change an answer.
//
// A malformed file is an ERROR rather than a skip. An operator who wrote a mount table and silently
// got none would see every path resolve through a minted mount instead, which reads as working.
func loadEnvConfig(cwd string, getenv func(string) string) (envConfig, string, error) {
	for _, dir := range envConfigSearch(cwd, getenv) {
		path := filepath.Join(dir, envConfigName)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg envConfig
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		dec.KnownFields(true)
		if err := dec.Decode(&cfg); err != nil {
			return envConfig{}, path, fmt.Errorf("%s: %w", path, err)
		}
		return cfg, path, nil
	}
	return envConfig{}, "", nil
}

// envConfigSearch is the directory list, nearest first.
func envConfigSearch(cwd string, getenv func(string) string) []string {
	var dirs []string
	dir := cwd
	for range maxEnvConfigWalk + 1 {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if home := getenv("XDG_CONFIG_HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, "agni"))
	} else if home := getenv("HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, ".config", "agni"))
	}
	return dirs
}

// mountSpecs renders the file's mounts as the `name=path` strings --mount takes, sorted so the table
// a run builds does not depend on map iteration order.
func (c envConfig) mountSpecs() []string {
	names := make([]string, 0, len(c.Mounts))
	for n := range c.Mounts {
		names = append(names, n)
	}
	sortStrings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, n+"="+c.Mounts[n])
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
