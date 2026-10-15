package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// KiCad names every unlabelled one-pin net "unconnected-(...)", flagged or not, so the name is not
// the designer's intent; the reader's no-connect flag is (agni issue 1019). A designer's own NC_ or
// no_connect label still is.
func TestNoConnectIntentComesFromTheFlagNotTheToolName(t *testing.T) {
	m := NewModel(&ir.Design{})
	for _, c := range []struct {
		net  *ir.Net
		want bool
	}{
		{&ir.Net{Name: "unconnected-(R1-Pad1)"}, false},
		{&ir.Net{Name: "unconnected-(R1-Pad1)", Attributes: map[string]string{netgraph.AttrNoConnect: "true"}}, true},
		{&ir.Net{Name: "NC_SPARE"}, true},
		{&ir.Net{Name: "no_connect_3"}, true},
	} {
		if got := IntentionallyUnconnected(m, c.net); got != c.want {
			t.Errorf("%s %v: intentional = %v, want %v", c.net.Name, c.net.Attributes, got, c.want)
		}
	}
}
