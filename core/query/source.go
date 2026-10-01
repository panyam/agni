package query

import (
	"strconv"
	"sync"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/jaala/datalog"
)

// modelSource is the engine's view of one design, the rows a facts.Registry projects from a
// check.Model served as positional tuples. Below it everything is a facts.Row with named slots, and
// above it everything is a Datalog tuple.
//
// model and netByName are nil for a spec-library base and a validation base, which have no design,
// so the circuit predicates read nothing through them.
type modelSource struct {
	reg       *facts.Registry
	model     check.Model
	netByName map[string]*ir.Net
	rows      map[string][]facts.Row

	schemaMu sync.RWMutex
	schemas  map[string]datalog.Schema

	relsOnce sync.Once
	rels     []string
}

func newModelSource(reg *facts.Registry) *modelSource {
	return &modelSource{
		reg:       reg,
		netByName: map[string]*ir.Net{},
		rows:      map[string][]facts.Row{},
		schemas:   map[string]datalog.Schema{},
	}
}

// Schema implements datalog.Source. The arity is the relation's field layout; the labels and closed
// vocabularies come from its catalog entry, which is what lets the engine refuse a constant such as
// net.role(?n, "swiching") rather than answer "no results" (agni 696). A relation whose catalog entry
// declares no argument kinds gets no labels, so its constants go unchecked.
func (s *modelSource) Schema(rel string) (datalog.Schema, bool) {
	s.schemaMu.RLock()
	sc, ok := s.schemas[rel]
	s.schemaMu.RUnlock()
	if ok {
		return sc, true
	}
	fields, ok := s.reg.SchemaOf(rel)
	if !ok {
		return datalog.Schema{}, false
	}
	sc = datalog.Schema{Arity: len(fields)}
	if info, ok := s.reg.InfoOf(rel); ok && len(info.ArgKinds) > 0 {
		sc.Labels = info.Args
		sc.Domains = make([][]string, len(info.Args))
		for i, label := range info.Args {
			sc.Domains[i] = info.ArgKinds[label].ValidOptions
		}
	}
	s.schemaMu.Lock()
	s.schemas[rel] = sc
	s.schemaMu.Unlock()
	return sc, true
}

// Tuples implements datalog.Source, reading each row's slots in the relation's declared order.
func (s *modelSource) Tuples(rel string) []datalog.Tuple {
	fields, _ := s.reg.SchemaOf(rel)
	rows := s.rows[rel]
	out := make([]datalog.Tuple, len(rows))
	for i, f := range rows {
		vals := make([]Value, len(fields))
		for j, fld := range fields {
			vals[j] = fieldValue(f, fld)
		}
		out[i] = datalog.Tuple{Vals: vals, Cites: f.Cites}
	}
	return out
}

// Relations implements datalog.Source, returning the catalog's names in display order, the order a
// did-you-mean hint breaks ties in. Empty when no relation catalog is installed, so the engine reports
// that rather than guessing at a typo (C29).
func (s *modelSource) Relations() []string {
	s.relsOnce.Do(func() {
		if !s.reg.Installed() {
			return
		}
		cat := CatalogFrom(s.reg)
		s.rels = make([]string, len(cat))
		for i, r := range cat {
			s.rels[i] = r.Name
		}
	})
	return s.rels
}

// NoVocabularyHint implements datalog.NoVocabularyHinter.
func (s *modelSource) NoVocabularyHint() string {
	return "no fact relations are installed (import a relation catalog, e.g. stdlib/relations)"
}

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
	s := newModelSource(reg)
	s.model = m
	for _, f := range reg.Rows(m) {
		s.rows[f.Relation] = append(s.rows[f.Relation], f)
	}
	for _, n := range m.Nets() {
		s.netByName[n.Name] = n
	}
	return datalog.NewBase(s, predicates)
}

// NewSpecLibBase builds a fact base over a whole seeded datasheet corpus with NO design (WS10-010).
// The datasheet relations (`param`, `part.audience`) project over every PartSpec the FactSource
// yields, so `agni query --speclib` searches the spec library rather than one design's parts.
// Model-dependent relations and predicates (net.*, component.*, reaches) yield nothing.
func NewSpecLibBase(fs param.FactSource) *Base {
	return NewSpecLibBaseFrom(facts.DefaultRegistry(), fs)
}

// NewSpecLibBaseFrom is NewSpecLibBase over an explicit relation vocabulary.
func NewSpecLibBaseFrom(reg *facts.Registry, fs param.FactSource) *Base {
	s := newModelSource(reg)
	for _, f := range reg.SpecLibRows(fs.AllSpecs()) {
		s.rows[f.Relation] = append(s.rows[f.Relation], f)
	}
	return datalog.NewBase(s, predicates)
}

// Validate reports why a query cannot run, reading only the query and the relation vocabulary it
// would run against. No design, no rows.
//
// Without it a rule compiled from a query fails only when it runs, where the failure is swallowed into
// a clean pass (agni issue 540). A registry with no relation installed still validates everything that
// needs no vocabulary, because stdlib/profiles compiles its built-in profiles in an init() that runs
// before any relation catalog has registered.
func Validate(q Query, reg *facts.Registry) error {
	return datalog.Validate(q, newModelSource(reg), predicates)
}

// Reads returns the fact-base relations a query references, sorted and deduped. A query-backed rule
// declares these as the facts it reads, and check.Available gates on them for params and board.
func Reads(q Query) []string { return ReadsFrom(facts.DefaultRegistry(), q) }

// ReadsFrom is Reads over an explicit relation vocabulary, for a caller composing its own.
func ReadsFrom(reg *facts.Registry, q Query) []string {
	return datalog.Reads(q, newModelSource(reg))
}

// didYouMean is the engine's unknown-relation hint over a relation vocabulary, for the checks in this
// package that name relations outside an evaluation.
func didYouMean(reg *facts.Registry, rel string) string {
	return datalog.DidYouMean(newModelSource(reg), predicates, rel)
}
