package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/datasheet/derive"
	"github.com/panyam/agni/datasheet/doc"
	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
)

// deriveCmd derives a parameter-IR PartSpec from a doc-IR file: the CLI face of the
// derivation stage (docs/24). Thin wiring per C13: file opening and os.DirFS happen
// here; the derive package is pure data-in data-out.
func deriveCmd() *cobra.Command {
	var recipesDir, patchesDir, mpn, manufacturer, deviceClass, outSpec, outManifest string
	cmd := &cobra.Command{
		Use:   "derive <doc-ir.textproto>",
		Short: "Derive a parameter-IR PartSpec from a doc-IR document decomposition",
		Long: "Derive a PartSpec from a doc-IR file (produced by a document parser such as " +
			"tools/pdf2doc): classify tables through --recipes, apply --patches last, tokenize rows " +
			"into parameters with provenance, and write the spec plus its run manifest (the " +
			"lockfile: pinned inputs and the gap list of everything seen but not extracted). " +
			"--mpn names the part being derived (the doc-IR does not know; the operator does).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			d, err := doc.Load(f)
			if err != nil {
				return err
			}
			if err := doc.Validate(d); err != nil {
				return fmt.Errorf("input doc-IR invalid: %w", err)
			}
			recipes, err := derive.LoadRecipes(os.DirFS(recipesDir))
			if err != nil {
				return fmt.Errorf("--recipes %s: %w", recipesDir, err)
			}
			var patches []*derivepb.Patch
			if patchesDir != "" {
				patches, err = derive.LoadPatches(os.DirFS(patchesDir))
				if err != nil {
					return fmt.Errorf("--patches %s: %w", patchesDir, err)
				}
			}
			spec, manifest, err := derive.Run(d, recipes, patches, derive.Identity{
				MPN: mpn, Manufacturer: manufacturer, DeviceClass: deviceClass,
			})
			if err != nil {
				return err
			}
			specText, err := derive.MarshalSpec(spec)
			if err != nil {
				return err
			}
			if err := os.WriteFile(outSpec, specText, 0o644); err != nil {
				return err
			}
			if outManifest != "" {
				mText, err := derive.MarshalManifest(manifest)
				if err != nil {
					return err
				}
				if err := os.WriteFile(outManifest, mText, 0o644); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d parameters (recipes: %v, patches applied: %d, gaps: %d)\n",
				outSpec, manifest.ParametersEmitted, manifest.Recipes, len(manifest.PatchesApplied), len(manifest.Gaps))
			for _, g := range manifest.Gaps {
				fmt.Fprintf(cmd.OutOrStdout(), "  gap [%s] %s %s\n", g.Kind, g.Region, g.Detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&recipesDir, "recipes", "", "directory of Recipe textprotos (required)")
	cmd.Flags().StringVar(&patchesDir, "patches", "", "directory of Patch textprotos (pinned human corrections, applied last)")
	cmd.Flags().StringVar(&mpn, "mpn", "", "MPN of the part this document describes (required)")
	cmd.Flags().StringVar(&manufacturer, "manufacturer", "", "manufacturer name for the emitted spec")
	cmd.Flags().StringVar(&deviceClass, "device-class", "", "free-form device class hint (ldo, nfet, mcu)")
	cmd.Flags().StringVar(&outSpec, "out", "spec.textproto", "output PartSpec path")
	cmd.Flags().StringVar(&outManifest, "manifest", "", "output RunManifest path (omit to skip)")
	_ = cmd.MarkFlagRequired("recipes")
	_ = cmd.MarkFlagRequired("mpn")
	return cmd
}
