package check

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// This file holds pure IR/param/geom fixture builders shared by the check-package tests
// (reach_test.go, trace_test.go). They build no check types, so stdlib/rules/builtin,
// stdlib/rules/datalog and stdlib/relations each keep their own copies of the ones their tests
// need. The sets are independent test fixtures, not a shared contract.

// tnet builds a net with "refdes.pin" connections.
func tnet(name string, conns ...string) *ir.Net {
	n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
	for _, c := range conns {
		p := strings.SplitN(c, ".", 2)
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: p[0], PinRef: p[1]})
	}
	return n
}
