package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runFailing runs a command as main does, error report included, and returns its stdout and stderr.
func runFailing(t *testing.T, cmd *cobra.Command, args ...string) (stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	// As the root command does, so no usage text lands in the middle of a report.
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("%v succeeded, want a failure", args)
	}
	reportError(cmd, err, &errOut)
	return out.String(), errOut.String()
}

// A failed request under --format json writes the error as the Connect protocol does, the same
// bytes whether this process or a --remote server answered, with a syntax error's position as a
// detail. The text error and its caret still go to stderr.
func TestAFailedRequestWritesTheWireErrorUnderJSON(t *testing.T) {
	url, design, _ := remoteParityServer(t)
	args := []string{design, `component.net(?r ?n) => ?r`, "--format", "json"}
	local, stderr := runFailing(t, queryCmd(), args...)
	remote, _ := runFailing(t, queryCmd(), append(args, "--remote", url)...)
	var got struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details []struct {
			Type  string         `json:"type"`
			Debug map[string]any `json:"debug"`
		} `json:"details"`
	}
	if err := json.Unmarshal([]byte(local), &got); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, local)
	}
	if got.Code != "invalid_argument" || !strings.Contains(got.Message, "variable ?r ?n") {
		t.Errorf("error %+v, want invalid_argument naming the variable", got)
	}
	if len(got.Details) != 1 || got.Details[0].Type != "agni.v1.webapi.TextPosition" || got.Details[0].Debug["column"] != 15.0 || got.Details[0].Debug["caret"] != "component.net(?r ?n) => ?r\n              ^" {
		t.Errorf("details %+v, want one TextPosition at column 15 carrying the engine's caret", got.Details)
	}
	if local != remote {
		t.Errorf("--remote wrote another error.\n--- local\n%s\n--- remote\n%s", local, remote)
	}
	if !strings.Contains(stderr, "error: ") || !strings.Contains(stderr, "^") {
		t.Errorf("stderr lost the text error or its caret:\n%s", stderr)
	}
}

// Without --format json nothing changes: the error is text on stderr and stdout stays empty.
func TestAFailedRequestWritesNoJSONInOtherFormats(t *testing.T) {
	_, design, _ := remoteParityServer(t)
	out, _ := runFailing(t, queryCmd(), design, `component.net(?r ?n) => ?r`)
	if out != "" {
		t.Errorf("stdout %q, want nothing", out)
	}
}

// A set writes its answer and then exits non-zero over the query that failed. That status is not a
// failed request, so stdout holds the one RunQueriesResponse and no error document after it.
func TestASetsStatusAfterItsAnswerAddsNoSecondDocument(t *testing.T) {
	_, design, _ := remoteParityServer(t)
	set := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(set, []byte("queries:\n  - name: bad\n    query: component.net(?r ?n) => ?r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := runFailing(t, queryCmd(), design, "--set", set, "--format", "json")
	dec := json.NewDecoder(strings.NewReader(out))
	var first map[string]any
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	if _, ok := first["results"]; !ok {
		t.Errorf("the first document is not the set's answer: %v", first)
	}
	if dec.More() {
		t.Errorf("stdout holds a second document after the answer:\n%s", out)
	}
}

// -o names where the answer would have gone, so the error goes there too.
func TestAFailedRequestWritesItsErrorWhereTheAnswerWouldGo(t *testing.T) {
	_, design, _ := remoteParityServer(t)
	dest := filepath.Join(t.TempDir(), "answer.json")
	out, _ := runFailing(t, queryCmd(), design, `compnent.net(?r, ?n) => ?r`, "--format", "json", "-o", dest)
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"code":"invalid_argument"`) || !strings.Contains(string(b), "did you mean") {
		t.Errorf("%s holds %q, want the wire error", dest, b)
	}
	if out != "" {
		t.Errorf("stdout %q, want nothing when -o names a file", out)
	}
}

// The other commands whose json is a wire message write a failed request the same way.
func TestEveryJSONCommandWritesAFailedRequestAsTheWireDoes(t *testing.T) {
	_, design, _ := remoteParityServer(t)
	missing := filepath.Join(filepath.Dir(design), "no-such-design.kicad_sch")
	for _, c := range []struct {
		name string
		mk   func() *cobra.Command
		args []string
	}{
		{"check", checkCmd, []string{missing, "--format", "json"}},
		{"trace", traceCmd, []string{missing, "--from", "J1.1", "--to", "U1.1", "--format", "json"}},
		{"review", reviewCmd, []string{missing, "--checklist", filepath.Join("..", "..", "examples", "design-review", "checklist.yaml"), "--format", "json"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, _ := runFailing(t, c.mk(), c.args...)
			var got struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil || got.Code == "" {
				t.Errorf("stdout %q is not a wire error (%v)", out, err)
			}
		})
	}
}
