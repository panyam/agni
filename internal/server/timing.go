package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/timing"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

// TimingHeader is the request header that asks for a request's timing, and the response header that
// carries it back as a RequestTiming in protojson. A value containing "explain" also asks each query
// evaluation for its plan.
const TimingHeader = "Agni-Timing"

// Timing times requests and reports where their time went (agni issue 914). A request is timed when
// it asks (the Agni-Timing header) or when Slow is set, and a request that asked gets its breakdown
// in the Agni-Timing and Server-Timing response headers.
type Timing struct {
	// Slow logs every request that takes longer, with the stage that took the most, the tier that
	// answered each cached read, the slowest rules, and the plan when evaluation was the slowest
	// stage. Zero logs nothing.
	Slow time.Duration
	// Log receives each line. Nil discards them.
	Log func(format string, args ...any)
}

// Interceptor returns the timing interceptor. A request that neither asks nor could be slow-logged
// passes straight through.
func (t Timing) Interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			asked := req.Header().Get(TimingHeader)
			logging := t.Slow > 0 && t.Log != nil
			if asked == "" && !logging {
				return next(ctx, req)
			}
			rec := timing.New()
			// A slow-logged request records its plans too, since whether it will be slow is only known
			// at the end, and a plan costs time in proportion to the query's rules, not its tuples.
			rec.Explain = strings.Contains(asked, "explain") || logging
			resp, err := next(timing.With(ctx, rec), req)
			snap := rec.Snapshot()
			proc := req.Spec().Procedure
			if asked != "" && resp != nil {
				resp.Header().Set("Server-Timing", service.ServerTiming(snap))
				if b, merr := protojson.Marshal(service.TimingProto(proc, snap)); merr == nil {
					resp.Header().Set(TimingHeader, string(b))
				}
			}
			if logging && snap.Total > t.Slow {
				t.Log("%s", SlowLine(proc, t.Slow, snap))
			}
			return resp, err
		}
	}
}

// SlowLine is the log line for one slow request: what took longest, which tier answered each cached
// read, the slowest rules, and the plan when evaluating a query was the slowest stage.
func SlowLine(procedure string, over time.Duration, s timing.Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "slow request: %s took %s, over %s", procedure, s.Total.Round(time.Millisecond), over)
	if st, ok := s.Slowest(); ok {
		fmt.Fprintf(&b, "; slowest stage %s %s", st.Name, st.Dur.Round(time.Millisecond))
	}
	if len(s.Lookups) > 0 {
		parts := make([]string, len(s.Lookups))
		for i, l := range s.Lookups {
			parts[i] = l.Layer + "=" + l.Source
		}
		fmt.Fprintf(&b, "; cache %s", strings.Join(parts, " "))
	}
	if len(s.Rules) > 0 {
		n := min(5, len(s.Rules))
		parts := make([]string, n)
		for i, r := range s.Rules[:n] {
			parts[i] = fmt.Sprintf("%s %s", r.Name, r.Dur.Round(time.Millisecond))
		}
		fmt.Fprintf(&b, "; slowest rules %s", strings.Join(parts, ", "))
	}
	if st, ok := s.Slowest(); ok && st.Name == "evaluate" {
		for _, p := range s.Plans {
			fmt.Fprintf(&b, "\n  plan for %s\n    %s", p.Query, strings.ReplaceAll(strings.TrimRight(p.Text, "\n"), "\n", "\n    "))
		}
	}
	return b.String()
}
