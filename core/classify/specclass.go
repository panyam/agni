package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// StampClassesFromSpecs is the DATASHEET evidence tier of the device-class stamp. For every
// component whose MPN joins a seeded spec carrying a device_class, it adds that class and its family
// tag to ir.Component.device_classes, attributed to CLASS_SOURCE_DATASHEET.
//
// It is its own pass, run after Stamp and StampMPN, because Stamp REPLACES the class set and the join
// key is the MPN that StampMPN fills. It also needs a params corpus, which a read without one never
// has (C9's evidence-tier variant; agni issue 280 named the case, 710 closed it). The ordering is in
// docsite/content/architecture/ingestion-and-ir.md#derived-fields-and-the-tiers-that-fill-them.
//
// Only this tier can subtype a clock source (tokenClasses says why the keyword path refuses) or reach
// ideal_diode_controller, since no netlist labels a FET plus a bias network as one.
//
// ADDITIVE AND IDEMPOTENT. It never removes or downgrades a convention-tier class, and a second run
// changes nothing. Both matter because two callers run it, the Loader when the read carries a corpus
// and check.Model when the model is given one the read did not have.
//
// deviceClassFor answers the vendor's device_class string for an MPN, or "" for a part with no spec
// or no class on it. It is a function rather than the param provider so classify does not import the
// datasheet layer.
func StampClassesFromSpecs(d *ir.Design, deviceClassFor func(mpn string) string) {
	if deviceClassFor == nil {
		return
	}
	for _, c := range d.GetComponents() {
		mpn := c.GetMpn()
		if mpn == "" {
			continue
		}
		// The vendor string is free-form ("SPXO", "ceramic resonator"), so it is normalized before it
		// is expanded, and a spelling variant reaches the same family tag the keyword path produces. An
		// unrecognised value passes through as itself and sorts behind the keyword-derived class.
		cl := NormalizeDeviceClass(deviceClassFor(mpn))
		for _, name := range ClassesOf(cl) {
			AddClassTag(c, name, ir.ClassSource_CLASS_SOURCE_DATASHEET)
		}
	}
}
