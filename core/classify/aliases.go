// Package classify holds the format-neutral ingestion passes that stamp derived facts onto the IR, so
// check reads a data fact instead of re-parsing vendor strings on every model build. The first was the
// component-classification pass (WS3-071), which derives a device class from ref-des prefix and
// part-type tokens. The package also owns the net-role vocabulary, the value parser and the MPN stamp.
// It imports the generated ir and config protos and the model read-surface contract, and check and
// formats both depend on it.
package classify

import "github.com/panyam/agni/core/model"

// The component-class vocabulary, re-exported from model so the classifier reads bare Class* names.
// A type alias is the same type, so classify.ComponentClass and model.ComponentClass interchange.
type ComponentClass = model.ComponentClass

const (
	ClassResistor             = model.ClassResistor
	ClassCapacitor            = model.ClassCapacitor
	ClassInductor             = model.ClassInductor
	ClassFerrite              = model.ClassFerrite
	ClassThermistor           = model.ClassThermistor
	ClassDiode                = model.ClassDiode
	ClassLED                  = model.ClassLED
	ClassTVS                  = model.ClassTVS
	ClassZener                = model.ClassZener
	ClassFuse                 = model.ClassFuse
	ClassConnector            = model.ClassConnector
	ClassTestConnector        = model.ClassTestConnector
	ClassInternalConnector    = model.ClassInternalConnector
	ClassTestPoint            = model.ClassTestPoint
	ClassClock                = model.ClassClock
	ClassOscillator           = model.ClassOscillator
	ClassCrystal              = model.ClassCrystal
	ClassCeramicResonator     = model.ClassCeramicResonator
	ClassIC                   = model.ClassIC
	ClassTransistor           = model.ClassTransistor
	ClassIdealDiodeController = model.ClassIdealDiodeController
	ClassUnknown              = model.ClassUnknown
)
