package wasmengine

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/goapplib/wasmhost"
	"google.golang.org/protobuf/encoding/protojson"
)

// timedCheck runs CheckDesign on the gateway asking for its timing (agni issue 914), and returns the
// breakdown the engine sent back with the response's two timing headers.
func timedCheck(t *testing.T, eng *Engine, ask bool) (*webapi.RequestTiming, string) {
	t.Helper()
	srv := httptest.NewServer(eng.Handler)
	defer srv.Close()
	req := connect.NewRequest(&webapi.CheckDesignRequest{Uri: "mount://m"})
	if ask {
		req.Header().Set("Agni-Timing", "1")
	}
	resp, err := webapiconnect.NewCheckServiceClient(srv.Client(), srv.URL).CheckDesign(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	raw := resp.Header().Get("Agni-Timing")
	if raw == "" {
		return nil, resp.Header().Get("Server-Timing")
	}
	var rt webapi.RequestTiming
	if err := protojson.Unmarshal([]byte(raw), &rt); err != nil {
		t.Fatalf("the Agni-Timing header is not a RequestTiming: %v\n%s", err, raw)
	}
	return &rt, resp.Header().Get("Server-Timing")
}

func lookup(rt *webapi.RequestTiming, layer string) string {
	for _, l := range rt.GetLookups() {
		if l.GetLayer() == layer {
			return l.GetSource()
		}
	}
	return ""
}

// TestARequestThatAsksIsToldWhereItsTimeWent holds the browser engine to the breakdown agni serve
// gives: the stages, each rule, and which tier answered each read, so a design read in the page can be
// diagnosed as one read by a server can.
func TestARequestThatAsksIsToldWhereItsTimeWent(t *testing.T) {
	store := &wasmhost.MemCache{}
	eng := gatewayEngine(t, store, "v1", nil)

	first, st := timedCheck(t, eng, true)
	if first == nil {
		t.Fatal("a request that asked got no Agni-Timing header")
	}
	stages := map[string]bool{}
	for _, s := range first.GetStages() {
		stages[s.GetName()] = true
	}
	for _, want := range []string{"resolve.overlay", "resolve.tiers", "build.model", "read.netlist", "model", "rules", "locate"} {
		if !stages[want] {
			t.Errorf("the breakdown has no %q stage: %v", want, stages)
		}
	}
	if first.GetRulesEvaluated() == 0 || len(first.GetRules()) == 0 || first.GetTotalMicros() <= 0 {
		t.Errorf("rules evaluated %d, listed %d, total %dus; want each set", first.GetRulesEvaluated(), len(first.GetRules()), first.GetTotalMicros())
	}
	if !strings.Contains(st, "total;dur=") || !strings.Contains(st, "rules;dur=") {
		t.Errorf("Server-Timing = %q, want the total and the rules", st)
	}
	// The model was built for this request, and the next one is answered from memory.
	if got := lookup(first, "model"); got != "built" {
		t.Errorf("the first request's model came from %q, want built", got)
	}
	if rt, st := timedCheck(t, eng, false); rt != nil || st != "" {
		t.Fatal("a request that did not ask was sent its timing")
	}
	second, _ := timedCheck(t, eng, true)
	if got := lookup(second, "model"); got != "memory" {
		t.Errorf("the second request's model came from %q, want memory", got)
	}
	// A second worker sharing the store, as after a reload, restores the read rather than reading.
	reloaded, _ := timedCheck(t, gatewayEngine(t, store, "v1", nil), true)
	if got := lookup(reloaded, "design"); got != "store" {
		t.Errorf("a second worker's design came from %q, want the store", got)
	}
}
