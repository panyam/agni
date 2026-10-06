package server

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
)

// API is the set of services one host serves over Connect. `agni serve` and the wasm engine both
// register through it, so the two cannot serve different lists of handlers. TableService holds no
// state, so it has no field here and every host serves it.
type API struct {
	Workspace *service.WorkspaceService
	Project   *service.ProjectService
	Design    *service.DesignService
	Check     *service.CheckService
	Diff      *service.DiffService
	Query     *service.QueryService
	Review    *service.ReviewService
	// Timing times the requests that ask, and logs the slow ones (agni issue 914). Its zero value
	// still answers a request that asks with the Agni-Timing header.
	Timing Timing
}

// Register mounts every service on mux and returns the paths it registered, in a fixed order.
// queryOpts apply to the services that evaluate queries (check, query, review), which is where a
// deployment's query budget attaches.
func (a API) Register(mux *http.ServeMux, queryOpts ...connect.HandlerOption) []string {
	var paths []string
	add := func(p string, h http.Handler) {
		mux.Handle(p, h)
		paths = append(paths, p)
	}
	// Timing wraps every handler, outermost, so a request's total includes the budget's own work.
	timed := connect.WithInterceptors(a.Timing.Interceptor())
	queried := append([]connect.HandlerOption{timed}, queryOpts...)
	add(webapiconnect.NewWorkspaceServiceHandler(NewWorkspace(a.Workspace), timed))
	add(webapiconnect.NewProjectServiceHandler(NewProject(a.Project), timed))
	add(webapiconnect.NewDesignServiceHandler(NewDesign(a.Design), timed))
	add(webapiconnect.NewCheckServiceHandler(NewCheck(a.Check), queried...))
	add(webapiconnect.NewDiffServiceHandler(NewDiff(a.Diff), timed))
	add(webapiconnect.NewTableServiceHandler(NewTable(), timed))
	add(webapiconnect.NewQueryServiceHandler(NewQuery(a.Query), queried...))
	add(webapiconnect.NewReviewServiceHandler(NewReview(a.Review), queried...))
	return paths
}
