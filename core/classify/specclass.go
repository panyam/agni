package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// StampClassesFromSpecs is the DATASHEET evidence tier of the device-class stamp: for every
// component whose MPN joins a seeded spec carrying a device_class, it adds that class and its family
// tag to ir.Component.device_classes, attributed to CLASS_SOURCE_DATASHEET.
//
// It is a second pass rather than part of classify.Stamp because it cannot run there. The join key is
// the MPN, which StampMPN fills in the same ingestion sweep, and the corpus it joins against only
// exists once a params tier is attached, which a read without one never has. That is C9's
// evidence-tier variant, one shared pass PER TIER, and this is the pass the variant was written for
// (agni issue 280 named the case, 710 closed it).
//
// WHAT ONLY THIS TIER KNOWS. The keyword path deliberately refuses to subtype a clock source, because
// the vendor's own part text is unreliable there (tokenClasses has the measurement: a library named
// "Oscillator" carrying every crystal in it, and a per-part type label swapped in the field). It
// cannot reach ideal_diode_controller at all, since no netlist labels a FET plus a bias network as
// one. So a design read without a corpus is not missing a refinement; it is missing the only evidence
// that resolves those classes.
//
// ADDITIVE AND IDEMPOTENT. It never removes or downgrades what the convention tier established, and
// running it twice, or running it after check.Model has already built its own class set, changes
// nothing. Both properties matter because two callers run it: the Loader when the read carries a
// corpus, and check.Model when the model is given one the read did not have.
//
// deviceClassFor answers the vendor's device_class string for an MPN, or "" for a part with no spec
// or no class on it. A narrow function rather than the param provider itself, so the ingestion layer
// does not take on the datasheet layer (C1): what this pass needs is one string per part.
func StampClassesFromSpecs(d *ir.Design, deviceClassFor func(mpn string) string) {
	if deviceClassFor == nil {
		return
	}
	for _, c := range d.GetComponents() {
		mpn := c.GetMpn()
		if mpn == "" {
			continue
		}
		// The vendor string is free-form ("SPXO", "ceramic resonator"), so it is normalized to a
		// canonical class FIRST and only then expanded, which is what lets a spelling variant reach
		// the same family tag the keyword path produces instead of landing bare. A value the engine
		// does not recognise passes through as itself: an unrecognised class is still a fact about
		// the part, and it sorts behind the keyword-derived one rather than displacing it.
		cl := NormalizeDeviceClass(deviceClassFor(mpn))
		for _, name := range ClassesOf(cl) {
			AddClassTag(c, name, ir.ClassSource_CLASS_SOURCE_DATASHEET)
		}
	}
}
