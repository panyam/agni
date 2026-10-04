package wasmengine

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"google.golang.org/protobuf/proto"
)

// TestConcurrentRequestsShareOneReadAndAgree runs queries and checks on one design at once, the way a
// viewer's panels do on open, against an engine whose reads and model are cached (agni issue 895).
// Every answer must equal the one the same request got alone, and under -race the shared model, fact
// base and cache must not race.
func TestConcurrentRequestsShareOneReadAndAgree(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")
	serve := func() *httptest.Server {
		eng, err := New(fshost.Mount{Name: "m", FS: fshost.MemFS(readTree(t, dir))})
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(eng.Handler)
		t.Cleanup(srv.Close)
		return srv
	}
	// Each answer alone first, from its own engine, so the baseline owes nothing to a warm cache.
	want := make([]proto.Message, len(requests))
	for i, call := range requests {
		m, err := call(serve())
		if err != nil {
			t.Fatal(err)
		}
		want[i] = m
	}
	if len(want[1].(*webapi.RunQueryResponse).GetRows()) == 0 {
		t.Fatal("the component.net query answered nothing, so agreement below would prove nothing")
	}

	srv := serve()
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for round := 0; round < 4; round++ {
		for i, call := range requests {
			wg.Add(1)
			go func(i int, call request) {
				defer wg.Done()
				got, err := call(srv)
				if err != nil {
					errs <- err.Error()
					return
				}
				if !proto.Equal(got, want[i]) {
					errs <- "a concurrent answer differs from the answer the same request got alone"
				}
			}(i, call)
		}
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

type request func(*httptest.Server) (proto.Message, error)

// requests are what a viewer asks of a design as it opens: two queries, the checks and the report.
// A query's work is zeroed, since it is a counter on the shared base that concurrent queries add to.
var requests = []request{
	func(s *httptest.Server) (proto.Message, error) {
		r, err := webapiconnect.NewQueryServiceClient(s.Client(), s.URL).RunQuery(context.Background(), connect.NewRequest(&webapi.RunQueryRequest{Uri: "mount://m", Query: `entity(?n, "net") => count(?n)`}))
		if err != nil {
			return nil, err
		}
		r.Msg.Work = 0
		return r.Msg, nil
	},
	func(s *httptest.Server) (proto.Message, error) {
		r, err := webapiconnect.NewQueryServiceClient(s.Client(), s.URL).RunQuery(context.Background(), connect.NewRequest(&webapi.RunQueryRequest{Uri: "mount://m", Query: `component.net(?r, ?n) => ?r, ?n`}))
		if err != nil {
			return nil, err
		}
		r.Msg.Work = 0
		return r.Msg, nil
	},
	func(s *httptest.Server) (proto.Message, error) {
		r, err := webapiconnect.NewCheckServiceClient(s.Client(), s.URL).CheckDesign(context.Background(), connect.NewRequest(&webapi.CheckDesignRequest{Uri: "mount://m"}))
		if err != nil {
			return nil, err
		}
		return r.Msg, nil
	},
	func(s *httptest.Server) (proto.Message, error) {
		r, err := webapiconnect.NewCheckServiceClient(s.Client(), s.URL).GetCheckReport(context.Background(), connect.NewRequest(&webapi.GetCheckReportRequest{Uri: "mount://m"}))
		if err != nil {
			return nil, err
		}
		return r.Msg, nil
	},
}
