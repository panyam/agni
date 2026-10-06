package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/panyam/agni/core/timing"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// timedRules caps how many rules a RequestTiming lists, slowest first. A catalog run evaluates over a
// hundred rules, and the reader wants the few that dominate.
const timedRules = 20

// TimingProto is a request's timing snapshot as the RequestTiming a client reads (agni issue 914).
func TimingProto(procedure string, s timing.Snapshot) *webapi.RequestTiming {
	out := &webapi.RequestTiming{Procedure: procedure, TotalMicros: s.Total.Microseconds(), RulesEvaluated: int32(len(s.Rules))}
	for _, st := range s.Stages {
		out.Stages = append(out.Stages, &webapi.TimedStage{Name: st.Name, StartMicros: st.Start.Microseconds(), Micros: st.Dur.Microseconds()})
	}
	for _, l := range s.Lookups {
		out.Lookups = append(out.Lookups, &webapi.CacheLookup{Layer: l.Layer, Source: l.Source})
	}
	for i, r := range s.Rules {
		if i == timedRules {
			break
		}
		out.Rules = append(out.Rules, &webapi.RuleTiming{Rule: r.Name, Micros: r.Dur.Microseconds()})
	}
	for _, p := range s.Plans {
		out.Plans = append(out.Plans, &webapi.QueryPlan{Query: p.Query, Plan: p.Text})
	}
	return out
}

// ServerTiming renders a snapshot as a Server-Timing header value, which a browser's network panel
// draws: the total, then each stage once, summed when it ran more than once, in the order it first
// started.
func ServerTiming(s timing.Snapshot) string {
	parts := []string{fmt.Sprintf("total;dur=%s", ms(s.Total))}
	sums := map[string]time.Duration{}
	var order []string
	for _, st := range s.Stages {
		if _, ok := sums[st.Name]; !ok {
			order = append(order, st.Name)
		}
		sums[st.Name] += st.Dur
	}
	for _, n := range order {
		parts = append(parts, fmt.Sprintf("%s;dur=%s", n, ms(sums[n])))
	}
	return strings.Join(parts, ", ")
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1f", float64(d.Microseconds())/1000) }
