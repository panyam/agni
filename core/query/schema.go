package query

import "github.com/panyam/agni/core/facts"

// fieldValue reads one fact Row field as a query Value (string + optional number). The numeric
// field carries both so a bound term serves equality and comparison alike.
//
// This is the whole impedance match between the fact layer and the evaluator: above it everything is
// datalog, below it everything is a plain Go struct. A relation's positional layout (facts.SchemaOf)
// says which field each argument binds to; this says what that field means as a logic value.
func fieldValue(f facts.Row, fld facts.Field) Value {
	switch fld {
	case facts.FieldSubject:
		return Value{S: f.Subject}
	case facts.FieldObject:
		return Value{S: f.Object}
	case facts.FieldValue:
		return Value{S: f.Value}
	case facts.FieldNum:
		if f.Num != nil {
			return Value{S: ftoa(*f.Num), Num: f.Num, BaseUnit: f.BaseUnit}
		}
		return Value{Absent: true}
	case facts.FieldConditions:
		return Value{S: f.Conditions}
	case facts.FieldQualifier:
		return Value{S: f.Qualifier}
	case facts.FieldMin:
		if f.Min != nil {
			return Value{S: ftoa(*f.Min), Num: f.Min, BaseUnit: f.BaseUnit}
		}
		return Value{Absent: true}
	}
	return Value{Absent: true}
}
