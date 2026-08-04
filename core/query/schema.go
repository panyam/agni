package query

// edbField names a FactRow field a relation's positional argument binds to.
type edbField int

const (
	fSubject edbField = iota
	fObject
	fValue
	fNum
	fConditions
	fMin // the SECOND numeric slot (FactRow.Min) — a two-sided relation's lower bound (param.range)
)

// edbSchema maps each fact-base relation to its positional argument layout, so a flat FactRow is
// queried as reln(arg0, arg1, ...). It is the engine's index of relation shapes; the built-in
// relations' layouts are installed at init by stdlib/relations through RegisterBuiltinFacts (issue
// 10), and overlay relations add theirs through RegisterRelation. Relations the evaluator computes
// rather than looks up (reaches) are NOT here.
var edbSchema = map[string][]edbField{}

// fieldValue reads one FactRow field as a query Value (string + optional number). The numeric
// field carries both so a bound term serves equality and comparison alike.
func fieldValue(f FactRow, fld edbField) Value {
	switch fld {
	case fSubject:
		return Value{S: f.Subject}
	case fObject:
		return Value{S: f.Object}
	case fValue:
		return Value{S: f.Value}
	case fNum:
		if f.Num != nil {
			return Value{S: ftoa(*f.Num), Num: f.Num}
		}
		return Value{}
	case fConditions:
		return Value{S: f.Conditions}
	case fMin:
		if f.Min != nil {
			return Value{S: ftoa(*f.Min), Num: f.Min}
		}
		return Value{}
	}
	return Value{}
}
