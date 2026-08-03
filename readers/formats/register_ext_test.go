// This file is package formats_test (black-box): it imports the package the way an
// out-of-module consumer does, so it proves the WS12-003 acceptance — an external caller can
// register a reader and have the extension resolve end to end through every derived surface.
package formats_test

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// TestExternalRegistrationResolvesEndToEnd registers a synthetic ".widget" format from outside
// the package and asserts the whole chain lights up: capability predicates, the supported-ext
// list, the UI label, and the Loader dispatch that actually runs the registered reader.
func TestExternalRegistrationResolvesEndToEnd(t *testing.T) {
	const ext = ".widget"
	if formats.ByExt("x"+ext) != nil {
		t.Fatalf("%s already registered; pick an unused synthetic extension", ext)
	}

	sentinel := &ir.Design{IrVersion: "0", SourceFormat: "widget"}
	formats.Register(&formats.Format{
		Ext:  ext,
		Name: "widget",
		Design: func(_ *formats.Loader, _ string) (*ir.Design, error) {
			return sentinel, nil
		},
	})

	if got := formats.NameForExt("board.widget"); got != "widget" {
		t.Errorf("NameForExt = %q, want widget (file-tree label)", got)
	}
	if !formats.HasNetlist("board.widget") {
		t.Error("HasNetlist = false, want true (checks/diff/auto-layout gate)")
	}
	if formats.HasFaithful("board.widget") {
		t.Error("HasFaithful = true, want false (registered with no Geometry)")
	}
	var listed bool
	for _, e := range formats.NetlistExts() {
		if e == ext {
			listed = true
		}
	}
	if !listed {
		t.Errorf("NetlistExts = %v, want it to include %s (supported-ext error text)", formats.NetlistExts(), ext)
	}

	// The dispatch actually runs the registered reader: a zero-value Loader suffices for a
	// reader that opens its own path (this one ignores it and returns the sentinel).
	got, err := (&formats.Loader{}).ReadDesign("anything.widget")
	if err != nil {
		t.Fatalf("ReadDesign(.widget) after Register: %v", err)
	}
	if got != sentinel {
		t.Errorf("ReadDesign returned %v, want the registered reader's sentinel design", got)
	}
}

// TestRegisterRejectsBadEntries pins the Register contract: duplicate extension and malformed
// entries panic (programming errors at startup, matching stdlib registry conventions).
func TestRegisterRejectsBadEntries(t *testing.T) {
	design := func(_ *formats.Loader, _ string) (*ir.Design, error) { return &ir.Design{}, nil }
	cases := map[string]*formats.Format{
		"duplicate built-in ext": {Ext: ".kicad_pcb", Name: "dup", Design: design},
		"missing dot":            {Ext: "widget2", Name: "x", Design: design},
		"uppercase ext":          {Ext: ".Widget2", Name: "x", Design: design},
		"no name":                {Ext: ".widget2", Design: design},
		"no capability":          {Ext: ".widget2", Name: "x"},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Register(%+v) did not panic; want a panic", f)
				} else if !strings.Contains(r.(string), "formats:") {
					t.Errorf("panic = %v, want a formats: message", r)
				}
			}()
			formats.Register(f)
		})
	}
}
