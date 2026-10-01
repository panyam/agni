package query

import (
	"strconv"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
)

// modelSource is the engine's view of one design, the rows a facts.Registry projects from a
// check.Model served as positional tuples. Below it everything is a facts.Row with named slots, and
// above it everything is a Datalog tuple.
//
// What each relation IS (its arity, labels and types) is the registry's vocabulary, so Schema reads it
// from there and this type only serves rows. env carries the design to the generators registered in
// the fact layer (facts.EnvOf); its Model is nil for a spec-library base and a validation base, which
// have no design, so the circuit walks read nothing through it.
type modelSource struct {
	reg   *facts.Registry
	vocab *ns.Vocabulary
	env   *facts.Env
	rows  map[string][]facts.Row
}

func newModelSource(reg *facts.Registry, m check.Model) *modelSource {
	return &modelSource{reg: reg, vocab: reg.Vocabulary(), env: facts.NewEnv(m), rows: map[string][]facts.Row{}}
}

// Schema implements ns.Source with the vocabulary's own schema, so the Source and the vocabulary
// cannot disagree about a relation's shape.
func (s *modelSource) Schema(rel string) (ns.Schema, bool) {
	if !s.reg.IsRelation(rel) {
		return ns.Schema{}, false
	}
	return s.vocab.Schema(rel)
}

// Tuples implements ns.Source, reading each row's slots in the relation's declared order.
func (s *modelSource) Tuples(rel string) []ns.Tuple {
	fields, _ := s.reg.SchemaOf(rel)
	rows := s.rows[rel]
	out := make([]ns.Tuple, len(rows))
	for i, f := range rows {
		vals := make([]Value, len(fields))
		for j, fld := range fields {
			vals[j] = fieldValue(f, fld)
		}
		out[i] = ns.Tuple{Vals: vals, Cites: f.Cites}
	}
	return out
}

// Relations implements ns.Source with the vocabulary's base relations, in the order a did-you-mean
// hint breaks ties in.
func (s *modelSource) Relations() []string { return s.vocab.BaseRelations() }

// FactsEnv implements facts.EnvSource, which is how a generator registered in the fact layer reaches
// the design.
func (s *modelSource) FactsEnv() *facts.Env { return s.env }

// fieldValue reads one fact Row field as a query Value (string + optional number). The numeric
// field carries both so a bound term serves equality and comparison alike.
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

func ftoa(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// NewBase projects a Model into its fact base over the process-default relation vocabulary. Use
// NewBaseFrom to supply one explicitly.
//
// A binary that installs no relation catalog gets an EMPTY base, which looks like a query that
// matched nothing. Registry.Installed separates the two.
func NewBase(m check.Model) *Base { return NewBaseFrom(facts.DefaultRegistry(), m) }

// NewBaseFrom projects a Model into its fact base over the given relation vocabulary and indexes it
// for querying. The registry is captured, so what this base can answer is fixed at construction.
func NewBaseFrom(reg *facts.Registry, m check.Model) *Base {
	s := newModelSource(reg, m)
	for _, f := range reg.Rows(m) {
		s.rows[f.Relation] = append(s.rows[f.Relation], f)
	}
	return mustBase(s)
}

// NewSpecLibBase builds a fact base over a whole seeded datasheet corpus with NO design (WS10-010).
// The datasheet relations (`param.max`, `part.audience`) project over every PartSpec the FactSource
// yields, so `agni query --speclib` searches the spec library rather than one design's parts.
// Model-dependent relations and predicates (net.*, component.*, net.reaches) yield nothing.
func NewSpecLibBase(fs param.FactSource) *Base {
	return NewSpecLibBaseFrom(facts.DefaultRegistry(), fs)
}

// NewSpecLibBaseFrom is NewSpecLibBase over an explicit relation vocabulary.
func NewSpecLibBaseFrom(reg *facts.Registry, fs param.FactSource) *Base {
	s := newModelSource(reg, nil)
	for _, f := range reg.SpecLibRows(fs.AllSpecs()) {
		s.rows[f.Relation] = append(s.rows[f.Relation], f)
	}
	return mustBase(s)
}

// mustBase pairs the registry's vocabulary with s. The engine refuses a Source that does not serve
// every base relation of the vocabulary, and both halves here come from one registry, so a refusal
// is a programming error in this file.
func mustBase(s *modelSource) *Base { return datalog.MustBase(s.vocab, s) }

// Validate reports why a query cannot run, reading only the query and the relation vocabulary it
// would run against. No design, no rows.
//
// Without it a rule compiled from a query fails only when it runs, where the failure is swallowed into
// a clean pass (agni issue 540). A registry with no relation installed still validates everything that
// needs no vocabulary, because stdlib/profiles compiles its built-in profiles in an init() that runs
// before any relation catalog has registered.
func Validate(q Query, reg *facts.Registry) error {
	return datalog.Validate(q, reg.Vocabulary())
}

// Reads returns the fact-base relations a query references, sorted and deduped. A query-backed rule
// declares these as the facts it reads, and check.Available gates on them for params and board.
func Reads(q Query) []string { return ReadsFrom(facts.DefaultRegistry(), q) }

// ReadsFrom is Reads over an explicit relation vocabulary, for a caller composing its own.
func ReadsFrom(reg *facts.Registry, q Query) []string {
	return datalog.Reads(q, reg.Vocabulary())
}

// ColumnKind says what one answer column denotes. See the engine's documentation.
type ColumnKind = datalog.ColumnKind

// ColumnKinds reports what each answer column of q denotes over the process-default vocabulary, read
// from the signatures its relations and predicates declare and inferred through the query's rules.
func ColumnKinds(q Query) ([]ColumnKind, error) {
	return ColumnKindsFrom(facts.DefaultRegistry(), q)
}

// ColumnKindsFrom is ColumnKinds over an explicit relation vocabulary.
func ColumnKindsFrom(reg *facts.Registry, q Query) ([]ColumnKind, error) {
	return datalog.ColumnKinds(q, reg.Vocabulary())
}

// didYouMean is the engine's unknown-name hint over a relation vocabulary, for the checks in this
// package that name relations outside an evaluation.
func didYouMean(reg *facts.Registry, rel string) string {
	return ns.DidYouMean(reg.Vocabulary(), rel)
}
