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
	resp, err := diffResponse(cmd, a, b, renameApprox, includeEqual)
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

// diffResponse asks the DiffDesigns rpc, over the CLI's local loader, for the diff of a and b. The
// json and the csv both read it, so neither composes the diff a second way.
func diffResponse(cmd *cobra.Command, a, b string, renameApprox, includeEqual bool) (*webapi.DiffDesignsResponse, error) {
	aURI, err := cliArgURI(a)
	if err != nil {
		return nil, err
	}
	bURI, err := cliArgURI(b)
	if err != nil {
		return nil, err
	}
	ll := &localLoader{loader: newLoader()}
	snaps := service.NewSnapshotService(ll, cliProjects(), nil, "")
	// Each side is a snapshot of its own, resolved as the text diff resolves it (C32).
	aSnap, dropA, err := ingest(cmd.Context(), snaps, &webapi.Snapshot{Uri: aURI, AsNamed: readAsNamed})
	if err != nil {
		return nil, rpcFailed(err)
	}
	defer dropA()
	bSnap, dropB, err := ingest(cmd.Context(), snaps, &webapi.Snapshot{Uri: bURI, AsNamed: readAsNamed})
	if err != nil {
		return nil, rpcFailed(err)
	}
	defer dropB()
	svc := service.NewDiffService(ll, cliProjects())
	req := &webapi.DiffDesignsRequest{ASnapshot: aSnap, BSnapshot: bSnap, IncludeEqual: includeEqual}
	if renameApprox {
		req.NearRenames = &webapi.NearRenameOptions{} // the calibrated thresholds
	}
	resp, err := svc.DiffDesigns(cmd.Context(), req)
	return resp, rpcFailed(err)
}
