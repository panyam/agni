package query

import (
	"fmt"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Board-scale evidence for WS3-113. The Naive evaluator's doc comment says "naïve join is
// sufficient because one design's fact base is small". This measures whether that survives a real
// board: the customer EVT netlist this catalog is developed against carries 3,980 components and
// 1,617 nets, so the sweep brackets it.
//
// The shapes are chosen to separate three claims that WS3-031 and WS3-113 bundle together:
//
//	flat      a conjunctive pattern with no recursion (the shipped pull-up shape)
//	reach     a bounded-radius protection question, which is what a migrated ESD rule would run
//	closure   a recursive transitive closure, where addTuple's linear-scan dedup is O(n^2)
//
// They fail differently, and a fix aimed at one does nothing for the others.

// benchDesign builds a synthetic board of n components in series chains: every component bridges
// two consecutive nets, which is the topology `reaches` walks. Resistors are the pass element, so
// the chains are genuinely traversable rather than a star that terminates in one hop.
func benchDesign(n int) *ir.Design {
	d := &ir.Design{}
	nets := n + 1
	for i := 0; i < nets; i++ {
		d.Nets = append(d.Nets, &ir.Net{Name: fmt.Sprintf("N%d", i), Prov: &ir.Provenance{SourceFile: "b"}})
	}
	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("R%d", i)
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "b"}})
		d.Nets[i].Connections = append(d.Nets[i].Connections, &ir.Connection{ComponentRef: ref, PinRef: "1"})
		d.Nets[i+1].Connections = append(d.Nets[i+1].Connections, &ir.Connection{ComponentRef: ref, PinRef: "2"})
	}
	return d
}

var benchSizes = []int{100, 500, 1000, 2000, 4000}

func benchQuery(b *testing.B, name, text string) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("%s/n=%d", name, n), func(b *testing.B) {
			m := check.NewModel(benchDesign(n))
			q, err := Parse(text)
			if err != nil {
				b.Fatalf("Parse: %v", err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := (Naive{}).Eval(q, NewBase(m)); err != nil {
					b.Fatalf("Eval: %v", err)
				}
			}
		})
	}
}

// A flat conjunctive pattern: two distinct components sharing a net. No recursion, no reach.
func BenchmarkEvalFlat(b *testing.B) {
	benchQuery(b, "flat", `component-on-net(?a,?n), component-on-net(?c,?n), ?a != ?c => ?a`)
}

// The bounded-radius protection shape, which is what an ESD rule migrated off its Go FFI would run
// (the form documented in relations/facts/docs/reaches.md).
func BenchmarkEvalReach(b *testing.B) {
	benchQuery(b, "reach", `reaches(?a,?bn,?h), ?h <= 2 => ?a`)
}

// Recursive transitive closure: the shape whose derived-tuple count grows quadratically on a chain,
// and where addTuple dedups by linear scan.
func BenchmarkEvalClosure(b *testing.B) {
	benchQuery(b, "closure", `conn(?a,?bn) :- component-on-net(?a,?bn); `+
		`linked(?a,?c) :- conn(?a,?n), conn(?c,?n), ?a != ?c; `+
		`linked("R1",?x) => ?x`)
}
