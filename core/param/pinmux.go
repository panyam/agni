package param

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/ident"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// A pin-mux reference is a part's mux table kept as a file of its own, which a board team usually
// produces from a spreadsheet transcribed out of the reference manual (agni issue 1200). It is a
// PartSpec holding only pins, the docs they cite and the packages their numbers name, so the mux table
// has no schema beside the one the corpus reads. The datasheet service reads one from YAML and merges
// it into the MPN's published spec before the corpus sees it, so a corpus keeps one file per MPN and
// nothing that reads a corpus has to join two.

// CheckPinMux refuses a reference that cannot be merged: one naming no MPN or no pins, one carrying
// parameters or relations, since limits belong in the published spec and a merge that took them would
// let a mux file override them, and a pin listing no alternates.
func CheckPinMux(mux *parampb.PartSpec) error {
	if mux.Mpn == "" {
		return fmt.Errorf("pin mux: no mpn, so it names no part to merge into")
	}
	if len(mux.Parameters) > 0 || len(mux.Relations) > 0 {
		return fmt.Errorf("pin mux for %q carries parameters or relations; a mux reference holds pins, and limits belong in the published spec", mux.Mpn)
	}
	if len(mux.Pins) == 0 {
		return fmt.Errorf("pin mux for %q lists no pins", mux.Mpn)
	}
	for i, p := range mux.Pins {
		if len(p.Alternates) == 0 {
			return fmt.Errorf("pin mux for %q: %s lists no alternates, which would leave the pin with nothing to check against", mux.Mpn, pinLabel(p, i))
		}
	}
	return nil
}

// PinMuxMerge says what a merge does with the alternates a matched pin already lists.
type PinMuxMerge struct {
	// Replace drops a matched pin's alternates and takes the reference's in their place, for a
	// revised table that should remove what the revision removed. Without it the two lists are joined,
	// the published ones first and then the reference's new ones in its order, an exact repeat of a
	// name dropped, so two partial tables for one part both land and merging a table twice changes
	// nothing.
	Replace bool
}

// MergePinMux returns base with mux's pins merged in, leaving both arguments unchanged. A nil base
// means the MPN has no published spec yet, and the result is the reference itself.
//
// Each reference pin finds the base pin it describes the way derive joins a mux row to a pin table:
// by id, else by a designator they share in a package, else, for a reference pin with no designators,
// by its name compared through ident.PinName. A matched pin gains the reference's alternates, joined
// with its own or in their place as opts says, and any designator it lacked; its name, function, prov
// and id stay as published, since parameters point at the id. A reference pin matching nothing is
// added after the base's pins. Docs and packages join by id.
//
// It refuses a reference CheckPinMux refuses, an MPN that differs from base's, a doc or package whose
// id the base already uses for different content, a reference pin matching several base pins (a
// repeated name such as NC), and two reference pins matching one base pin. The result is not
// validated here; publishing does that.
func MergePinMux(base, mux *parampb.PartSpec, opts PinMuxMerge) (*parampb.PartSpec, error) {
	if err := CheckPinMux(mux); err != nil {
		return nil, err
	}
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
		if opts.Replace {
			target.Alternates = nil
		}
		have := map[string]bool{}
		for _, a := range target.Alternates {
			have[a.Name] = true
		}
		for _, a := range m.Alternates {
			if !have[a.Name] {
				have[a.Name] = true
				target.Alternates = append(target.Alternates, proto.Clone(a).(*parampb.PinAlternate))
			}
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
