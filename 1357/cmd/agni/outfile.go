package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// outFileFlag registers the -o/--out flag on a command that writes a rendered artifact.
//
// The spelling is `render`'s. `-o -` and an omitted flag both mean stdout, so adding the flag
// changes no existing invocation and no committed capture.
func outFileFlag(cmd *cobra.Command, out *string) {
	cmd.Flags().StringVarP(out, "out", "o", "-", "write the --format output to this file (- for stdout). "+
		"Distinct from --results-out, which writes the self-contained check-result DOCUMENT that "+
		"`agni results` re-renders later; this writes what you would otherwise have redirected")
}

// redirectOut points the command's own output at a file for the rest of the run, and returns the
// closer. It is a no-op for stdout, so a caller may always defer the result.
//
// It redirects the COMMAND rather than each write site. Every format writes through
// cmd.OutOrStdout(), so one call covers all of them, including a format added later. This is safe
// only because the root command sets SilenceUsage and SilenceErrors. Cobra's usage and error text
// then cannot appear in the middle of a report.
//
// It creates the file at once, so a path that cannot be written fails the run before any work.
// A run refused before it writes anything, such as `check --table pin_map` on a design with no pin
// map, then has its empty file removed and announces none (agni issues 637 and 1342).
//
// The written-file note goes to STDERR, matching render, so `-o` composes with a pipe.
func redirectOut(cmd *cobra.Command, out string) (func(), error) {
	if out == "" || out == "-" {
		return func() {}, nil
	}
	f, err := os.Create(out) // #nosec G304 -- writes the -o file the user named
	if err != nil {
		return nil, err
	}
	w := &countingFile{f: f}
	cmd.SetOut(w)
	return func() {
		f.Close()
		if w.n == 0 {
			_ = os.Remove(out)
			return
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", absForNote(out))
	}, nil
}

// countingFile counts what is written to it, so the closer can tell a run that wrote nothing.
type countingFile struct {
	f *os.File
	n int
}

func (w *countingFile) Write(p []byte) (int, error) {
	n, err := w.f.Write(p)
	w.n += n
	return n, err
}

// absForNote is the path to PRINT for a file just written. It is absolute because a terminal
// linkifies an absolute path, and a report is usually opened from somewhere other than the
// directory it was written in.
//
// It falls back to the path as given when Abs fails (the working directory cannot be resolved),
// since a note is never worth failing a run that has already written its artifact.
func absForNote(out string) string {
	abs, err := filepath.Abs(out)
	if err != nil {
		return out
	}
	return abs
}
