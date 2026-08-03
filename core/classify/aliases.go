// Package classify is the format-neutral component-classification pass (WS3-071). It derives a
// component's device class from cross-format conventions (ref-des prefix, part-type token vocabulary)
// and stamps the normalized result onto the IR at ingestion, so check reads a data fact instead of
// re-parsing vendor strings on every model build. It imports only the generated ir proto and the model
// read-surface contract; check and formats both depend on it.
package classify

import "github.com/panyam/agni/core/model"

// The component-class vocabulary, re-exported from model so the classifier reads bare Class* names.
// A type alias is the same type, so classify.ComponentClass and model.ComponentClass interchange.
type ComponentClass = model.ComponentClass

const (
	ClassResistor         = model.ClassResistor
	ClassCapacitor        = model.ClassCapacitor
	ClassInductor         = model.ClassInductor
	ClassFerrite          = model.ClassFerrite
	ClassDiode            = model.ClassDiode
	ClassLED              = model.ClassLED
	ClassTVS              = model.ClassTVS
	ClassZener            = model.ClassZener
	ClassFuse             = model.ClassFuse
	ClassConnector        = model.ClassConnector
	ClassTestConnector    = model.ClassTestConnector
	ClassTestPoint        = model.ClassTestPoint
	ClassClock            = model.ClassClock
	ClassOscillator       = model.ClassOscillator
	ClassCrystal          = model.ClassCrystal
	ClassCeramicResonator = model.ClassCeramicResonator
	ClassIC               = model.ClassIC
	ClassTransistor       = model.ClassTransistor
	ClassUnknown          = model.ClassUnknown
)
