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
// The boundary is one question: does this change WHAT is checked, or only WHERE bytes are found?
// Conventions, profiles, parameters, intent and a review checklist change the answer, so they belong
// to a PROJECT (agni.v1.webapi.AnalysisConfig), scoped to the designs that declared them. Mounts and
// symbol paths only locate input and belong to a machine, so a plain file holds them. An analysis
// tier added here would apply to every design the CLI opened; see
// docsite/content/architecture/projects-and-designs.md#three-tiers-of-configuration.

// envConfigName is the file, looked for beside the work as well as in a home directory, because a
// repo checked out on two machines wants the same mounts on both.
const envConfigName = "agni.yaml"

// maxEnvConfigWalk bounds the upward search from the working directory, as the project walk does, so
// a stray agni.yaml far up a home directory never becomes the mount table a command ran against.
const maxEnvConfigWalk = 4

// envConfig is the tier-1 file's shape.
//
// A field here reaches every command in the process with no design to scope it to, so anything whose
// wrong value produces a QUIET wrong answer rather than a loud failure does not belong.
type envConfig struct {
	// Mounts expose folders as `name: path`, the file form of --mount.
	Mounts map[string]string `yaml:"mounts"`
	// SymbolPaths are default symbol-library search directories, the file form of --symbol-path.
	// This scope suits a vendor library installed system-wide. A PROJECT's own libraries go in its
	// descriptor (AnalysisConfig.symbol_path_uris), where they travel with the design and reach a
	// served surface too.
	SymbolPaths []string `yaml:"symbol_paths"`
	// WebDir is where the viewer's own assets live, the file form of --web-dir. A wrong value fails
	// at startup, because checkWebAssets stats four named files before the listener opens.
	WebDir string `yaml:"web_dir"`
	// NativeTools names the native golden renderers a served deployment may shell out to, the file
	// form of serve's --enable-native. A tool missing from PATH fails at the point of use with its
	// own name in the error.
	//
	// Only serve consumes it, at the point nativeTools is used rather than in the pre-run hook. The
	// pre-run note still names it, as it does WebDir, because the note reports what was LOADED.
	NativeTools []string `yaml:"native_tools"`
}

// loadEnvConfig finds and parses the nearest agni.yaml, returning the zero value when there is none.
//
// The search is nearest-first from the working directory and then the user config directory, and the
// FIRST hit wins outright rather than merging, so the effective mount table never depends on which
// directory a command ran from.
//
// A malformed file, including one with an unknown key, is an ERROR rather than a skip. A skipped
// mount table would send every path through a minted mount instead, which reads as working.
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
