package service

import (
	"fmt"
	"sort"
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
		out.Stages = append(out.Stages, &webapi.TimedStage{
			Name: st.Name, StartMicros: st.Start.Microseconds(), Micros: st.Dur.Microseconds(),
			SysBytes: int64(st.Sys), HeapBytes: int64(st.HeapAlloc),
		})
	}
	for _, l := range s.Lookups {
		out.Lookups = append(out.Lookups, &webapi.CacheLookup{Layer: l.Layer, Source: l.Source})
	}
	for _, r := range listedRules(s.Rules) {
		out.Rules = append(out.Rules, &webapi.RuleTiming{Rule: r.Name, Micros: r.Dur.Microseconds(), AllocBytes: int64(r.Alloc)})
	}
	for _, p := range s.Plans {
		out.Plans = append(out.Plans, &webapi.QueryPlan{Query: p.Query, Plan: p.Text})
	}
	return out
}

// listedRules is the slowest timedRules of rules (which arrive slowest first), and, when allocation was
// recorded, the timedRules that allocated most as well, since the rule raising a worker's memory is
// often not among the slowest (agni issue 994).
func listedRules(rules []timing.RuleTime) []timing.RuleTime {
	out := rules[:min(timedRules, len(rules))]
	byAlloc := append([]timing.RuleTime(nil), rules...)
	sort.SliceStable(byAlloc, func(i, j int) bool { return byAlloc[i].Alloc > byAlloc[j].Alloc })
	listed := map[string]bool{}
	for _, r := range out {
		listed[r.Name] = true
	}
	out = append([]timing.RuleTime(nil), out...)
	for _, r := range byAlloc[:min(timedRules, len(byAlloc))] {
		if r.Alloc > 0 && !listed[r.Name] {
			out = append(out, r)
		}
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
