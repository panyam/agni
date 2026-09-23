package formats

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestReadDesignStampsDatasheetClasses is the wiring half of agni issue 710: a Loader carrying a
// datasheet corpus stamps the classes only that corpus can establish, so the design every consumer
// receives already knows what its parts are. Before this the enrichment lived inside check.Model, so
// a drawing built from the same design disagreed with a query over it.
//
// It reads through ReadDesign rather than calling the pass, because the ORDER is the part that can
// break: the join key is the MPN a previous pass fills, and the convention stamp REPLACES the set, so
// either one running after this would silently undo it.
func TestReadDesignStampsDatasheetClasses(t *testing.T) {
	const design = "../../cmd/agni/testdata/classproject/designs/clocks/clocks.edn"
	l := &Loader{DeviceClassFor: func(mpn string) string {
		return map[string]string{
			"ACME-XTAL-25M": "crystal",
			"ACME-RESO-16M": "Ceramic Resonator",
			"ACME-PROT-5V":  "tvs",
		}[mpn]
	}}
	d, err := l.ReadDesign(design)
	if err != nil {
		t.Fatalf("ReadDesign: %v", err)
	}
	want := map[string]string{"Y1": "crystal", "Y2": "ceramic_resonator", "D1": "tvs"}
	for _, c := range d.GetComponents() {
		w, ok := want[c.GetRefDes()]
		if !ok {
			continue
		}
		if got := classify.MostSpecific(classify.ClassNames(c)); string(got) != w {
			t.Errorf("%s classified %q, want the datasheet's %q", c.GetRefDes(), got, w)
		}
		found := false
		for _, tag := range c.GetDeviceClasses() {
			if tag.GetClass() == w && tag.GetSource() == ir.ClassSource_CLASS_SOURCE_DATASHEET {
				found = true
			}
		}
		if !found {
			t.Errorf("%s carries %q without recording that a datasheet established it", c.GetRefDes(), w)
		}
	}
}

// TestReadDesignWithoutACorpusIsUnchanged is the positive control for the test above. Without it a
// pass that never fired would read as a pass that had nothing to add, and the assertions would hold
// for the wrong reason.
func TestReadDesignWithoutACorpusIsUnchanged(t *testing.T) {
	const design = "../../cmd/agni/testdata/classproject/designs/clocks/clocks.edn"
	d, err := (&Loader{}).ReadDesign(design)
	if err != nil {
		t.Fatalf("ReadDesign: %v", err)
	}
	want := map[string]string{"Y1": "clock", "Y2": "clock", "D1": "diode", "U1": "ic"}
	for _, c := range d.GetComponents() {
		w, ok := want[c.GetRefDes()]
		if !ok {
			continue
		}
		if got := classify.MostSpecific(classify.ClassNames(c)); string(got) != w {
			t.Errorf("%s classified %q with no corpus, want the convention tier's %q", c.GetRefDes(), got, w)
		}
		for _, tag := range c.GetDeviceClasses() {
			if tag.GetSource() != ir.ClassSource_CLASS_SOURCE_CONVENTION {
				t.Errorf("%s carries %q sourced %v with no corpus attached", c.GetRefDes(), tag.GetClass(), tag.GetSource())
			}
		}
	}
}
