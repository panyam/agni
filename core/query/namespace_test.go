package query

import (
	"sort"
	"strings"
	"testing"
)

// TestNoNameIsBothAMemberAndAModule holds the namespace rule agni issue 751 adopted: a dot separates
// a module from its member, so no queryable name may also be the module part of another name. `pin`
// beside `pin.net` read either as a relation or as a module, depending on what followed it; the
// relation became `component.pin` so that `pin` is only ever a module.
func TestNoNameIsBothAMemberAndAModule(t *testing.T) {
	names := map[string]bool{}
	for _, r := range Catalog() {
		names[r.Name] = true
	}
	if !names["net.pin_count"] {
		t.Fatal("the catalog lacks net.pin_count, so this sweep would pass over an empty vocabulary")
	}
	var clash []string
	for n := range names {
		parts := strings.Split(n, ".")
		for i := 1; i < len(parts); i++ {
			if prefix := strings.Join(parts[:i], "."); names[prefix] {
				clash = append(clash, prefix+" is a relation and the module of "+n)
			}
		}
	}
	sort.Strings(clash)
	if len(clash) > 0 {
		t.Errorf("names used as both a member and a module:\n  %s", strings.Join(clash, "\n  "))
	}
}
