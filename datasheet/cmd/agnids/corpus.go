package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/datasheet/corpus"
	"github.com/panyam/agni/datasheet/dsservice"
)

// publishCmd publishes a draft from the corpus's store by MPN (agni issues 747, 749), the scripted
// form of the workbench's PublishDraft. It validates the draft, refuses one that fails (listing every
// problem) or whose MPN another published file already seeds, and otherwise writes <mpn>.textproto
// and the index. The draft stays, as the start of the next edit.
func publishCmd() *cobra.Command {
	var dir string
	c := &cobra.Command{
		Use:   "publish <mpn>",
		Short: "Validate a draft and make it the published spec for its MPN",
		Long: `Publish the draft for an MPN from a corpus's store, where the workbench saves drafts, into
the same corpus's published specs, where checks and PartSpecService read them, and record it in the
index (` + corpus.IndexFile + `).

A draft is saved without validation so work is never lost, and no check reads one. Publishing runs
param.Validate and refuses a draft that fails it, listing every problem. It also refuses when another
published file already seeds the same MPN, since one MPN in two files fails every load. Publishing
an MPN again replaces its earlier published spec.

  agnids publish LM1117 --corpus params/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDir("--corpus", dir); err != nil {
				return err
			}
			p, err := newOSDraftStore(dir).Publish(cmd.Context(), args[0])
			var refused *dsservice.PublishRefused
			if errors.As(err, &refused) {
				var b strings.Builder
				b.WriteString(refused.Reason)
				for _, pr := range refused.Problems {
					fmt.Fprintf(&b, "\n  [%s] %s", pr.Kind, pr.Message)
				}
				return errors.New(b.String())
			}
			if err != nil {
				return err
			}
			verb := "published"
			if p.Replaced {
				verb = "republished"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s; index generation %d\n", verb, args[0], p.Generation)
			return nil
		},
	}
	c.Flags().StringVar(&dir, "corpus", "", "the corpus directory holding the draft and the published specs (required)")
	return c
}

// requireDir reports a flag that is unset or does not name a directory.
func requireDir(flag, dir string) error {
	if dir == "" {
		return fmt.Errorf("%s <dir> is required", flag)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s %q is not a directory", flag, dir)
	}
	return nil
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
