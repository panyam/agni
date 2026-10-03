package server

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
)

// API is the set of services one host serves over Connect. `agni serve` and the wasm engine both
// register through it, so the two cannot serve different lists of handlers.
type API struct {
	Workspace *service.WorkspaceService
	Project   *service.ProjectService
	Design    *service.DesignService
	Check     *service.CheckService
	Diff      *service.DiffService
	Query     *service.QueryService
	Review    *service.ReviewService
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
	add(webapiconnect.NewWorkspaceServiceHandler(NewWorkspace(a.Workspace)))
	add(webapiconnect.NewProjectServiceHandler(NewProject(a.Project)))
	add(webapiconnect.NewDesignServiceHandler(NewDesign(a.Design)))
	add(webapiconnect.NewCheckServiceHandler(NewCheck(a.Check), queryOpts...))
	add(webapiconnect.NewDiffServiceHandler(NewDiff(a.Diff)))
	add(webapiconnect.NewQueryServiceHandler(NewQuery(a.Query), queryOpts...))
	add(webapiconnect.NewReviewServiceHandler(NewReview(a.Review), queryOpts...))
	return paths
}
