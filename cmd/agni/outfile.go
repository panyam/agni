package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// outFileFlag registers the -o/--out flag on a command that writes a rendered artifact.
//
// The spelling is `render`'s, verbatim, because a second convention for the same idea is worse than
// either convention: `-o -` and an omitted flag both mean stdout, so adding the flag changes no
// existing invocation and no committed capture.
func outFileFlag(cmd *cobra.Command, out *string) {
	cmd.Flags().StringVarP(out, "out", "o", "-", "write the --format output to this file (- for stdout). "+
		"Distinct from --results-out, which writes the self-contained check-result DOCUMENT that "+
		"`agni results` re-renders later; this writes what you would otherwise have redirected")
}

// redirectOut points the command's own output at a file for the rest of the run, and returns the
// closer. It is a no-op for stdout, so a caller may always defer the result.
//
// Redirecting the COMMAND rather than each write site is what keeps this small: every format already
// writes through cmd.OutOrStdout(), so one call covers text, csv, json, markdown, report and html
// without a per-format branch, and a format added later inherits it. That is only safe because the
// root command sets SilenceUsage and SilenceErrors, so cobra's own usage and error text cannot land
// in the middle of a report.
//
// The written-file note goes to STDERR, matching render, so `-o` composes with a pipe: the artifact
// is in the file and the human line is not in whatever reads stdout next.
func redirectOut(cmd *cobra.Command, out string) (func(), error) {
	if out == "" || out == "-" {
		return func() {}, nil
	}
	f, err := os.Create(out)
	if err != nil {
		return nil, err
	}
	cmd.SetOut(f)
	return func() {
		f.Close()
		fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", absForNote(out))
	}, nil
}

// absForNote is the path to PRINT for a file just written. Absolute, because the note's job is to
// hand the reader something they can act on: a terminal linkifies an absolute path into a click, and
// a relative one is only meaningful to someone standing in the directory the command ran in. That is
// the common case for a report, which is written from wherever the design happens to be and opened
// from a browser somewhere else entirely.
//
// Falls back to the path as given, because Abs fails only when the working directory cannot be
// resolved, and a note is never worth failing a run that has already written its artifact.
func absForNote(out string) string {
	abs, err := filepath.Abs(out)
	if err != nil {
		return out
	}
	return abs
}
