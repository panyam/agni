package main

import (
	"github.com/spf13/cobra"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// diffViaService answers `diff --format json` with the DiffDesignsResponse the DiffDesigns rpc
// returns, built by the same service over the CLI's local loader, so the CLI and the web API emit
// one message with the same content, sheet and placement maps included (agni issue 737).
// EmitUnpopulated keeps empty lists and maps present, so a no-change diff is still a well-formed
// object rather than fields that appear and vanish per run.
func diffViaService(cmd *cobra.Command, a, b string, renameApprox, includeEqual bool) error {
	aURI, err := cliArgURI(a)
	if err != nil {
		return err
	}
	bURI, err := cliArgURI(b)
	if err != nil {
		return err
	}
	svc := service.NewDiffService(&localLoader{loader: newLoader()}, cliProjects())
	req := &webapi.DiffDesignsRequest{AUri: aURI, BUri: bURI, IncludeEqual: includeEqual}
	if renameApprox {
		req.NearRenames = &webapi.NearRenameOptions{} // the calibrated thresholds
	}
	resp, err := svc.DiffDesigns(cmd.Context(), req)
	if err != nil {
		return err
	}
	out, err := protoJSON(resp)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write([]byte(out))
	return err
}
