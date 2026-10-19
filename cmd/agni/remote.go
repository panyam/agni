package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/panyam/agni/artifact"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"google.golang.org/protobuf/types/known/emptypb"
)

// remoteMountTimeout bounds the ListMounts call --remote makes before anything else, so a wrong
// address fails in a moment rather than when the analysis request times out.
const remoteMountTimeout = 5 * time.Second

// remoteFlag adds --remote to a command whose answer comes from one service request (agni issue 998).
func remoteFlag(cmd *cobra.Command, remote *string) {
	cmd.Flags().StringVar(remote, "remote", "",
		"send this command's request to the agni serve at this URL instead of reading the design here, "+
			"so a server that already holds the design answers from what it kept. The output is the "+
			"same; only where the engine ran changes. A path is addressed through the server's own "+
			"mounts, so the server must serve it, and a path it does not serve is an error rather "+
			"than a local read")
}

// remoteServer is a running `agni serve` this command sends its request to, with the mount table it
// reported, so a local path can be addressed as the server addresses it.
type remoteServer struct {
	url    string
	http   *http.Client
	mounts []*webapi.Mount
}

// dialRemote asks the server at raw for its mounts.
//
// UNREACHABLE IS AN ERROR HERE, unlike --server's link check (verifyServerMount), because --server
// only decides whether a link is printed while --remote is where the answer comes from. Falling back
// to a local read would answer from a different engine without saying so.
func dialRemote(ctx context.Context, raw string) (*remoteServer, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("--remote %q: want a URL like http://localhost:8080", raw)
	}
	r := &remoteServer{url: strings.TrimRight(raw, "/"), http: http.DefaultClient}
	ctx, cancel := context.WithTimeout(ctx, remoteMountTimeout)
	defer cancel()
	resp, err := webapiconnect.NewWorkspaceServiceClient(r.http, r.url).
		ListMounts(ctx, connect.NewRequest(&webapi.ListMountsRequest{}))
	if err != nil {
		return nil, fmt.Errorf("--remote %s: could not list the server's mounts: %w", r.url, err)
	}
	r.mounts = resp.Msg.GetMounts()
	return r, nil
}

// uri addresses a command-line argument through the server's mounts. An empty argument stays empty,
// as cliArgURI keeps an unsupplied optional flag unsupplied.
//
// An argument that is already a mount URI is sent as written, naming the server's mount. A path is
// made absolute and addressed through the server mount whose root contains it, the longest root
// winning as inDeclared does locally. That only works when the server reads the same filesystem
// this command does, which is the case --remote is for: a warm server on this machine.
func (r *remoteServer) uri(arg string) (string, error) {
	if arg == "" {
		return "", nil
	}
	if strings.Contains(arg, "://") {
		u, err := artifact.Parse(arg)
		if err != nil {
			return "", err
		}
		return u.String(), nil
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	abs = realPath(abs)
	var best *webapi.Mount
	var bestRel string
	for _, m := range r.mounts {
		root := realPath(filepath.Clean(m.GetRoot()))
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if best == nil || len(m.GetRoot()) > len(best.GetRoot()) {
			best, bestRel = m, rel
		}
	}
	if best == nil {
		names := make([]string, len(r.mounts))
		for i, m := range r.mounts {
			names[i] = m.GetName() + "=" + m.GetRoot()
		}
		return "", fmt.Errorf("--remote %s serves no mount containing %s (it serves %s); start it with --mount <name>=<dir> over that path", r.url, abs, strings.Join(names, ", "))
	}
	u, err := artifact.New(best.GetName(), filepath.ToSlash(bestRel))
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// realPath resolves symlinks where it can, so a path under /tmp and a root under /private/tmp (both
// spellings of one macOS directory) compare equal. A path that does not resolve is kept as given.
func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// refuseLocalOnly names the flags a --remote run cannot honour, because each one configures the
// engine THIS process would have built rather than riding the request. The server answers with the
// corpus, profiles and catalog it was started with, so passing one of these would read as applied
// while doing nothing.
//
// --timing, --timing-memory and --explain are refused for every command too, because they record the
// stages of this process, and under --remote nearly all the work is on the server.
func refuseLocalOnly(cmd *cobra.Command, flags ...string) error {
	for _, f := range []string{"timing", "timing-memory", "explain"} {
		if fl := cmd.Flags().Lookup(f); fl != nil && fl.Changed {
			return fmt.Errorf("--%s times this process, and --remote's work happens on the server; start it with --slow-request, or send the Agni-Timing header", f)
		}
	}
	for _, f := range flags {
		if fl := cmd.Flags().Lookup(f); fl != nil && fl.Changed {
			return fmt.Errorf("--%s configures the engine in this process, and --remote runs the server's; start the server with it instead", f)
		}
	}
	return nil
}

// The services a command calls, narrowed to the methods it uses. The in-process service satisfies
// each one, and so does the Connect client wrapper below, which is what keeps a remote answer the
// same message the local one is (C31, C32).
type (
	checkAPI interface {
		CheckDesign(context.Context, *webapi.CheckDesignRequest) (*webapi.CheckDesignResponse, error)
		GetCheckReport(context.Context, *webapi.GetCheckReportRequest) (*webapi.GetCheckReportResponse, error)
		RenderCheckReport(context.Context, *webapi.RenderCheckReportRequest) (*webapi.RenderCheckReportResponse, error)
	}
	queryAPI interface {
		RunQuery(context.Context, *webapi.RunQueryRequest) (*webapi.RunQueryResponse, error)
		RunQueries(context.Context, *webapi.RunQueriesRequest) (*webapi.RunQueriesResponse, error)
		ListQuerySets(context.Context, *webapi.ListQuerySetsRequest) (*webapi.ListQuerySetsResponse, error)
	}
	traceAPI interface {
		TraceDesign(context.Context, *webapi.TraceDesignRequest) (*webapi.TraceDesignResponse, error)
	}
	reviewAPI interface {
		CreateReview(context.Context, *webapi.CreateReviewRequest) (*webapi.Review, error)
		RenderReviewReport(context.Context, *webapi.RenderReviewReportRequest) (*webapi.RenderReviewReportResponse, error)
	}
	snapshotAPI interface {
		Ingest(context.Context, *webapi.IngestRequest) (*webapi.Snapshot, error)
		DeleteSnapshot(context.Context, *webapi.DeleteSnapshotRequest) (*emptypb.Empty, error)
	}
)

// ingest loads the design a command asks about and returns the snapshot's name, and a function that
// deletes it once the command is done (agni issue 1061). The CLI is a client of the services like any
// other, so a local run and a --remote one ingest and ask the same way, and a server is not left
// holding a snapshot nobody will ask about.
func ingest(ctx context.Context, api snapshotAPI, def *webapi.Snapshot) (string, func(), error) {
	snap, err := api.Ingest(ctx, &webapi.IngestRequest{Snapshot: def})
	if err != nil {
		return "", func() {}, err
	}
	name := snap.GetName()
	return name, func() {
		_, _ = api.DeleteSnapshot(context.WithoutCancel(ctx), &webapi.DeleteSnapshotRequest{Name: name})
	}, nil
}

type remoteSnapshot struct {
	c webapiconnect.SnapshotServiceClient
}

func (r *remoteServer) snapshots() snapshotAPI {
	return remoteSnapshot{webapiconnect.NewSnapshotServiceClient(r.http, r.url)}
}

func (r remoteSnapshot) Ingest(ctx context.Context, req *webapi.IngestRequest) (*webapi.Snapshot, error) {
	return unary(ctx, r.c.Ingest, req)
}

func (r remoteSnapshot) DeleteSnapshot(ctx context.Context, req *webapi.DeleteSnapshotRequest) (*emptypb.Empty, error) {
	return unary(ctx, r.c.DeleteSnapshot, req)
}

// unary calls one Connect method and returns the bare message, so a remote call reads like the
// in-process one.
func unary[Req, Resp any](ctx context.Context, call func(context.Context, *connect.Request[Req]) (*connect.Response[Resp], error), req *Req) (*Resp, error) {
	resp, err := call(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

type remoteCheck struct {
	c webapiconnect.CheckServiceClient
}

func (r *remoteServer) check() checkAPI {
	return remoteCheck{webapiconnect.NewCheckServiceClient(r.http, r.url)}
}

func (r remoteCheck) CheckDesign(ctx context.Context, req *webapi.CheckDesignRequest) (*webapi.CheckDesignResponse, error) {
	return unary(ctx, r.c.CheckDesign, req)
}

func (r remoteCheck) GetCheckReport(ctx context.Context, req *webapi.GetCheckReportRequest) (*webapi.GetCheckReportResponse, error) {
	return unary(ctx, r.c.GetCheckReport, req)
}

func (r remoteCheck) RenderCheckReport(ctx context.Context, req *webapi.RenderCheckReportRequest) (*webapi.RenderCheckReportResponse, error) {
	return unary(ctx, r.c.RenderCheckReport, req)
}

type remoteQuery struct {
	c webapiconnect.QueryServiceClient
}

func (r *remoteServer) query() queryAPI {
	return remoteQuery{webapiconnect.NewQueryServiceClient(r.http, r.url)}
}

func (r remoteQuery) RunQuery(ctx context.Context, req *webapi.RunQueryRequest) (*webapi.RunQueryResponse, error) {
	return unary(ctx, r.c.RunQuery, req)
}

func (r remoteQuery) RunQueries(ctx context.Context, req *webapi.RunQueriesRequest) (*webapi.RunQueriesResponse, error) {
	return unary(ctx, r.c.RunQueries, req)
}

func (r remoteQuery) ListQuerySets(ctx context.Context, req *webapi.ListQuerySetsRequest) (*webapi.ListQuerySetsResponse, error) {
	return unary(ctx, r.c.ListQuerySets, req)
}

type remoteTrace struct {
	c webapiconnect.DesignServiceClient
}

func (r *remoteServer) trace() traceAPI {
	return remoteTrace{webapiconnect.NewDesignServiceClient(r.http, r.url)}
}

func (r remoteTrace) TraceDesign(ctx context.Context, req *webapi.TraceDesignRequest) (*webapi.TraceDesignResponse, error) {
	return unary(ctx, r.c.TraceDesign, req)
}

type remoteReview struct {
	c webapiconnect.ReviewServiceClient
}

func (r *remoteServer) review() reviewAPI {
	return remoteReview{webapiconnect.NewReviewServiceClient(r.http, r.url)}
}

func (r remoteReview) CreateReview(ctx context.Context, req *webapi.CreateReviewRequest) (*webapi.Review, error) {
	return unary(ctx, r.c.CreateReview, req)
}

func (r remoteReview) RenderReviewReport(ctx context.Context, req *webapi.RenderReviewReportRequest) (*webapi.RenderReviewReportResponse, error) {
	return unary(ctx, r.c.RenderReviewReport, req)
}
