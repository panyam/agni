package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/mounts"
)

// remoteParityServer is an `agni serve` with no mounts at all, and this process mounts the tutorial
// project as tut, so a remote run's answer comes from the files it sent (agni issue 1061). served
// reports the procedures the server has answered so far.
func remoteParityServer(t *testing.T) (url, design string, served func() []string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	mux := servedMuxFor(t, []mounts.Mount{{Name: "tut", Root: root}}, nil)
	var mu sync.Mutex
	var procs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		procs = append(procs, r.URL.Path)
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, filepath.Join(root, "designs", "gateway"), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), procs...)
	}
}

// runBoth runs one command locally and again with --remote, returning both stdouts.
func runBoth(t *testing.T, mk func() *cobra.Command, url string, args ...string) (local, remote string) {
	t.Helper()
	return runCLI(t, mk(), args...), runCLI(t, mk(), append(append([]string{}, args...), "--remote", url)...)
}

// TestRemoteAnswersAsTheLocalCommandDoes is agni issue 998's acceptance: each command that can send
// its request to a server prints the same bytes whichever engine answered, in every format that
// renders the response a different way.
func TestRemoteAnswersAsTheLocalCommandDoes(t *testing.T) {
	url, design, served := remoteParityServer(t)
	cases := []struct {
		name string
		mk   func() *cobra.Command
		// rpc is the procedure the remote run must have reached, so a --remote the command ignored
		// cannot pass by printing its local answer twice.
		rpc  string
		args []string
	}{
		{"query text", queryCmd, "/RunQuery", []string{design, `component.mpn(?r, ?m) => ?r, ?m`}},
		{"query json", queryCmd, "/RunQuery", []string{design, `component.mpn(?r, ?m) => ?r, ?m`, "--format", "json"}},
		{"query markdown", queryCmd, "/RunQuery", []string{design, `component.mpn(?r, ?m) => ?r, ?m`, "--format", "markdown"}},
		{"check text", checkCmd, "/CheckDesign", []string{design}},
		{"check json", checkCmd, "/CheckDesign", []string{design, "--format", "json"}},
		{"check verdicts", checkCmd, "/CheckDesign", []string{design, "--verdicts"}},
		{"check subject", checkCmd, "/CheckDesign", []string{design, "--verdicts", "--subject", "component:U1"}},
		{"check markdown", checkCmd, "/GetCheckReport", []string{design, "--format", "markdown"}},
		{"trace text", traceCmd, "/TraceDesign", []string{design, "--from", "J1.1", "--to", "U1.1"}},
		{"trace json", traceCmd, "/TraceDesign", []string{design, "--from", "J1.1", "--to", "U1.1", "--format", "json"}},
		{"review markdown", reviewCmd, "/CreateReview", []string{design}},
		{"review json", reviewCmd, "/CreateReview", []string{design, "--format", "json"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := len(served())
			local, remote := runBoth(t, c.mk, url, c.args...)
			reached := false
			for _, p := range served()[before:] {
				reached = reached || strings.HasSuffix(p, c.rpc)
			}
			if !reached {
				t.Errorf("the --remote run never called %s on the server (it called %v)", c.rpc, served()[before:])
			}
			// A count guard, so an empty answer on both sides cannot pass as agreement.
			if len(strings.TrimSpace(local)) < 40 {
				t.Fatalf("the local run printed %d bytes, too few to compare:\n%s", len(local), local)
			}
			if local != remote {
				t.Errorf("--remote printed something else.\n--- local\n%s\n--- remote\n%s", local, remote)
			}
		})
	}
}

// TestRemoteRefusesWhatItCannotSend holds the two refusals --remote makes rather than answering from
// somewhere else: a flag that configures this process's engine, and a design whose files this
// process cannot list to send, which is refused before anything reaches the server.
func TestRemoteRefusesWhatItCannotSend(t *testing.T) {
	url, design, served := remoteParityServer(t)
	run := func(mk func() *cobra.Command, args ...string) error {
		cmd := mk()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(append(args, "--remote", url))
		return cmd.Execute()
	}
	err := run(checkCmd, design, "--params", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "--params configures the engine in this process") {
		t.Errorf("check --params --remote: err = %v, want the local-only refusal", err)
	}
	err = run(func() *cobra.Command { return withTiming(queryCmd(), true) }, design, `component.mpn(?r, ?m) => ?r`, "--timing")
	if err == nil || !strings.Contains(err.Error(), "--timing times this process") {
		t.Errorf("query --timing --remote: err = %v, want the timing refusal", err)
	}
	before := len(served())
	err = run(queryCmd, filepath.Join(t.TempDir(), "elsewhere.kicad_sch"), `component.mpn(?r, ?m) => ?r`)
	if err == nil || !strings.Contains(err.Error(), "files to send") {
		t.Errorf("query of a path that is not here: err = %v, want the refusal to list its files", err)
	}
	for _, p := range served()[before:] {
		if strings.HasSuffix(p, "/Ingest") {
			t.Errorf("a design whose files could not be listed was ingested anyway (%s)", p)
		}
	}
}

// TestABadQueryPointsAtItsPlace: a query that would not parse prints the line it stopped on with a
// caret under the place, whether this process or a --remote server parsed it.
func TestABadQueryPointsAtItsPlace(t *testing.T) {
	url, design, _ := remoteParityServer(t)
	want := "\n  component.net(?r ?n) => ?r\n                ^"
	for _, extra := range [][]string{nil, {"--remote", url}} {
		cmd := queryCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(append([]string{design, `component.net(?r ?n) => ?r`}, extra...))
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("%v: the malformed query was answered", extra)
		}
		if !strings.HasSuffix(err.Error(), want) {
			t.Errorf("%v: error\n%s\nwant it to end with%s", extra, err, want)
		}
	}
}
