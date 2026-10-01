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
// cmd.OutOrStdout(), so one call covers all of them, including a format added later. That is safe
// only because the root command sets SilenceUsage and SilenceErrors, so cobra's usage and error
// text cannot land in the middle of a report.
//
// It creates the file, so a caller resolves anything that can refuse the run (--server) BEFORE
// calling it, or a refused run still announces "wrote <file>" over an empty one (agni issue 637).
//
// The written-file note goes to STDERR, matching render, so `-o` composes with a pipe.
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
