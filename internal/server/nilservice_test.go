package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// A host that leaves a service nil answers its requests UNIMPLEMENTED, rather than panicking in the
// handler (agni issue 1753).
func TestANilServiceAnswersUnimplemented(t *testing.T) {
	mux := http.NewServeMux()
	API{}.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"review": func() error {
			_, err := webapiconnect.NewReviewServiceClient(http.DefaultClient, srv.URL).ListReviews(ctx, connect.NewRequest(&webapi.ListReviewsRequest{}))
			return err
		},
		"check": func() error {
			_, err := webapiconnect.NewCheckServiceClient(http.DefaultClient, srv.URL).ListRules(ctx, connect.NewRequest(&webapi.ListRulesRequest{}))
			return err
		},
		"lexicon": func() error {
			_, err := webapiconnect.NewLexiconServiceClient(http.DefaultClient, srv.URL).GetLexicon(ctx, connect.NewRequest(&webapi.GetLexiconRequest{}))
			return err
		},
		"query": func() error {
			_, err := webapiconnect.NewQueryServiceClient(http.DefaultClient, srv.URL).RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{}))
			return err
		},
		"workspace": func() error {
			_, err := webapiconnect.NewWorkspaceServiceClient(http.DefaultClient, srv.URL).ListMounts(ctx, connect.NewRequest(&webapi.ListMountsRequest{}))
			return err
		},
	} {
		if err := call(); connect.CodeOf(err) != connect.CodeUnimplemented {
			t.Errorf("%s on a host that left it nil = %v; want UNIMPLEMENTED", name, err)
		}
	}
}
