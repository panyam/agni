package param

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/core/ident"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/yamlpb"
)

// A pin-mux reference is a part's mux table kept as a file of its own, the shape a board team already
// holds when the table came out of a reference manual rather than a datasheet agni derived (agni issue
// 1200). It is a PartSpec written in YAML holding only pins, the docs they cite and the packages their
// numbers name, so the mux table has no schema beside the one the corpus reads:
//
//	mpn: KW47
//	docs: [{id: kw47-rm, title: KW47 Reference Manual Rev. 2}]
//	packages: [{id: 48qfn, name: 48QFN}]
//	pins:
//	  - id: ptb3
//	    name: PTB3
//	    numbers: [{package_ref: 48qfn, number: "1"}]
//	    alternates: [{name: PTB3}, {name: LPUART1_RX}]
//	    prov: {doc_ref: kw47-rm, page: 58, method: hand, confidence: 1}
//
// It is merged into the MPN's published spec before the corpus sees it, so a corpus keeps one file per
// MPN and nothing that reads a corpus has to join two.

// LoadPinMux parses a pin-mux reference from YAML. Keys are PartSpec's proto field names, and an
// unknown key fails with its line. A reference carrying parameters or relations is refused, because
// those belong to the published spec and a merge that took them would let a mux file override limits.
func LoadPinMux(b []byte) (*parampb.PartSpec, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("pin mux: invalid YAML: %w", err)
	}
	mux := &parampb.PartSpec{}
	if err := yamlpb.Decode(&doc, mux); err != nil {
		return nil, fmt.Errorf("pin mux: %w", err)
	}
	if mux.Mpn == "" {
		return nil, fmt.Errorf("pin mux: no mpn, so it names no part to merge into")
	}
	if len(mux.Parameters) > 0 || len(mux.Relations) > 0 {
		return nil, fmt.Errorf("pin mux for %q carries parameters or relations; a mux reference holds pins, and limits belong in the published spec", mux.Mpn)
	}
	if len(mux.Pins) == 0 {
		return nil, fmt.Errorf("pin mux for %q lists no pins", mux.Mpn)
	}
	for i, p := range mux.Pins {
		if len(p.Alternates) == 0 {
			return nil, fmt.Errorf("pin mux for %q: %s lists no alternates, which would erase the functions a merge is meant to supply", mux.Mpn, pinLabel(p, i))
		}
	}
	return mux, nil
}

// MergePinMux returns base with mux's pins merged in, leaving both arguments unchanged. A nil base
// means the MPN has no published spec yet, and the result is the reference itself.
//
// Each reference pin finds the base pin it describes the way derive joins a mux row to a pin table:
// by id, else by a designator they share in a package, else, for a reference pin with no designators,
// by its name compared through ident.PinName. A matched pin takes the reference's alternates in place
// of its own, so merging a revised table drops functions the revision dropped, and gains any
// designator it lacked; its name, function, prov and id stay as published, since parameters point at
// the id. A reference pin matching nothing is added after the base's pins. Docs and packages join by
// id.
//
// It refuses an MPN that differs from base's, a doc or package whose id the base already uses for
// different content, a reference pin matching several base pins (a repeated name such as NC), and two
// reference pins matching one base pin. The result is not validated here; publishing does that.
func MergePinMux(base, mux *parampb.PartSpec) (*parampb.PartSpec, error) {
	if base == nil {
		return proto.Clone(mux).(*parampb.PartSpec), nil
	}
	if !strings.EqualFold(base.Mpn, mux.Mpn) {
		return nil, fmt.Errorf("pin mux names %q and the spec it merges into names %q", mux.Mpn, base.Mpn)
	}
	out := proto.Clone(base).(*parampb.PartSpec)
	for _, d := range mux.Docs {
		if err := joinByID(&out.Docs, d, func(x *parampb.SourceDoc) string { return x.Id }, "doc"); err != nil {
			return nil, err
		}
	}
	for _, p := range mux.Packages {
		if err := joinByID(&out.Packages, p, func(x *parampb.Package) string { return x.Id }, "package"); err != nil {
			return nil, err
		}
	}
	claimed := map[*parampb.Pin]string{}
	var added []*parampb.Pin
	for i, m := range mux.Pins {
		target, err := muxTarget(out.Pins, m, i)
		if err != nil {
			return nil, err
		}
		if target == nil {
			added = append(added, proto.Clone(m).(*parampb.Pin))
			continue
		}
		if prev, dup := claimed[target]; dup {
			return nil, fmt.Errorf("pin mux: %s and %s both describe published pin %q", prev, pinLabel(m, i), target.Id)
		}
		claimed[target] = pinLabel(m, i)
		target.Alternates = nil
		for _, a := range m.Alternates {
			target.Alternates = append(target.Alternates, proto.Clone(a).(*parampb.PinAlternate))
		}
		for _, n := range m.Numbers {
			if !hasNumber(target, n) {
				target.Numbers = append(target.Numbers, proto.Clone(n).(*parampb.PinNumber))
			}
		}
	}
	out.Pins = append(out.Pins, added...)
	return out, nil
}

// muxTarget finds the base pin a reference pin describes, or nil when it describes a new one.
func muxTarget(pins []*parampb.Pin, m *parampb.Pin, i int) (*parampb.Pin, error) {
	if m.Id != "" {
		for _, p := range pins {
			if p.Id == m.Id {
				return p, nil
			}
		}
	}
	var hits []*parampb.Pin
	if len(m.Numbers) > 0 {
		for _, p := range pins {
			for _, n := range m.Numbers {
				if hasNumber(p, n) {
					hits = append(hits, p)
					break
				}
			}
		}
	} else if key := ident.PinName(m.Name); key != "" {
		for _, p := range pins {
			if ident.PinName(p.Name) == key {
				hits = append(hits, p)
			}
		}
	}
	if len(hits) > 1 {
		ids := make([]string, len(hits))
		for j, h := range hits {
			ids[j] = h.Id
		}
		return nil, fmt.Errorf("pin mux: %s matches published pins %s; give it the id of the one it describes", pinLabel(m, i), strings.Join(ids, ", "))
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	return nil, nil
}

func hasNumber(p *parampb.Pin, n *parampb.PinNumber) bool {
	for _, have := range p.Numbers {
		if have.PackageRef == n.PackageRef && normalizePinNumber(have.Number) == normalizePinNumber(n.Number) {
			return true
		}
	}
	return false
}

func joinByID[T proto.Message](list *[]T, x T, id func(T) string, what string) error {
	for _, have := range *list {
		if id(have) == id(x) {
			if !proto.Equal(have, x) {
				return fmt.Errorf("pin mux declares %s %q differently from the spec it merges into", what, id(x))
			}
			return nil
		}
	}
	*list = append(*list, proto.Clone(x).(T))
	return nil
}

func pinLabel(p *parampb.Pin, i int) string {
	switch {
	case p.Id != "":
		return fmt.Sprintf("pin %q", p.Id)
	case p.Name != "":
		return fmt.Sprintf("pin %q", p.Name)
	}
	return fmt.Sprintf("pins[%d]", i)
}
