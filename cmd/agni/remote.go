package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/storage/mounts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// remoteMountTimeout bounds the ListMounts call --remote makes before anything else, so a wrong
// address fails in a moment rather than when the analysis request times out.
const remoteMountTimeout = 5 * time.Second

// remoteFlag adds --remote to a command whose answer comes from one service request (agni issue 998).
func remoteFlag(cmd *cobra.Command, remote *string) {
	cmd.Flags().StringVar(remote, "remote", "",
		"send this command's request to the agni serve at this URL instead of reading the design here. "+
			"The design's files go with it, those the server does not already hold, so the server needs "+
			"no copy of them, and one that kept what it read answers from that. The output is the same; "+
			"only where the engine ran changes. An argument designs/<id> that names no path here names a design "+
			"the server keeps, and nothing is sent")
}

// remoteServer is a running `agni serve` this command sends its request to.
type remoteServer struct {
	url  string
	http *http.Client
}

// dialRemote checks that the server at raw answers, by asking it for its mounts.
//
// UNREACHABLE IS AN ERROR HERE, unlike --server's link check (verifyServerMount). --server only
// decides whether a link is printed, while --remote supplies the answer itself. Falling back
// to a local read would answer from a different engine without saying so.
func dialRemote(ctx context.Context, raw string) (*remoteServer, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("--remote %q: want a URL like http://localhost:8080", raw)
	}
	r := &remoteServer{url: strings.TrimRight(raw, "/"), http: http.DefaultClient}
	ctx, cancel := context.WithTimeout(ctx, remoteMountTimeout)
	defer cancel()
	if _, err := webapiconnect.NewWorkspaceServiceClient(r.http, r.url).
		ListMounts(ctx, connect.NewRequest(&webapi.ListMountsRequest{})); err != nil {
		return nil, fmt.Errorf("--remote %s: could not reach the server: %w", r.url, err)
	}
	return r, nil
}

// uri names a command-line argument as this process does (cliArgURI), with this machine's own
// mounts. Under --remote the design's files are sent with the request (agni issue 1061). The server
// then needs no mount of its own over the path, and every locator in the answer reads as a local
// run's does.
//
// An argument naming a design the server holds, designs/{d}, with no such path here, passes through as
// that name (agni issue 1695). ingest then sends it as the snapshot's design, and the server reads its
// own copy, so no file goes with the request.
func (r *remoteServer) uri(arg string) (string, error) {
	if storedDesignArg(arg) {
		return arg, nil
	}
	return cliArgURI(arg)
}

var storedDesignPattern = regexp.MustCompile(`^designs/[a-z0-9][a-z0-9._-]*$`)

// storedDesignArg reports whether a command-line argument names a design on a server rather than a
// path here. A path that exists wins, so a local folder called designs/gw is still read as a folder.
func storedDesignArg(arg string) bool {
	if !storedDesignPattern.MatchString(arg) {
		return false
	}
	_, err := os.Stat(arg)
	return err != nil
}

// refuseLocalOnly names the flags a --remote run cannot honour. Each one configures the engine that
// THIS process would have built, and none of them travels with the request. The server answers with
// the corpus, profiles and catalog it was started with. Passing one of these would look applied
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
// each one, and so does the Connect client wrapper below. So a remote answer is the same message as
// the local one (C31, C32).
type (
	checkAPI interface {
		ListRules(context.Context, *webapi.ListRulesRequest) (*webapi.ListRulesResponse, error)
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
		ReachablePins(context.Context, *webapi.ReachablePinsRequest) (*webapi.ReachablePinsResponse, error)
	}
	walkAPI interface {
		WalkDesign(context.Context, *webapi.WalkDesignRequest) (*webapi.WalkDesignResponse, error)
	}
	reviewAPI interface {
		CreateReview(context.Context, *webapi.CreateReviewRequest) (*webapi.Review, error)
		RenderReviewReport(context.Context, *webapi.RenderReviewReportRequest) (*webapi.RenderReviewReportResponse, error)
	}
	diffAPI interface {
		DiffDesigns(context.Context, *webapi.DiffDesignsRequest) (*webapi.DiffDesignsResponse, error)
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
	if readFlatten && !def.GetFlattenHierarchy() {
		def = proto.Clone(def).(*webapi.Snapshot)
		def.FlattenHierarchy = true
	}
	if readOverlay != nil && def.GetOverlay() == nil {
		def = proto.Clone(def).(*webapi.Snapshot)
		def.Overlay = readOverlay
	}
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

// Ingest sends the design as files. It sends a manifest of the files that its analysis reads, and it
// uploads their contents only when the server reports them missing. A second run of an unchanged
// design sends the manifest alone (agni issue 1061).
func (r remoteSnapshot) Ingest(ctx context.Context, req *webapi.IngestRequest) (*webapi.Snapshot, error) {
	req = proto.Clone(req).(*webapi.IngestRequest)
	if def := req.GetSnapshot(); storedDesignPattern.MatchString(def.GetUri()) {
		// A design the server holds is named, and the server reads its own files (agni issue 1695).
		def.Design, def.Uri = def.GetUri(), ""
		return unary(ctx, r.c.Ingest, req)
	}
	read, err := sendFiles(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	snap, err := unary(ctx, r.c.Ingest, req)
	var ce *connect.Error
	if err == nil || !errors.As(err, &ce) {
		return snap, err
	}
	var missing []string
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if bm, ok := v.(*webapi.BlobsMissing); ok {
				missing = append(missing, bm.GetSha256()...)
			}
		}
	}
	if len(missing) == 0 {
		return nil, err
	}
	if err := r.upload(ctx, missing, read); err != nil {
		return nil, err
	}
	return unary(ctx, r.c.Ingest, req)
}

// uploadBatchBytes bounds one UploadBlobs request, so a large board goes in several.
const uploadBatchBytes = 64 << 20

// upload sends the contents of the files whose hashes the server asked for.
func (r remoteSnapshot) upload(ctx context.Context, hashes []string, read func(sha string) ([]byte, error)) error {
	var batch [][]byte
	var size int
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		_, err := unary(ctx, r.c.UploadBlobs, &webapi.UploadBlobsRequest{Contents: batch})
		batch, size = nil, 0
		return err
	}
	for _, h := range hashes {
		b, err := read(h)
		if err != nil {
			return err
		}
		if size+len(b) > uploadBatchBytes {
			if err := flush(); err != nil {
				return err
			}
		}
		batch, size = append(batch, b), size+len(b)
	}
	return flush()
}

// sendFiles fills def's manifest with the files that its design's analysis reads on this machine.
// It names them as ListDesignFiles does for a browser, and adds a --board-path outside that set. It
// returns a function that reads a file's contents by its hash.
func sendFiles(ctx context.Context, def *webapi.Snapshot) (func(sha string) ([]byte, error), error) {
	u, err := mounts.ParseURI(def.GetUri())
	if err != nil {
		return nil, err
	}
	ws, err := workspace()
	if err != nil {
		return nil, err
	}
	table := mountset.NewTable(ws.Mounts()...)
	files := osFiles(table)
	listed, err := service.NewWorkspaceService(osWorkspace(table)).WithDesignFiles(cliProjects(), files).
		ListDesignFiles(ctx, &webapi.ListDesignFilesRequest{Uri: def.GetUri()})
	if err != nil {
		return nil, fmt.Errorf("--remote: listing %s's files to send: %w", def.GetUri(), err)
	}
	byHash := map[string]string{}
	have := map[string]bool{}
	for _, f := range listed.GetFiles() {
		def.Files = append(def.Files, &webapi.SnapshotFile{Path: f.GetPath(), Sha256: f.GetSha256(), Size: f.GetSize()})
		byHash[f.GetSha256()], have[f.GetPath()] = f.GetPath(), true
	}
	if b := def.GetBoardUri(); b != "" {
		bu, err := mounts.ParseURI(b)
		if err != nil {
			return nil, err
		}
		if bu.Mount != u.Mount {
			return nil, fmt.Errorf("--remote sends the design's mount alone, and --board-path %s is in another; put the board under %s", b, u.Mount)
		}
		if !have[bu.Path] {
			c, err := files.ReadFile(ctx, bu)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(c)
			h := "sha256:" + hex.EncodeToString(sum[:])
			def.Files = append(def.Files, &webapi.SnapshotFile{Path: bu.Path, Sha256: h, Size: int64(len(c))})
			byHash[h] = bu.Path
		}
	}
	return func(sha string) ([]byte, error) {
		p, ok := byHash[sha]
		if !ok {
			return nil, fmt.Errorf("--remote: the server asked for a file this design does not have (%s)", sha)
		}
		f, err := mounts.NewURI(u.Mount, p)
		if err != nil {
			return nil, err
		}
		return files.ReadFile(ctx, f)
	}, nil
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

type remoteDiff struct {
	c webapiconnect.DiffServiceClient
}

func (r *remoteServer) diff() diffAPI {
	return remoteDiff{webapiconnect.NewDiffServiceClient(r.http, r.url)}
}

func (r remoteDiff) DiffDesigns(ctx context.Context, req *webapi.DiffDesignsRequest) (*webapi.DiffDesignsResponse, error) {
	return unary(ctx, r.c.DiffDesigns, req)
}

type remoteCheck struct {
	c webapiconnect.CheckServiceClient
}

func (r *remoteServer) check() checkAPI {
	return remoteCheck{webapiconnect.NewCheckServiceClient(r.http, r.url)}
}

func (r remoteCheck) ListRules(ctx context.Context, req *webapi.ListRulesRequest) (*webapi.ListRulesResponse, error) {
	return unary(ctx, r.c.ListRules, req)
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
	c webapiconnect.DesignViewServiceClient
}

func (r *remoteServer) trace() traceAPI {
	return remoteTrace{webapiconnect.NewDesignViewServiceClient(r.http, r.url)}
}

func (r remoteTrace) TraceDesign(ctx context.Context, req *webapi.TraceDesignRequest) (*webapi.TraceDesignResponse, error) {
	return unary(ctx, r.c.TraceDesign, req)
}

func (r remoteTrace) ReachablePins(ctx context.Context, req *webapi.ReachablePinsRequest) (*webapi.ReachablePinsResponse, error) {
	return unary(ctx, r.c.ReachablePins, req)
}

func (r *remoteServer) walk() walkAPI {
	return remoteTrace{webapiconnect.NewDesignViewServiceClient(r.http, r.url)}
}

func (r remoteTrace) WalkDesign(ctx context.Context, req *webapi.WalkDesignRequest) (*webapi.WalkDesignResponse, error) {
	return unary(ctx, r.c.WalkDesign, req)
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

type remoteLexicon struct {
	c webapiconnect.LexiconServiceClient
}

func (r *remoteServer) lexicon() lexiconAPI {
	return remoteLexicon{webapiconnect.NewLexiconServiceClient(r.http, r.url)}
}

func (r remoteLexicon) GetLexicon(ctx context.Context, req *webapi.GetLexiconRequest) (*webapi.GetLexiconResponse, error) {
	return unary(ctx, r.c.GetLexicon, req)
}

func (r remoteLexicon) ExplainNet(ctx context.Context, req *webapi.ExplainNetRequest) (*webapi.ExplainNetResponse, error) {
	return unary(ctx, r.c.ExplainNet, req)
}

func (r remoteLexicon) CompareLexicons(ctx context.Context, req *webapi.CompareLexiconsRequest) (*webapi.CompareLexiconsResponse, error) {
	return unary(ctx, r.c.CompareLexicons, req)
}

// storedDesignPacks is the packs a stored design reads under by default, when check or review names
// one, designs/<id>, on a server and passes no --pack (agni issue 1756). A design added from a
// mounted project names that project's pack, so its rules apply with no flag. It notes the packs on
// stderr, since rules nobody typed decide the run. Anything else returns none.
func storedDesignPacks(cmd *cobra.Command, remoteURL, arg string) ([]string, error) {
	if remoteURL == "" || !storedDesignArg(arg) {
		return nil, nil
	}
	r, err := dialRemote(cmd.Context(), remoteURL)
	if err != nil {
		return nil, err
	}
	d, err := webapiconnect.NewDesignServiceClient(r.http, r.url).GetDesign(cmd.Context(), connect.NewRequest(&webapi.GetDesignRequest{Name: service.StandaloneDesignName(strings.TrimPrefix(arg, "designs/"))}))
	if err != nil {
		return nil, rpcFailed(err)
	}
	packs := d.Msg.GetPacks()
	if len(packs) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: reading %s under its packs %s; pass --pack to name others\n", arg, strings.Join(packs, ", "))
	}
	return packs, nil
}
