package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// runMounts runs `agni mounts <args>` and returns what it printed.
func runMounts(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := mountsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestTheMountsCommandAddsListsAndRemovesAGitHubMount(t *testing.T) {
	gh, fakeCommit := jetsonRepo(t)
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL})

	out, err := runMounts(t, "add", "jetson", "github://antmicro/jetson-orin-baseboard", "--ref", fakeCommit, "--remote", url)
	if err != nil || out != "jetson\tgithub\tgithub://antmicro/jetson-orin-baseboard@"+fakeCommit+"\truntime\n" {
		t.Fatalf("mounts add = %q, %v", out, err)
	}
	out, err = runMounts(t, "ls", "--remote", url)
	if err != nil || !strings.Contains(out, "uploads\tuploads\t\tfixed\n") || !strings.Contains(out, "jetson\tgithub\tgithub://antmicro/jetson-orin-baseboard@"+fakeCommit+"\truntime\n") {
		t.Fatalf("mounts ls = %q, %v; want the uploads mount fixed and jetson at runtime", out, err)
	}
	if out, err := runMounts(t, "rm", "uploads", "--remote", url); err == nil || !strings.Contains(err.Error(), "comes from the server's flags") {
		t.Fatalf("mounts rm uploads = %q, %v; want a refusal naming the flags", out, err)
	}
	if out, err := runMounts(t, "rm", "jetson", "--remote", url); err != nil || out != "removed jetson\n" {
		t.Fatalf("mounts rm jetson = %q, %v", out, err)
	}
	if out, _ := runMounts(t, "ls", "--remote", url); strings.Contains(out, "jetson") {
		t.Fatalf("after rm, mounts ls = %q; want no jetson", out)
	}
}

func TestTheMountsCommandSaysWhatItNeeds(t *testing.T) {
	if _, err := runMounts(t, "ls"); err == nil || !strings.Contains(err.Error(), "needs --remote") {
		t.Errorf("mounts ls with no --remote = %v; want it to ask for --remote", err)
	}
	plain := serveViewer(t, viewerOpts{})
	_, err := runMounts(t, "add", "x", "github://a/b", "--remote", plain)
	if err == nil || !strings.Contains(err.Error(), "start it with --store and --uploads") {
		t.Errorf("mounts add on a server with no store = %v; want it to name --store and --uploads", err)
	}
}
