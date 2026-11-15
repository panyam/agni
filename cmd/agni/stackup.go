package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/stackup"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// stackupCmd prints a design's board stack, top to bottom, as the board file states it (agni
// 1592). Each row gives its kind, thickness, material and electrical values, and a value the file
// does not give reads as unknown or blank. The text form ends with the board thickness the file
// states and whether the layers add up to it. --format json is the board geometry's Stackup message.
func stackupCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "stackup <design>",
		Short: "Print a board's layer stack: each layer's kind, thickness and material",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("--format %s: want text or json", format)
			}
			ws, err := workspace()
			if err != nil {
				return err
			}
			src, err := newDesignResolver(ws).Resolve(context.Background(), args[0])
			if err != nil {
				return err
			}
			noteSource(os.Stderr, src)
			b, err := newLoader().BoardGeometry(localOf(src.BoardURI))
			if err != nil {
				return err
			}
			if b == nil {
				return fmt.Errorf("%s has no board, so it has no stack", args[0])
			}
			w := cmd.OutOrStdout()
			if format == "json" {
				s := b.GetStackup()
				if s == nil {
					s = &geom.Stackup{}
				}
				out, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(s)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(w, string(out))
				return err
			}
			return writeStackup(w, b)
		},
	}
	c.Flags().StringVar(&format, "format", "text", "text (a table) or json (the board geometry's Stackup message)")
	return c
}

// writeStackup writes the stack as a table, then one line on the board's total thickness.
func writeStackup(w io.Writer, b *geom.BoardGeometry) error {
	s, unit := b.GetStackup(), b.GetUnitNm()
	if len(s.GetLayers()) == 0 && s.GetThickness() == 0 {
		_, err := fmt.Fprintln(w, "The board file states no stack.")
		return err
	}
	if len(s.GetLayers()) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "LAYER\tKIND\tTHICKNESS\tMATERIAL\tTYPE\tER\tTAN D")
		for _, l := range s.GetLayers() {
			thick := "unknown"
			if l.Thickness != nil {
				thick = stackup.MM(l.GetThickness(), unit) + " mm"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", l.GetName(), l.GetKind(), thick, l.GetMaterial(),
				l.GetDielectricType(), optFloat(l.EpsilonR), optFloat(l.LossTangent))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	sum, missing := stackup.Sum(s)
	line := "The file states no board thickness."
	if s.Thickness != nil {
		line = "The file states a board thickness of " + stackup.MM(s.GetThickness(), unit) + " mm."
	}
	switch {
	case len(s.GetLayers()) == 0:
	case missing > 0:
		line += fmt.Sprintf(" %d copper or dielectric layers have no thickness, so the layers have no total.", missing)
	case stackup.Check(s, unit) != "":
		line += " The layers sum to " + stackup.MM(sum, unit) + " mm, more than " +
			strconv.Itoa(int(stackup.Tolerance*100)) + "% away from it."
	default:
		line += " The layers sum to " + stackup.MM(sum, unit) + " mm."
	}
	_, err := fmt.Fprintln(w, "\n"+line)
	return err
}

func optFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
