package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/panyam/agni/check"
	"github.com/panyam/agni/intake"
	"github.com/panyam/agni/datasheet/param"
	"github.com/spf13/cobra"
)

// intakeCmd is the CLI wrapper over the intake library: load a design, build the sanitized Skeleton, and
// render it. The sanitization guarantee lives in the intake.Skeleton type, not here — this command only
// picks the model constructor (with/without a params tier) and the output format.
func intakeCmd() *cobra.Command {
	var paramsDir, format, parts string
	c := &cobra.Command{
		Use:   "intake <file>",
		Short: "Extract a SANITIZED design summary (parts, classes, rails-by-nominal, anomalies) safe to cross the confidentiality boundary",
		Long: `Extract the deterministic, sanitized skeleton of a design — aggregates, a query-derived class
summary, the AVL/BOM parts table, rail NOMINALS (net names withheld), and anomaly counts. The output
holds no net name or topology by construction (the intake.Skeleton type has no field for one), so it is
safe to commit and cross the confidentiality boundary (CONSTRAINTS C16). This is the deterministic first
half of design onboarding; the /design-intake skill runs it and then layers judgment (interface
role-naming, anomaly narration) on top. Pass --params to populate the MPN and datasheet-gap columns.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := readDesign(args[0])
			if err != nil {
				return err
			}
			var m check.Model
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return err
				}
				m = check.NewModelWithParams(d, nil, set)
			} else {
				m = check.NewModel(d)
			}
			s := intake.Build(m)
			full := parts == "full"
			switch format {
			case "json":
				if !full {
					s.Parts = nil // default json carries the type-BOM (part_types); the full per-component list only with --parts full
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(s)
			case "", "md", "markdown":
				fmt.Fprint(cmd.OutOrStdout(), intake.Markdown(s, full))
				return nil
			default:
				return fmt.Errorf("unknown --format %q (want md|json)", format)
			}
		},
	}
	c.Flags().StringVar(&paramsDir, "params", "", "datasheet corpus dir (PartSpec textprotos) — enables the MPN + datasheet-gap columns")
	c.Flags().StringVar(&format, "format", "md", "output format: md | json")
	c.Flags().StringVar(&parts, "parts", "types", "parts view: types (BOM by distinct part type, default) | full (per-component AVL)")
	return c
}
