package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
)

// promoteCmd moves a workbench DRAFT into a published corpus and updates the corpus's index (agni
// issues 747, 749).
//
// The workbench saves an unvalidated <stem>.partspec.json on purpose, and LoadSet never reads one, so
// until a draft is promoted no check sees it. This is the step that validates. corpus.Promote decides
// and this writes the spec, re-loads the corpus and restores what was there if it no longer loads, so
// a promotion cannot leave a corpus that fails every check, and only then writes the index.
func promoteCmd() *cobra.Command {
	var to string
	c := &cobra.Command{
		Use:   "promote <draft.partspec.json>",
		Short: "Validate a workbench draft, write it into a corpus as <mpn>.textproto, and update the index",
		Long: `Promote a PartSpec draft the datasheets workbench saved into a published corpus, where checks
and queries read it, and record it in the corpus's index (` + corpus.IndexFile + `).

A draft is saved without validation so work is never lost, and no check reads one. Promotion runs
param.Validate and refuses a draft that fails it, listing every problem. It also refuses when another
file in the corpus already seeds the same MPN, since one MPN in two files fails every load. A draft
promoted before is written over its own earlier file.

  agnids promote datasheets/ti/LM1117.partspec.json --to params/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to <params dir> is required: promotion writes into a corpus")
			}
			draft, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			if fi, err := os.Stat(to); err != nil || !fi.IsDir() {
				return fmt.Errorf("--to %q is not a directory", to)
			}
			fsys := os.DirFS(to)
			prev, err := readIndex(fsys)
			if err != nil {
				return err
			}
			p, err := corpus.Promote(draft, fsys, prev)
			if err != nil {
				return err
			}
			dst := filepath.Join(to, filepath.FromSlash(p.File))
			prevText, prevErr := os.ReadFile(dst) // a replaced file is restored, a new one removed
			if err := writeAtomic(dst, p.Text); err != nil {
				return err
			}
			if _, err := param.LoadSet(fsys); err != nil {
				if prevErr == nil {
					_ = writeAtomic(dst, prevText)
				} else {
					_ = os.Remove(dst)
				}
				return fmt.Errorf("the corpus stopped loading after writing %s, so it was put back: %w", dst, err)
			}
			if err := writeIndex(to, p.Index); err != nil {
				return fmt.Errorf("%s was written but the index was not, so run `agnids index %s`: %w", dst, to, err)
			}
			verb := "promoted"
			if p.Replaces {
				verb = "updated"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s to %s (%d parameters, %d pins); index generation %d\n", verb, p.Spec.GetMpn(), dst, len(p.Spec.GetParameters()), len(p.Spec.GetPins()), p.Index.Generation)
			return nil
		},
	}
	c.Flags().StringVar(&to, "to", "", "the params corpus directory to write into (required)")
	return c
}

// indexCmd rebuilds a corpus's index from its files, which are the source of truth. It is what to run
// after editing a published spec by hand, and --check is what a corpus repository's CI runs to catch
// an edit that skipped it.
func indexCmd() *cobra.Command {
	var check bool
	c := &cobra.Command{
		Use:   "index <params dir>",
		Short: "Rebuild a corpus's index from its spec files, or check that it is current",
		Long: `Walk a published corpus, validate every spec as LoadSet does, and rewrite its index
(` + corpus.IndexFile + `). The generation advances only when the entries changed. A file that does not
validate, or two files claiming one MPN, fail the rebuild and leave the index as it was.

With --check, write nothing and exit non-zero when the index is missing or does not match the files,
listing each MPN that differs.

  agnids index params/
  agnids index params/ --check`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
				return fmt.Errorf("%q is not a directory", dir)
			}
			fsys := os.DirFS(dir)
			prev, err := readIndex(fsys)
			if err != nil {
				return err
			}
			next, changed, err := corpus.Refresh(fsys, prev)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if check {
				if prev == nil {
					return fmt.Errorf("%s has no %s; run `agnids index %s`", dir, corpus.IndexFile, dir)
				}
				if changed {
					return fmt.Errorf("%s is stale; run `agnids index %s`:\n  %s", corpus.IndexFile, dir, strings.Join(corpus.Diff(prev, next), "\n  "))
				}
				fmt.Fprintf(out, "%s is current: %d specs, generation %d\n", corpus.IndexFile, len(next.Entries), next.Generation)
				return nil
			}
			if !changed {
				fmt.Fprintf(out, "%s unchanged: %d specs, generation %d\n", corpus.IndexFile, len(next.Entries), next.Generation)
				return nil
			}
			if err := writeIndex(dir, next); err != nil {
				return err
			}
			fmt.Fprintf(out, "%s updated: %d specs, generation %d\n", corpus.IndexFile, len(next.Entries), next.Generation)
			for _, d := range corpus.Diff(prev, next) {
				fmt.Fprintf(out, "  %s\n", d)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&check, "check", false, "write nothing, and exit non-zero when the index is missing or stale")
	return c
}

// readIndex reads the corpus's index, treating a corpus never indexed as having none.
func readIndex(fsys fs.FS) (*corpus.Index, error) {
	ix, err := corpus.Read(fsys)
	if errors.Is(err, corpus.ErrNoIndex) {
		return nil, nil
	}
	return ix, err
}

// writeIndex writes the index into the corpus directory.
func writeIndex(dir string, ix *corpus.Index) error {
	b, err := ix.Marshal()
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, corpus.IndexFile), b)
}

// writeAtomic replaces path with data through a temporary file in the same directory and a rename,
// so a reader of the corpus sees the old file or the new one and never half of either.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once the rename has happened
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
