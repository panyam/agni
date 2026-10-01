package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/panyam/agni/artifact"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mounts"

	rpt "github.com/panyam/agni/core/report"
)

// serverMountTimeout bounds the one call --server makes, so a wrong address costs a moment rather
// than stalling a run whose analysis has already finished.
const serverMountTimeout = 2 * time.Second

// linkTarget is the path half of a viewer URL, plus the reason there is none.
//
// A link is only promised for a mount THE OPERATOR NAMED, because a mount the CLI minted locally
// means nothing on a server the operator did not start with it (agni issue 392). The test is whether
// the mount was declared, not how the argument was spelled (agni issue 459).
//
// The caller prints why whenever --server was given and no path came back, since a 265-row report
// with no links otherwise looks like a broken renderer (#476).
func linkTarget(ws *cliWorkspace, designURI string, selfServed bool) (path, why string) {
	if ws == nil {
		return "", "this run has no mount table"
	}
	u, err := artifact.Parse(designURI)
	if err != nil || u.Mount == "" || u.Path == "" {
		return "", fmt.Sprintf("%s does not address a mount", designURI)
	}
	// Under `--server self` this process serves the table it just minted, so a minted mount is
	// linkable and the declared-mount rule does not apply.
	if !selfServed && !ws.Declared(u.Mount) {
		return "", fmt.Sprintf("mount %q was minted for this run rather than declared, so a link built from it would resolve on no server; pass --mount %s=<root> to name it, or --server self to serve it from here", u.Mount, u.Mount)
	}
	return u.Mount + "/" + u.Path, ""
}

// verifyServerMount asks the server at urlBase whether it serves this mount from the same root.
//
// linkTarget checks only that the OPERATOR named the mount. This checks that the SERVER agrees, since
// `--mount gateway=/a` against a server started with `--mount gateway=/b` passes every local check
// and emits links that load a different board. The content hash on the URL catches the same mismatch
// in the browser when the server cannot be reached here.
//
// UNREACHABLE IS NOT MISMATCHED. A report may be generated now and read once the viewer is up, so an
// unreachable server keeps the links with a note that the table went unverified. A server that
// answers and disagrees gets its links dropped.
func verifyServerMount(ctx context.Context, urlBase string, want mounts.Mount) (ok bool, why string) {
	ctx, cancel := context.WithTimeout(ctx, serverMountTimeout)
	defer cancel()
	client := webapiconnect.NewWorkspaceServiceClient(&http.Client{Timeout: serverMountTimeout}, urlBase)
	resp, err := client.ListMounts(ctx, connect.NewRequest(&webapi.ListMountsRequest{}))
	if err != nil {
		return true, fmt.Sprintf("could not reach %s to confirm it serves mount %q, so the links are unverified", urlBase, want.Name)
	}
	for _, m := range resp.Msg.GetMounts() {
		if m.GetName() != want.Name {
			continue
		}
		if m.GetRoot() == want.Root {
			return true, ""
		}
		return false, fmt.Sprintf("%s serves mount %q from %s, not %s, so every link would name a different design", urlBase, want.Name, m.GetRoot(), want.Root)
	}
	return false, fmt.Sprintf("%s serves no mount named %q, so every link would resolve to nothing", urlBase, want.Name)
}

// mountURIAuthority is the mount name in an artifact URI, or "" when it names none. The link site
// uses it to look the mount's ROOT back up in the mount table for verifyServerMount.
func mountURIAuthority(designURI string) string {
	u, err := artifact.Parse(designURI)
	if err != nil {
		return ""
	}
	return u.Mount
}

// verdictLinkTarget resolves the design ONCE and returns both halves of the link built from it, the
// viewer path and the revision that path is at, so the two always name the same artifact (agni
// issue 489).
//
// Built separately, the path came from the ARGUMENT and the hash from the resolved ENTRY. A design
// FOLDER then linked to `/designs/<mount>/<dir>/view`, which the viewer reads as a file and refuses,
// and a declared COMPANION got the entry's hash, which drew the stale-link banner over an in-sync
// design.
func verdictLinkTarget(ctx context.Context, ws *cliWorkspace, ll *localLoader, designURI string, selfServed bool) (mountPath, contentHash, why string) {
	target := designURI
	// A resolution failure leaves the argument standing rather than dropping the link, because the
	// design has already been read and analysed by now.
	if ll != nil {
		if u, err := artifact.Parse(designURI); err == nil {
			if e, err := ll.designEntry(ctx, u); err == nil {
				target = e.String()
			}
		}
	}
	mountPath, why = linkTarget(ws, target, selfServed)
	return mountPath, designContentHash(ctx, ll, target), why
}

// designContentHash is the hash of the bytes a run actually read, for the staleness signal on a
// verdict link (issue 392).
//
// It goes through the loader's DesignHash rather than hashing the caller's argument, because the
// argument may name a design FOLDER or a companion view, and neither is the file that was analysed
// (#479).
//
// An error yields "", which is what DesignRef.content_hash documents for a producer that did not
// hash. The link then carries no staleness parameter, and the finished run does not fail.
func designContentHash(ctx context.Context, ll *localLoader, designURI string) string {
	u, err := artifact.Parse(designURI)
	if err != nil {
		return ""
	}
	h, err := ll.DesignHash(ctx, u)
	if err != nil {
		return ""
	}
	return h
}

// viewerLinkMeta resolves the LINK half of a run, meaning where a viewer would serve this design, the
// bytes the run read, and whether the server at urlBase agrees about the mount. `check` and `trace`
// both call it, so the rule for when a link is promised lives only here.
//
// Three refusals, all fail-closed and all reported on stderr and in LinksWithheld when urlBase was
// given (agni issue 626). They are a workspace that failed to build, a design reached through a mount
// the CLI minted rather than one the operator named, and a server that serves that mount name from a
// different root.
//
// It returns an rpt.Report because that is what the renderers take. A caller wanting only the URL
// halves reads URLBase and MountPath off it.
func viewerLinkMeta(cmd *cobra.Command, ctx context.Context, ll *localLoader, designURI string, spec serverSpec) rpt.Report {
	ws, _ := workspace()
	urlBase := spec.base()
	mountPath, contentHash, why := verdictLinkTarget(ctx, ws, ll, designURI, spec.self)
	withheld := ""
	if urlBase != "" && why != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --server is set but no links were emitted: %s\n", why)
		withheld = why
	}
	// Only a remote server is asked. Under `self` the table the links name is the one this process
	// is about to serve.
	if spec.url != "" && mountPath != "" {
		if m, ok := mounts.Find(ws.Mounts(), mountURIAuthority(designURI)); ok {
			keep, note := verifyServerMount(ctx, urlBase, m)
			if note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", note)
			}
			if !keep {
				mountPath = ""
				// Carried into the report as well as stderr, since a reader of the saved page cannot
				// guess a root mismatch from the page itself.
				withheld = note
			}
		}
	}
	return rpt.Report{
		Design:        designURI,
		ContentHash:   contentHash,
		URLBase:       urlBase,
		MountPath:     mountPath,
		LinksWithheld: withheld,
	}
}
