package classify

import (
	"sort"
	"strings"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// attrVocab is the compiled attribute vocabulary, the keys each fact is read from in preference order
// and the values that mark a part not fitted. See configpb.AttributeVocab.
type attrVocab struct {
	mpn, value, dnp, dnpValues []string
}

func buildAttrVocab(a *configpb.AttributeVocab) attrVocab {
	return attrVocab{
		mpn: a.GetMpn().GetKeys(), value: a.GetValue().GetKeys(),
		dnp: a.GetDnp().GetKeys(), dnpValues: a.GetDnpValues().GetKeys(),
	}
}

// componentAttr reads keys off a component's own attributes and then its sections', key by key, so a
// reader that keeps a house field on the section (KiCad, agni issue 1285) reaches the same keys.
func componentAttr(c *ir.Component, keys []string) string {
	for _, k := range keys {
		if v := attrValue(c.GetAttributes(), []string{k}); v != "" {
			return v
		}
		for _, s := range c.GetSections() {
			if v := attrValue(s.GetAttributes(), []string{k}); v != "" {
				return v
			}
		}
	}
	return ""
}

// attrValue returns the non-empty value of the first of keys an attribute map carries, comparing keys
// without regard to case, or "". Two attributes differing only in case are tried in sorted order, so
// the answer never depends on map order.
func attrValue(attrs map[string]string, keys []string) string {
	if len(attrs) == 0 {
		return ""
	}
	names := make([]string, 0, len(attrs))
	for k := range attrs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, want := range keys {
		for _, k := range names {
			if strings.EqualFold(k, want) && strings.TrimSpace(attrs[k]) != "" {
				return attrs[k]
			}
		}
	}
	return ""
}

// IsUnfitted reports whether a component carries a do-not-populate flag: an attribute.dnp key whose
// value is one of attribute.dnp_values. Every key is tried, so KiCad's own dnp flag set to "no" does
// not hide a house DNP field saying the part is not fitted. A section's attributes count too.
func (v *RoleVocab) IsUnfitted(c *ir.Component) bool {
	if v.unfittedBy(c.GetAttributes()) {
		return true
	}
	for _, s := range c.GetSections() {
		if v.unfittedBy(s.GetAttributes()) {
			return true
		}
	}
	return false
}

func (v *RoleVocab) unfittedBy(attrs map[string]string) bool {
	for k, val := range attrs {
		if containsFold(v.attrs.dnp, k) && containsFold(v.attrs.dnpValues, strings.TrimSpace(val)) {
			return true
		}
	}
	return false
}

// ValueText returns the value text a component carries under attribute.value, or "" when it carries
// none.
func (v *RoleVocab) ValueText(c *ir.Component) string {
	return componentAttr(c, v.attrs.value)
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// mergeKeys is mergeVocab for an attribute list.
func mergeKeys(base, over *configpb.AttributeKeys) *configpb.AttributeKeys {
	if over == nil || (len(over.GetKeys()) == 0 && !over.GetReplace()) {
		return base
	}
	if over.GetReplace() {
		return over
	}
	return &configpb.AttributeKeys{Keys: append(append([]string{}, base.GetKeys()...), over.GetKeys()...)}
}

func mergeAttributes(base, over *configpb.AttributeVocab) *configpb.AttributeVocab {
	return &configpb.AttributeVocab{
		Mpn:       mergeKeys(base.GetMpn(), over.GetMpn()),
		Value:     mergeKeys(base.GetValue(), over.GetValue()),
		Dnp:       mergeKeys(base.GetDnp(), over.GetDnp()),
		DnpValues: mergeKeys(base.GetDnpValues(), over.GetDnpValues()),
	}
}
