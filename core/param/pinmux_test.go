package param

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/yamlpb"
)

// A published spec as a pin function table leaves it: three pins with designators, one with stale
// alternates, two sharing the name NC, and a parameter bound to a pin by id.
const muxBase = `
mpn: "KW47"
docs { id: "ds" title: "KW47 data sheet Rev. 4" }
packages { id: "48qfn" name: "48QFN" }
pins { id: "ptb3" name: "PTB3" numbers { package_ref: "48qfn" number: "1" } prov { doc_ref: "ds" page: 58 method: "hand" confidence: 1 } }
pins { id: "ptb4" name: "PTB4" numbers { package_ref: "48qfn" number: "2" } alternates { name: "OLD_FN" } prov { doc_ref: "ds" page: 59 method: "hand" confidence: 1 } }
pins { id: "ptc1" name: "PTC1" prov { doc_ref: "ds" page: 60 method: "hand" confidence: 1 } }
pins { id: "nc1" name: "NC" numbers { package_ref: "48qfn" number: "40" } prov { doc_ref: "ds" page: 60 method: "hand" confidence: 1 } }
pins { id: "nc2" name: "NC" numbers { package_ref: "48qfn" number: "41" } prov { doc_ref: "ds" page: 60 method: "hand" confidence: 1 } }
parameters {
  name: "VDD" symbol: "VDD" limit_kind: LIMIT_KIND_RECOMMENDED_OPERATING pin_refs: "ptb3"
  value { min: 1.71 max: 3.6 } unit: "V"
  prov { doc_ref: "ds" page: 20 method: "hand" confidence: 1 }
}
`

func muxBaseSpec(t *testing.T) *parampb.PartSpec {
	t.Helper()
	spec := &parampb.PartSpec{}
	if err := prototext.Unmarshal([]byte(muxBase), spec); err != nil {
		t.Fatal(err)
	}
	if err := Validate(spec); err != nil {
		t.Fatalf("the base fixture must be a valid spec: %v", err)
	}
	return spec
}

// mustLoadPinMux reads a reference from YAML as the datasheet service does, without CheckPinMux, so a
// test can hand MergePinMux a reference it must refuse.
func mustLoadPinMux(t *testing.T, y string) *parampb.PartSpec {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(y), &doc); err != nil {
		t.Fatal(err)
	}
	mux := &parampb.PartSpec{}
	if err := yamlpb.Decode(&doc, mux); err != nil {
		t.Fatal(err)
	}
	return mux
}

func pinByID(spec *parampb.PartSpec, id string) *parampb.Pin {
	for _, p := range spec.Pins {
		if p.Id == id {
			return p
		}
	}
	return nil
}

func altNames(p *parampb.Pin) []string {
	var out []string
	for _, a := range p.Alternates {
		out = append(out, a.Name)
	}
	return out
}

func TestMergePinMuxJoinsByIDDesignatorAndName(t *testing.T) {
	base := muxBaseSpec(t)
	before := proto.Clone(base)
	mux := mustLoadPinMux(t, `
mpn: kw47
docs: [{id: rm, title: KW47 Reference Manual Rev. 2}]
pins:
  - id: ptb3
    alternates: [{name: PTB3}, {name: LPUART1_RX}]
  - name: renamed-but-same-leg
    numbers: [{package_ref: 48qfn, number: "2"}]
    alternates: [{name: PTB4}, {name: LPI2C1_SDA}]
  - name: ptc_1
    alternates: [{name: PTC1}, {name: TPM0_CH1}]
`)
	got, err := MergePinMux(base, mux, PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("the merged spec must validate: %v", err)
	}
	for id, want := range map[string][]string{
		"ptb3": {"PTB3", "LPUART1_RX"},
		"ptb4": {"OLD_FN", "PTB4", "LPI2C1_SDA"},
		"ptc1": {"PTC1", "TPM0_CH1"},
	} {
		if got := altNames(pinByID(got, id)); !slices.Equal(got, want) {
			t.Errorf("%s alternates = %v, want %v", id, got, want)
		}
	}
	if n := len(got.Pins); n != 5 {
		t.Errorf("every reference pin matched a published one, so the merge must add none; got %d pins", n)
	}
	if p := pinByID(got, "ptb4"); p.Name != "PTB4" || p.Prov.GetPage() != 59 {
		t.Errorf("a matched pin keeps its published name and prov, got name %q page %d", p.Name, p.Prov.GetPage())
	}
	if len(got.Parameters) != 1 || got.Parameters[0].PinRefs[0] != "ptb3" {
		t.Errorf("the published parameter and its pin binding must survive the merge, got %v", got.Parameters)
	}
	if len(got.Docs) != 2 {
		t.Errorf("the reference's doc joins the published one, got %d docs", len(got.Docs))
	}
	if !proto.Equal(base, before) {
		t.Errorf("MergePinMux changed its base argument")
	}
}

func TestMergePinMuxAddsAPinTheSpecLacks(t *testing.T) {
	mux := mustLoadPinMux(t, `
mpn: KW47
docs: [{id: rm, title: KW47 Reference Manual Rev. 2}]
pins:
  - id: ptd0
    name: PTD0
    numbers: [{package_ref: 48qfn, number: "12"}]
    alternates: [{name: PTD0}, {name: LPSPI0_SCK}]
    prov: {doc_ref: rm, page: 101, method: hand, confidence: 1}
`)
	got, err := MergePinMux(muxBaseSpec(t), mux, PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("the merged spec must validate: %v", err)
	}
	if last := got.Pins[len(got.Pins)-1]; last.Id != "ptd0" || !slices.Equal(altNames(last), []string{"PTD0", "LPSPI0_SCK"}) {
		t.Errorf("an unmatched reference pin is added after the published ones, got %v", last)
	}
}

func TestMergePinMuxAddedPinWithoutProvFailsValidate(t *testing.T) {
	mux := mustLoadPinMux(t, `
mpn: KW47
pins:
  - {id: ptd0, name: PTD0, alternates: [{name: PTD0}]}
`)
	got, err := MergePinMux(muxBaseSpec(t), mux, PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(got); err == nil || !strings.Contains(err.Error(), "ptd0: no prov") {
		t.Errorf("a new pin with no prov must fail Validate naming it, got %v", err)
	}
}

func TestMergePinMuxJoinsAlternatesUnlessToldToReplace(t *testing.T) {
	first := mustLoadPinMux(t, `
mpn: KW47
pins: [{id: ptb4, alternates: [{name: PTB4}, {name: DROPPED_IN_REV_B}]}]
`)
	revised := mustLoadPinMux(t, `
mpn: KW47
pins: [{id: ptb4, alternates: [{name: PTB4}, {name: LPI2C1_SDA}]}]
`)
	once, err := MergePinMux(muxBaseSpec(t), first, PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	if got := altNames(pinByID(once, "ptb4")); !slices.Equal(got, []string{"OLD_FN", "PTB4", "DROPPED_IN_REV_B"}) {
		t.Errorf("a join keeps the published alternates first and adds the new ones in order, got %v", got)
	}
	joined, err := MergePinMux(once, revised, PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	if got := altNames(pinByID(joined, "ptb4")); !slices.Equal(got, []string{"OLD_FN", "PTB4", "DROPPED_IN_REV_B", "LPI2C1_SDA"}) {
		t.Errorf("a second table joins without repeating a name, got %v", got)
	}
	replaced, err := MergePinMux(once, revised, PinMuxMerge{Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := altNames(pinByID(replaced, "ptb4")); !slices.Equal(got, []string{"PTB4", "LPI2C1_SDA"}) {
		t.Errorf("Replace takes the revision's alternates in place of the published ones, got %v", got)
	}
	for _, opts := range []PinMuxMerge{{}, {Replace: true}} {
		prev, err := MergePinMux(once, revised, opts)
		if err != nil {
			t.Fatal(err)
		}
		again, err := MergePinMux(prev, revised, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(again, prev) {
			t.Errorf("Replace=%v: merging the same table again must change nothing", opts.Replace)
		}
	}
}

func TestMergePinMuxWithNoPublishedSpec(t *testing.T) {
	mux := mustLoadPinMux(t, `
mpn: KW47
docs: [{id: rm, title: KW47 Reference Manual Rev. 2}]
pins:
  - {id: ptb3, name: PTB3, alternates: [{name: PTB3}], prov: {doc_ref: rm, page: 58, method: hand, confidence: 1}}
`)
	got, err := MergePinMux(nil, mux, PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(got); err != nil {
		t.Errorf("a complete reference is a valid spec of its own: %v", err)
	}
	if got == mux {
		t.Errorf("MergePinMux must not return its argument")
	}
}

func TestMergePinMuxRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, mux, want string
	}{
		{"another part", `
mpn: KW45
pins: [{id: ptb3, alternates: [{name: PTB3}]}]`, `names "KW45"`},
		{"a doc id reused for another document", `
mpn: KW47
docs: [{id: ds, title: Something else}]
pins: [{id: ptb3, alternates: [{name: PTB3}]}]`, `doc "ds" differently`},
		{"a repeated name", `
mpn: KW47
pins: [{name: NC, alternates: [{name: NC}]}]`, "matches published pins nc1, nc2"},
		{"two reference pins on one published pin", `
mpn: KW47
pins:
  - {id: ptb3, alternates: [{name: PTB3}]}
  - {name: x, numbers: [{package_ref: 48qfn, number: "1"}], alternates: [{name: PTB3}]}`, `both describe published pin "ptb3"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := MergePinMux(muxBaseSpec(t), mustLoadPinMux(t, tc.mux), PinMuxMerge{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestMergePinMuxRefusesAnUnmergeableReference(t *testing.T) {
	for _, tc := range []struct {
		name, y, want string
	}{
		{"no mpn", "pins: [{id: ptb3, alternates: [{name: PTB3}]}]\n", "no mpn"},
		{"no pins", "mpn: KW47\n", "lists no pins"},
		{"a pin with no alternates", "mpn: KW47\npins: [{id: ptb3}]\n", `pin "ptb3" lists no alternates`},
		{"limits", "mpn: KW47\nparameters: [{name: VDD}]\npins: [{id: ptb3, alternates: [{name: PTB3}]}]\n", "parameters or relations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, base := range []*parampb.PartSpec{nil, muxBaseSpec(t)} {
				_, err := MergePinMux(base, mustLoadPinMux(t, tc.y), PinMuxMerge{})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("base %v: want an error containing %q, got %v", base != nil, tc.want, err)
				}
			}
		})
	}
}
