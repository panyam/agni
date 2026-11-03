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

// outFileFixture is a design that produces findings in every format, so a format-specific
// implementation of -o shows up as an empty file rather than passing by accident.
const outFileFixture = "testdata/conformance/fires.edn"

// runWithOut executes a command capturing what it would have printed, and returns that alongside the
// file -o was pointed at. Both are returned on every call, because the interesting assertion is
// about the RELATIONSHIP between them. Output written to the file must be absent from stdout.
func runWithOut(t *testing.T, cmd *cobra.Command, args ...string) (stdout, file string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "out.txt")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append(args, "-o", p))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("-o wrote no file at %s: %v", p, err)
	}
	return buf.String(), string(b)
}

// TestOutFileDivertsEveryFormat asserts what the flag exists for. The artifact goes to the file and
// stdout is left clean, whatever --format was asked for.
//
// Every format is a row. The implementation redirects the COMMAND rather than each write site. If
// that goes wrong, it goes wrong per format. One format keeps writing to the real stdout, and its
// file comes back empty. Red-checks by dropping the cmd.SetOut in redirectOut.
func TestOutFileDivertsEveryFormat(t *testing.T) {
	for _, format := range []string{"text", "json", "csv", "markdown", "html"} {
		t.Run(format, func(t *testing.T) {
			stdout, file := runWithOut(t, checkCmd(), "--format", format, outFileFixture)
			if file == "" {
				t.Error("-o produced an empty file, so this format still writes somewhere else")
			}
			if stdout != "" {
				t.Errorf("stdout carried %d bytes that should have gone to the file: %.80q", len(stdout), stdout)
			}
		})
	}
}

// TestOutFileDefaultsToStdout pins the additive guarantee. Every committed capture and every
// existing invocation depends on two spellings meaning stdout, an omitted flag and `-o -`. So this
// test shows the flag changed nothing for anyone who does not pass it.
func TestOutFileDefaultsToStdout(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"omitted", []string{"--format", "csv", outFileFixture}},
		{"dash", []string{"--format", "csv", "-o", "-", outFileFixture}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := checkCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if buf.Len() == 0 {
				t.Error("nothing reached stdout")
			}
		})
	}
}

// TestOutFileIsNotResultsOut keeps the two apart. They write different artifacts to different paths
// and a reader could reasonably assume one supersedes the other. So this asserts that both files
// are written and that their contents differ. --results-out is the check-result DOCUMENT (JSON with
// a meta.schema that `agni results` re-renders), and -o is the rendered --format output.
func TestOutFileIsNotResultsOut(t *testing.T) {
	dir := t.TempDir()
	rendered := filepath.Join(dir, "report.txt")
	document := filepath.Join(dir, "results.json")

	cmd := checkCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "text", "-o", rendered, "--results-out", document, outFileFixture})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatalf("-o wrote nothing: %v", err)
	}
	raw, err := os.ReadFile(document)
	if err != nil {
		t.Fatalf("--results-out wrote nothing: %v", err)
	}
	if string(got) == string(raw) {
		t.Fatal("-o and --results-out wrote the same bytes, so one of them is not writing what it claims")
	}
	if json.Valid(got) {
		t.Errorf("-o --format text wrote JSON, which is the results document rather than the rendered output:\n%.200s", got)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("--results-out is not JSON: %v", err)
	}
	if _, ok := doc["meta"]; !ok {
		t.Error("--results-out carries no meta, so it is not the check-result document")
	}
}

// TestQueryWritesItsViewToAFile covers query. Its own help calls markdown and html "a VIEW
// ... ready to hand to someone", so it wants a file more than any of the others. It was missed when
// the flag first shipped. The commands share one helper, so this is a wiring test rather than a
// second implementation. One format is enough here, and TestOutFileDivertsEveryFormat sweeps every
// format over check.
func TestQueryWritesItsViewToAFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "view.html")
	var out, errOut bytes.Buffer
	cmd := queryCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{outFileFixture, `component.class(?c, ?k) => ?k, count(?c)`, "--format", "html", "-o", p})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("-o wrote no file: %v", err)
	}
	if !bytes.Contains(b, []byte("<html")) {
		t.Errorf("the file is not the html view: %.120s", b)
	}
	if out.Len() != 0 {
		t.Errorf("stdout carried %d bytes that should have gone to the file", out.Len())
	}
}

// TestOutFileNoteGoesToStderr keeps `-o` composable with a pipe. The human line about the write must
// not reach whatever reads stdout next. render has worked this way since it shipped.
func TestOutFileNoteGoesToStderr(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.txt")
	var out, errOut bytes.Buffer
	cmd := checkCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--format", "csv", "-o", p, outFileFixture})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "wrote "+p) {
		t.Errorf("stderr does not name the written file: %q", errOut.String())
	}
	if strings.Contains(out.String(), "wrote ") {
		t.Errorf("the written-file note reached stdout: %q", out.String())
	}
}

// TestOutFileUnwritablePathErrors checks that a path that cannot be created fails the run. A silent
// fallback to stdout would leave the operator with an artifact they cannot find.
func TestOutFileUnwritablePathErrors(t *testing.T) {
	cmd := checkCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"-o", filepath.Join(t.TempDir(), "no-such-dir", "out.txt"), outFileFixture})
	if err := cmd.Execute(); err == nil {
		t.Error("an uncreatable --out path succeeded, so the output went somewhere unasked for")
	}
}

// TestOutFileNoteIsAbsolute holds the note to a path the reader can act on. A relative `-o` is
// resolved before it is printed. A terminal linkifies an absolute path into a click, and a relative
// one only means something to someone standing where the command ran. Reports are the main reason.
// A report is written from wherever the design is and opened from a browser somewhere else.
func TestOutFileNoteIsAbsolute(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.Abs(outFileFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	var out, errOut bytes.Buffer
	cmd := checkCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--format", "csv", "-o", "report.csv", fixture})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// EvalSymlinks is needed because a temp dir is under a symlinked /var on darwin. The note
	// reports the path from Abs rather than the resolved one. Comparing the basename alone would
	// pass on the relative spelling this test exists to reject. The written-file note is the line
	// that starts "wrote"; other notes (no board was read) may precede it on the same stream.
	var got string
	for _, line := range strings.Split(errOut.String(), "\n") {
		if strings.HasPrefix(line, "wrote ") {
			got = strings.TrimSpace(strings.TrimPrefix(line, "wrote "))
		}
	}
	if !filepath.IsAbs(got) {
		t.Errorf("note is not an absolute path: %q", errOut.String())
	}
	if filepath.Base(got) != "report.csv" {
		t.Errorf("note names the wrong file: %q", got)
	}
}
