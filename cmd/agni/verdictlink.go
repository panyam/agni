package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/panyam/agni/artifact"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mounts"
)

// serverMountTimeout bounds the one call --url-base makes. A wrong address must cost a moment, not
// the run: the links are a convenience on top of an analysis that has already finished.
const serverMountTimeout = 2 * time.Second

// linkTarget is the path half of a viewer URL, plus the reason there is none.
//
// The rule it applies is unchanged from the linkablePath it replaces: a link is only promised for a
// mount THE OPERATOR NAMED, because a mount the CLI minted locally means nothing on a server the
// operator did not start with it (agni issue 392), and the question is whether the mount was
// declared rather than how the argument was spelled (agni issue 459).
//
// What is new is the second return. Refusing to link is the right answer, but refusing silently is
// not: a report with 265 rows and no links looks like a broken renderer, and nothing on the page or
// in the terminal said which of the two halves was missing. The caller prints this whenever
// --url-base was given and nothing came back.
func linkTarget(ws *cliWorkspace, designURI string) (path, why string) {
	if ws == nil {
		return "", "this run has no mount table"
	}
	u, err := artifact.Parse(designURI)
	if err != nil || u.Mount == "" || u.Path == "" {
		return "", fmt.Sprintf("%s does not address a mount", designURI)
	}
	if !ws.Declared(u.Mount) {
		return "", fmt.Sprintf("mount %q was minted for this run rather than declared, so a link built from it would resolve on no server; pass --mount %s=<root> to name it", u.Mount, u.Mount)
	}
	return u.Mount + "/" + u.Path, ""
}

// verifyServerMount asks the server at urlBase whether it serves this mount from the same root.
//
// linkablePath could only ever check that the OPERATOR named the mount, never that the SERVER agrees
// about it, and the two are different claims. `--mount gateway=/a` against a server started with
// `--mount gateway=/b` passed every local check and emitted links that resolve to a different board,
// which is worse than emitting none: the reader has no reason to doubt a link that loads. The
// mitigation named in the old comment was the content hash on the URL. That now works end to end (the
// URL carries it and the viewer compares it), so a mount table that disagrees is caught twice: here
// when the server can be reached, and in the browser when it cannot.
//
// UNREACHABLE IS NOT MISMATCHED. A server that does not answer leaves the question open, and the
// report may well be generated now and read once the viewer is up, so an unreachable address keeps
// the links and says the table went unverified. A server that answers and disagrees is a definite
// broken promise, and those links are dropped.
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

// mountURIAuthority is the mount name in an artifact URI, or "" when it names none. It exists so the
// link site can look the mount's ROOT back up: linkTarget answers whether to link and returns the
// path, but verifying against the server needs the local root to compare, which only the mount table
// carries.
func mountURIAuthority(designURI string) string {
	u, err := artifact.Parse(designURI)
	if err != nil {
		return ""
	}
	return u.Mount
}

// verdictLinkTarget resolves the design ONCE and returns both halves of the link built from it: the
// viewer path, and the revision that path is at.
//
// One function because the two used to be two, and they disagreed (agni issue 489). The path came
// from the caller's ARGUMENT and the hash came from the resolved ENTRY, which is a difference with no
// symptom until the argument is not the entry. Then it has two, both bad:
//
//   - A design FOLDER produced `/designs/<mount>/<dir>/view`, which the viewer's URL space reads as
//     the FILE at <dir>. GetDesign refuses it and the page loads no design at all. That is the form
//     the tutorial teaches.
//   - A declared COMPANION produced its own path with the ENTRY's hash, so the viewer compared two
//     different files and drew the stale-link banner over a design that was perfectly in sync. A
//     false alarm in the mechanism built to prevent false confidence is worse than the silence it
//     replaced.
//
// Resolving once is what makes those unrepresentable rather than merely fixed. There is no longer a
// pair of values that could name different artifacts.
func verdictLinkTarget(ctx context.Context, ws *cliWorkspace, ll *localLoader, designURI string) (mountPath, contentHash, why string) {
	target := designURI
	// A resolution failure leaves the argument standing rather than dropping the link. The design was
	// already read and analysed by the time anything asks for a link, so a descriptor that will not
	// resolve here is a surprise about CONFIG, not a reason to withhold the answer.
	if ll != nil {
		if u, err := artifact.Parse(designURI); err == nil {
			if e, err := ll.designEntry(ctx, u); err == nil {
				target = e.String()
			}
		}
	}
	mountPath, why = linkTarget(ws, target)
	return mountPath, designContentHash(ctx, ll, target), why
}

// designContentHash is the hash of the bytes a run actually read, for the staleness signal on a
// verdict link (issue 392).
//
// It goes through the loader's DesignHash rather than hashing the caller's argument, because the
// argument may name a design FOLDER or a companion view, and neither is the file that was analysed.
// Hashing the argument gave "" for a folder, since opening a directory succeeds and reading it fails,
// and hashSource reports a read failure and a genuinely unhashable file the same way.
//
// An error still yields "", which is what DesignRef.content_hash documents for a producer that did
// not hash. A link without the staleness parameter is worse than one with it and better than no link,
// and this is not the place to fail a run that has already finished its analysis.
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
