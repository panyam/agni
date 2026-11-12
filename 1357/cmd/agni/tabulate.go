package main

import (
	"fmt"
	"io"
	"os"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

// tabulateCmd is the CLI face of TableService.Tabulate (agni issue 862), so a client on the CLI
// transport gets the engine's projection of an answer rather than deriving its own rows.
func tabulateCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "tabulate <request.json|->",
		Short: "Project an answer agni returned into tables, the rows every csv and workbook carries",
		Long: "Project an answer into tables. The input is a TabulateRequest as protojson, carrying the " +
			"answer an analysis command printed with --format json (a check run, a query, a query set) " +
			"and optionally order_by and column_types. The output is the TabulateResponse, the same " +
			"tables `check --format csv` and `query --format csv` encode, so a client writing a " +
			"spreadsheet carries the engine's rows and columns rather than deriving its own.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "json" {
				return fmt.Errorf("unknown --format %q (want: json)", format)
			}
			var in io.Reader = cmd.InOrStdin()
			if args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return err
				}
				defer f.Close()
				in = f
			}
			b, err := io.ReadAll(in)
			if err != nil {
				return err
			}
			var req webapi.TabulateRequest
			if err := protojson.Unmarshal(b, &req); err != nil {
				return fmt.Errorf("the input is not a TabulateRequest: %w", err)
			}
			resp, err := service.TableService{}.Tabulate(cmd.Context(), &req)
			if err != nil {
				return err
			}
			out, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(resp)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return err
		},
	}
	c.Flags().StringVar(&format, "format", "json", "output format: json (the TabulateResponse the Tabulate rpc returns)")
	return c
}
