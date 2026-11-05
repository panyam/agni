package report

import (
	"fmt"
	"io"

	"github.com/panyam/agni/core/check"
)

// WriteWalkText renders a walk for a person, hop by hop. Each net says how the walk entered it and
// what else sits on it, a net the walk stopped at says why, and the parts it did not cross come last
// with their reasons.
func WriteWalkText(w io.Writer, wk check.Walk) {
	if wk.Reason != "" {
		fmt.Fprintf(w, "cannot walk: %s\n", wk.Reason)
		return
	}
	start := wk.From.RefDes
	if wk.From.Pin != "" {
		start = endpointLabel(wk.From)
	}
	fmt.Fprintf(w, "walk from %s, %s, %d %s\n", start, wk.Kind, wk.Radius, pluralWord(wk.Radius, "hop", "hops"))
	hop := 0
	for _, n := range wk.Nets {
		if n.Hop != hop {
			hop = n.Hop
			fmt.Fprintf(w, "\nhop %d\n", hop)
		}
		e := n.Entered
		how := fmt.Sprintf("from %s pin %s", e.RefDes, e.ExitPin)
		if e.FromNet != "" {
			how = fmt.Sprintf("through %s%s pin %s --> pin %s, from %s", e.RefDes, classNote(e.Class), e.EnterPin, e.ExitPin, e.FromNet)
		}
		fmt.Fprintf(w, "  net %s   (%s)\n", n.Net.Name, how)
		if n.Ended != "" {
			fmt.Fprintf(w, "      ends here: %s\n", n.Ended)
		}
		writeNetParts(w, n.Net)
	}
	for _, e := range wk.Elided {
		fmt.Fprintf(w, "  and %d more %s at hop %d, past the cap\n", e.Nets, pluralWord(e.Nets, "net", "nets"), e.Hop)
	}
	if len(wk.Boundary) > 0 {
		fmt.Fprintf(w, "\nnot crossed\n")
		for _, b := range wk.Boundary {
			fmt.Fprintf(w, "  %s%s on %s, hop %d: %s\n", b.RefDes, classNote(b.Class), b.Net, b.Hop, b.Reason)
		}
	}
	fmt.Fprintf(w, "\nwalked: %d %s over %d %s, %d %s not crossed\n",
		len(wk.Nets), pluralWord(len(wk.Nets), "net", "nets"), hop, pluralWord(hop, "hop", "hops"),
		len(wk.Boundary), pluralWord(len(wk.Boundary), "part", "parts"))
}
