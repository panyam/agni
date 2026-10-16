package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mounts"
)

// checklistLinks settles whether a review's page may promise links, and where they point. The page
// itself is RenderReviewReport's (agni issue 1030), so this decides only what the rpc is told.
//
// It applies the rule `check --server` applies by calling the same two functions, verdictLinkTarget
// and verifyServerMount, so the two surfaces cannot disagree about what a link means. The mount has to
// be one the operator DECLARED, and the server has to agree it serves that name from the same root.
func checklistLinks(cmd *cobra.Command, ll *localLoader, designArg string, spec serverSpec) (*webapi.ReportLinks, error) {
	designURI, err := cliArgURI(designArg)
	if err != nil {
		return nil, err
	}
	ws, _ := workspace()
	urlBase := spec.base()
	mountPath, _, why := verdictLinkTarget(cmd.Context(), ws, ll, designURI, spec.self)
	withheld := ""
	if urlBase != "" && why != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --server is set but no findings were linked: %s\n", why)
		withheld = why
	}
	if urlBase != "" && mountPath != "" {
		if m, ok := mounts.Find(ws.Mounts(), mountURIAuthority(designURI)); ok {
			keep, note := verifyServerMount(cmd.Context(), urlBase, m)
			if note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", note)
			}
			if !keep {
				mountPath = ""
				withheld = note
			}
		}
	}
	return &webapi.ReportLinks{UrlBase: urlBase, DesignUri: linkURI(mountPath), Withheld: withheld}, nil
}
