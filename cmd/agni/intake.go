package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/intake"
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
role-naming, anomaly narration) on top. The MPN and datasheet-gap columns need a parameter set, which a
design inside a project gets from the project's own params/ with no flag; --params names one for a
design that belongs to no project, or overrides nothing when the project already declares one.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, ov, err := readDesignWithConfig(args[0])
			if err != nil {
				return err
			}
			// The corpus comes from the design's PROJECT first and the flag second, which is the
			// precedence check already uses (Overlay.SpecsOr). Before this, intake read --params
			// alone, so inside a project declaring params/ you had to name a directory the project
			// already names, and forgetting it produced a report whose datasheet-gap section was
			// absent rather than empty (agni issue 474).
			var flagSpecs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %s: %w", paramsDir, err)
				}
				flagSpecs = set
			}
			// One constructor for both cases: NewModelWithParams guards a nil provider, so a design
			// with no corpus builds the same model NewModel would have.
			s := intake.Build(check.NewModelWithParams(d, nil, ov.SpecsOr(flagSpecs)))
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
	c.Flags().StringVar(&paramsDir, "params", "", "datasheet corpus dir (PartSpec textprotos) enabling the MPN + datasheet-gap columns, for a design that belongs to no project. A design inside a project reads the params/ that project declares, which wins over this flag")
	c.Flags().StringVar(&format, "format", "md", "output format: md | json")
	c.Flags().StringVar(&parts, "parts", "types", "parts view: types (BOM by distinct part type, default) | full (per-component AVL)")
	return c
}
