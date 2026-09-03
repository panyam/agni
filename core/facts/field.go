package facts

// Field names the Row field a relation's positional argument binds to, so a flat Row is queried as
// reln(arg0, arg1, ...). A registration reads as, e.g., []Field{FieldSubject, FieldNum} for
// reln(subject, number).
type Field int

const (
	FieldSubject    Field = iota // Row.Subject — the primary entity (net, ref-des, mpn)
	FieldObject                  // Row.Object — the second entity or attribute key
	FieldValue                   // Row.Value — the rendered string value
	FieldNum                     // Row.Num — the numeric value (for range/compare)
	FieldConditions              // Row.Conditions — a parameter's test conditions
	FieldMin                     // Row.Min — the SECOND numeric slot (a two-sided range's lower bound)
	FieldQualifier               // Row.Qualifier — the SECOND string slot (a discriminator when Value is spent)
)

// Label names a field for a synthesized template argument, used where a relation registers a layout
// without supplying its own argument labels.
func (f Field) Label() string {
	switch f {
	case FieldSubject:
		return "subject"
	case FieldObject:
		return "object"
	case FieldValue:
		return "value"
	case FieldNum:
		return "n"
	case FieldConditions:
		return "conditions"
	case FieldMin:
		return "min"
	case FieldQualifier:
		return "qualifier"
	}
	return "arg"
}
