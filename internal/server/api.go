package server

import (
	"context"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/service"
)

// API is the set of services one host serves over Connect. `agni serve` and the wasm engine both
// register through it, so the two cannot serve different lists of handlers. TableService holds no
// state, so it has no field here and every host serves it.
type API struct {
	Workspace *service.WorkspaceService
	Project   *service.ProjectService
	// Snapshot ingests designs, and every other service answers questions about them (agni issue
	// 1061). It must hold the same ProjectResolver as they do, or a question names a snapshot that
	// none of them holds.
	Snapshot   *service.SnapshotService
	DesignView *service.DesignViewService
	// Design serves the stored designs, "designs/{design}" (agni issue 1694). Nil serves an empty
	// catalog that refuses every write.
	Design *service.DesignService
	// Pack serves the stored packs, "packs/{pack}" (agni issue 1704). Nil serves none and refuses
	// every write.
	Pack *service.PackService
	// Connection serves the sign-ins private sources are read through, "connections/{connection}"
	// (agni issue 1724). Nil serves none and refuses a write.
	Connection *service.ConnectionService
	Check      *service.CheckService
	Diff       *service.DiffService
	Query      *service.QueryService
	Review     *service.ReviewService
	// Info answers ServerService.GetServerInfo (agni issue 1629). Nil answers this build's version and
	// the snapshot features alone, as the browser engine does.
	Info *service.ServerInfoService
	// Timing times the requests that ask, and logs the slow ones (agni issue 914). Its zero value
	// still answers a request that asks with the Agni-Timing header.
	Timing Timing
	// Hooks run inside Timing around every service's requests, so a host can act on what a request
	// names. `agni serve` warms a design's browser-engine seed this way (agni issue 1572).
	Hooks []connect.Interceptor
}

// Register mounts every service on mux and returns the paths it registered, in a fixed order.
// queryOpts apply to the services that evaluate queries (check, query, review). A deployment's
// query budget attaches to those services.
func (a API) Register(mux *http.ServeMux, queryOpts ...connect.HandlerOption) []string {
	var paths []string
	add := func(p string, h http.Handler) {
		mux.Handle(p, h)
		paths = append(paths, p)
	}
	// Timing wraps every handler, outermost, so a request's total includes the budget's own work.
	timed := connect.WithInterceptors(append([]connect.Interceptor{a.Timing.Interceptor(), refreshedHeader()}, a.Hooks...)...)
	queried := append([]connect.HandlerOption{timed}, queryOpts...)
	// A service the host left nil answers UNIMPLEMENTED, rather than a wrapper calling a nil service
	// and panicking in the handler (agni issue 1753).
	add(pick(a.Workspace == nil, func() (string, http.Handler) {
		return webapiconnect.NewWorkspaceServiceHandler(webapiconnect.UnimplementedWorkspaceServiceHandler{}, timed)
	}, func() (string, http.Handler) {
		return webapiconnect.NewWorkspaceServiceHandler(NewWorkspace(a.Workspace), timed)
	}))
	add(pick(a.Project == nil, func() (string, http.Handler) {
		return webapiconnect.NewProjectServiceHandler(webapiconnect.UnimplementedProjectServiceHandler{}, timed)
	}, func() (string, http.Handler) {
		return webapiconnect.NewProjectServiceHandler(NewProject(a.Project), timed)
	}))
	add(pick(a.Snapshot == nil, func() (string, http.Handler) {
		return webapiconnect.NewSnapshotServiceHandler(webapiconnect.UnimplementedSnapshotServiceHandler{}, timed)
	}, func() (string, http.Handler) {
		return webapiconnect.NewSnapshotServiceHandler(NewSnapshot(a.Snapshot), timed)
	}))
	add(pick(a.DesignView == nil, func() (string, http.Handler) {
		return webapiconnect.NewDesignViewServiceHandler(webapiconnect.UnimplementedDesignViewServiceHandler{}, timed)
	}, func() (string, http.Handler) {
		return webapiconnect.NewDesignViewServiceHandler(NewDesignView(a.DesignView), timed)
	}))
	design := a.Design
	if design == nil {
		design = service.NewDesignService(nil, nil)
	}
	add(webapiconnect.NewDesignServiceHandler(NewDesign(design), timed))
	pack := a.Pack
	if pack == nil {
		pack = service.NewPackService(nil, nil)
	}
	add(webapiconnect.NewPackServiceHandler(NewPack(pack), timed))
	conns := a.Connection
	if conns == nil {
		conns = service.NewConnectionService(nil, nil)
	}
	add(webapiconnect.NewConnectionServiceHandler(NewConnection(conns), timed))
	add(pick(a.Check == nil, func() (string, http.Handler) {
		return webapiconnect.NewCheckServiceHandler(webapiconnect.UnimplementedCheckServiceHandler{}, queried...)
	}, func() (string, http.Handler) {
		return webapiconnect.NewCheckServiceHandler(NewCheck(a.Check), queried...)
	}))
	// The lexicon questions run the check service's catalog over its loader, so they are built from
	// the check service. A field would let a host fill in a different catalog (agni issue 1266).
	add(pick(a.Check == nil, func() (string, http.Handler) {
		return webapiconnect.NewLexiconServiceHandler(webapiconnect.UnimplementedLexiconServiceHandler{}, queried...)
	}, func() (string, http.Handler) {
		return webapiconnect.NewLexiconServiceHandler(NewLexicon(service.NewLexiconService(a.Check)), queried...)
	}))
	add(pick(a.Diff == nil, func() (string, http.Handler) {
		return webapiconnect.NewDiffServiceHandler(webapiconnect.UnimplementedDiffServiceHandler{}, timed)
	}, func() (string, http.Handler) { return webapiconnect.NewDiffServiceHandler(NewDiff(a.Diff), timed) }))
	add(webapiconnect.NewTableServiceHandler(NewTable(), timed))
	add(pick(a.Query == nil, func() (string, http.Handler) {
		return webapiconnect.NewQueryServiceHandler(webapiconnect.UnimplementedQueryServiceHandler{}, queried...)
	}, func() (string, http.Handler) {
		return webapiconnect.NewQueryServiceHandler(NewQuery(a.Query), queried...)
	}))
	add(pick(a.Review == nil, func() (string, http.Handler) {
		return webapiconnect.NewReviewServiceHandler(webapiconnect.UnimplementedReviewServiceHandler{}, queried...)
	}, func() (string, http.Handler) {
		return webapiconnect.NewReviewServiceHandler(NewReview(a.Review), queried...)
	}))
	info := a.Info
	if info == nil {
		info = &service.ServerInfoService{Version: version.Version()}
	}
	add(webapiconnect.NewServerServiceHandler(NewServerInfo(info), timed))
	return paths
}

// SnapshotRefreshedHeader is the response header naming the tiers a refreshed snapshot found changed,
// comma-separated, when a question read a snapshot again rather than refusing it as stale (agni issue
// 1639). A snapshot refreshes only when its definition sets refresh.
const SnapshotRefreshedHeader = "Agni-Snapshot-Refreshed"

// refreshedHeader records refreshes while a request runs, and names what changed in the response.
func refreshedHeader() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ctx, rs := service.WithRefreshes(ctx)
			resp, err := next(ctx, req)
			if changed := rs.Changed(); resp != nil && len(changed) > 0 {
				resp.Header().Set(SnapshotRefreshedHeader, strings.Join(changed, ", "))
			}
			return resp, err
		}
	}
}

// pick builds a service's handler with none when the host left the service nil, and with some
// otherwise.
func pick(nilService bool, none, some func() (string, http.Handler)) (string, http.Handler) {
	if nilService {
		return none()
	}
	return some()
}
