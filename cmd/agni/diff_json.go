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
func diffViaService(cmd *cobra.Command, a, b, remote string, renameApprox, includeEqual bool) error {
	resp, err := diffResponse(cmd, a, b, remote, renameApprox, includeEqual)
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

// diffResponse asks the DiffDesigns rpc for the diff of a and b, over the CLI's local loader or, with
// remote, on that server (agni issue 1695). The json and the csv both read it, so neither composes the
// diff a second way. A server reads a and b from its own copies when they name its designs,
// designs/{d}, and from the files sent with the request otherwise.
func diffResponse(cmd *cobra.Command, a, b, remote string, renameApprox, includeEqual bool) (*webapi.DiffDesignsResponse, error) {
	addr := cliArgURI
	var snaps snapshotAPI
	var svc diffAPI
	if remote != "" {
		r, err := dialRemote(cmd.Context(), remote)
		if err != nil {
			return nil, err
		}
		addr, snaps, svc = r.uri, r.snapshots(), r.diff()
	} else {
		ll := &localLoader{loader: newLoader()}
		snaps, svc = service.NewSnapshotService(ll, cliProjects(), nil, ""), service.NewDiffService(ll, cliProjects())
	}
	aURI, err := addr(a)
	if err != nil {
		return nil, err
	}
	bURI, err := addr(b)
	if err != nil {
		return nil, err
	}
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
	req := &webapi.DiffDesignsRequest{ASnapshot: aSnap, BSnapshot: bSnap, IncludeEqual: includeEqual}
	if renameApprox {
		req.NearRenames = &webapi.NearRenameOptions{} // the calibrated thresholds
	}
	resp, err := svc.DiffDesigns(cmd.Context(), req)
	return resp, rpcFailed(err)
}
