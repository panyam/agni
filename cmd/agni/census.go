package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/census"
)

// censusCmd is the corpus-facing half of the element-coverage guard (WS6-011): it walks design
// files and reports every source construct NOT classified in its format's census manifest. The
// committed-fixture half runs in CI (census package test); this half runs over the private corpus
// (`make census`), which CI can never see, to surface real-world constructs the fixtures lack so a
// human classifies them. Report-only by default; exits non-zero when unclassified constructs are
// found so a gate can catch drift, matching `agni validate`'s posture.
func censusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "census <file|dir>...",
		Short: "Report source constructs not classified in the reader-coverage census manifests",
		Long: "census enumerates the constructs in each design file and reports any not classified\n" +
			"in its format's manifest (census package). It is the discovery half of WS6-011: the\n" +
			"CI test guards the committed fixtures, and this guards a private corpus the fixtures\n" +
			"cannot cover. Directories are walked recursively; files with no manifest are skipped.\n" +
			"Exits non-zero if any construct is unclassified.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Aggregate unclassified tokens per format, tracking one example file each.
			type key struct{ format, token string }
			examples := map[key]string{}
			counts := map[key]int{}
			scanned := 0
			scan := func(path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					return
				}
				m, ok := census.Lookup(path, data)
				if !ok {
					return
				}
				scanned++
				for _, u := range m.Audit(map[string][]byte{path: data}) {
					k := key{m.Format, u.Token}
					if counts[k] == 0 {
						examples[k] = path
					}
					counts[k]++
				}
			}
			for _, arg := range args {
				info, err := os.Stat(arg)
				if err != nil {
					return err
				}
				if info.IsDir() {
					filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
						if err == nil && !d.IsDir() {
							scan(path)
						}
						return nil
					})
				} else {
					scan(arg)
				}
			}
			var keys []key
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				if keys[i].format != keys[j].format {
					return keys[i].format < keys[j].format
				}
				return keys[i].token < keys[j].token
			})
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "census: scanned %d classifiable file(s), %d unclassified construct(s)\n", scanned, len(keys))
			for _, k := range keys {
				fmt.Fprintf(out, "  [%s] %-24s x%-4d e.g. %s\n", k.format, k.token, counts[k], examples[k])
			}
			if len(keys) > 0 {
				return fmt.Errorf("%d unclassified construct(s) — classify each in its census manifest", len(keys))
			}
			return nil
		},
	}
	return cmd
}
