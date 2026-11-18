// Package deploy holds no code. Its test reads compose.yaml beside the Dockerfile, so a change to
// either that would lose a customer's uploads fails the gate, which never runs Docker (agni issue
// 1698). hack/compose_smoke.sh brings the bundle up for real in the images workflow.
package deploy

import (
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]struct {
		Command []string `yaml:"command"`
		Volumes []string `yaml:"volumes"`
	} `yaml:"services"`
	Volumes map[string]any `yaml:"volumes"`
}

// writableFlags are the serve flags that name a folder the server writes into, as a file:// URL.
var writableFlags = []string{"--store", "--uploads"}

func readCompose(t *testing.T) composeFile {
	t.Helper()
	b, err := os.ReadFile("compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var c composeFile
	if err := yaml.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// namedTargets returns where the service mounts named volumes, as against host folders.
func namedTargets(t *testing.T, c composeFile, service string) []string {
	t.Helper()
	var out []string
	for _, v := range c.Services[service].Volumes {
		parts := strings.Split(v, ":")
		if len(parts) < 2 {
			t.Fatalf("volume %q has no target", v)
		}
		if _, named := c.Volumes[parts[0]]; named {
			out = append(out, parts[1])
		}
	}
	return out
}

func under(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, "/")+"/")
}

// A design a customer uploads, a review run and a snapshot's definition each live in a folder a
// serve flag names. Each folder must sit on a named volume, or recreating the container loses it.
func TestServeKeepsItsWritesOnANamedVolume(t *testing.T) {
	c := readCompose(t)
	targets := namedTargets(t, c, "agni")
	cmd := c.Services["agni"].Command
	for _, flag := range writableFlags {
		i := indexOf(cmd, flag)
		if i < 0 || i+1 >= len(cmd) {
			t.Errorf("the agni service's command has no %s, so the server keeps that state in memory or not at all", flag)
			continue
		}
		dir, isFile := strings.CutPrefix(cmd[i+1], "file://")
		if !isFile {
			t.Errorf("%s %s is no file:// URL", flag, cmd[i+1])
			continue
		}
		onVolume := false
		for _, target := range targets {
			onVolume = onVolume || under(dir, target)
		}
		if !onVolume {
			t.Errorf("%s %s is on no named volume (the volumes are at %v), so recreating the container loses it", flag, dir, targets)
		}
	}
}

// A named volume copies the ownership of the folder it is mounted over, and a folder the image lacks
// comes up owned by root. The server runs as the user agni, so every named volume's target must be a
// folder the Dockerfile makes, or the server cannot write to it.
func TestTheImageMakesEveryNamedVolumesFolder(t *testing.T) {
	c := readCompose(t)
	b, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	var made []string
	for _, m := range regexp.MustCompile(`mkdir -p ([^\\&\n]+)`).FindAllSubmatch(b, -1) {
		made = append(made, strings.Fields(string(m[1]))...)
	}
	if len(made) == 0 {
		t.Fatal("the Dockerfile has no mkdir -p")
	}
	targets := namedTargets(t, c, "agni")
	if len(targets) == 0 {
		t.Fatal("the agni service mounts no named volume")
	}
	for _, target := range targets {
		ok := false
		for _, dir := range made {
			ok = ok || under(path.Clean(dir), path.Clean(target))
		}
		if !ok {
			t.Errorf("the Dockerfile makes none of %s, so a named volume there is owned by root (it makes %v)", target, made)
		}
	}
}

func indexOf(xs []string, x string) int {
	for i, s := range xs {
		if s == x {
			return i
		}
	}
	return -1
}
