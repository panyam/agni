package param

import (
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The parameter contract's enums, rendered as the lowercase string tokens every surface prints.
//
// They live here rather than beside any one consumer because more than one surface now spells them:
// the query relations publish `param.range`'s kind and `param.pin`'s function as tuple values, and
// `agni params` prints the same vocabulary in a rendered record. A second private copy would be the
// drift shape agni issue 380 records, where two renderers written apart stopped agreeing. A user
// moving between `agni query` and `agni params` has to see one vocabulary.
//
// A token is part of the CLI contract, since a datalog query matches on the literal
// (`param.range(?m, ?s, "absolute_max", ?min, ?max)`). Renaming one breaks saved queries, so treat
// these strings as data rather than as presentation.

// LimitKindToken renders a parameter's LimitKind. UNSPECIFIED fails param.Validate, so
// "unspecified" here means a spec that never reached a corpus.
func LimitKindToken(k parampb.LimitKind) string {
	switch k {
	case parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX:
		return "absolute_max"
	case parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING:
		return "recommended_operating"
	case parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC:
		return "characteristic"
	default:
		return "unspecified"
	}
}

// PinFunctionToken renders a pin's PinFunction. Unlike a limit kind, "unspecified" is a LEGAL and
// common answer: a document may print a pin table with no type column at all, and a pin whose name
// and number are known is still worth recording.
func PinFunctionToken(f parampb.PinFunction) string {
	switch f {
	case parampb.PinFunction_PIN_FUNCTION_POWER_INPUT:
		return "power_input"
	case parampb.PinFunction_PIN_FUNCTION_POWER_OUTPUT:
		return "power_output"
	case parampb.PinFunction_PIN_FUNCTION_GROUND:
		return "ground"
	case parampb.PinFunction_PIN_FUNCTION_INPUT:
		return "input"
	case parampb.PinFunction_PIN_FUNCTION_OUTPUT:
		return "output"
	case parampb.PinFunction_PIN_FUNCTION_BIDIRECTIONAL:
		return "bidirectional"
	case parampb.PinFunction_PIN_FUNCTION_PASSIVE:
		return "passive"
	case parampb.PinFunction_PIN_FUNCTION_NO_CONNECT:
		return "no_connect"
	default:
		return "unspecified"
	}
}

// ModalityToken renders a pin relation's Modality. UNSPECIFIED gets its own token rather than being
// omitted, because a bound whose modal verb was never recorded differs from one that has none, and
// a query filtering on modality must be able to find it.
func ModalityToken(m parampb.Modality) string {
	switch m {
	case parampb.Modality_MODALITY_REQUIRED:
		return "required"
	case parampb.Modality_MODALITY_RECOMMENDED:
		return "recommended"
	default:
		return "unspecified"
	}
}
