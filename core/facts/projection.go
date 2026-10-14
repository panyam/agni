package facts

import (
	"sync"

	"github.com/panyam/agni/core/check"
)

// A ProjectorGroup is a projector with the relations it emits, so a fact base can run it only when a
// query first reads one of them (agni issue 1000's follow-on). A group emits ONLY the relations it
// lists, and each built-in relation belongs to exactly one group: a relation listed by the wrong group
// would read as empty until its real group happened to run, which is the bug shape, so the relation
// catalog's tests hold every group to what it emits.
type ProjectorGroup struct {
	Relations []string
	Project   Projector
}

// A Projection is one design's fact base computed a relation at a time. Rows(rel) runs the projector
// that owns rel the first time any of its relations is asked for and keeps what it emitted, so a
// query reading two relations never projects the other sixty. It is safe for concurrent use, since
// the query engine reads different relations at once.
//
// A registry whose built-in catalog supplies no Groups projects its whole built-in payload the first
// time any built-in relation is asked for, which is what Rows does.
type Projection struct {
	reg   *Registry
	m     check.Model
	owner map[string]*projected
}

// projected is one projector's run: its rows by relation, filled once.
type projected struct {
	once sync.Once
	run  func() []Row
	rows map[string][]Row
}

// Projection returns m's fact base under this registry, projecting nothing until a relation is read.
func (r *Registry) Projection(m check.Model) *Projection {
	p := &Projection{reg: r, m: m, owner: map[string]*projected{}}
	if r.builtin.Model != nil || len(r.builtin.Groups) > 0 {
		if len(r.builtin.Groups) == 0 {
			whole := &projected{run: func() []Row { return r.builtin.Model(m) }}
			for name := range r.schema {
				if !r.isExtension(name) {
					p.owner[name] = whole
				}
			}
		}
		for _, g := range r.builtin.Groups {
			pr := &projected{run: func() []Row { return g.Project(m) }}
			for _, name := range g.Relations {
				p.owner[name] = pr
			}
		}
	}
	for _, rel := range r.extension {
		name := rel.Name
		p.owner[name] = &projected{run: func() []Row {
			rows := rel.Project(m)
			for i := range rows {
				rows[i].Relation = name
			}
			return rows
		}}
	}
	return p
}

// Rows returns rel's rows for this design, in the order Registry.Rows lists them, or nil when rel is
// not a relation of this registry.
func (p *Projection) Rows(rel string) []Row {
	pr := p.owner[rel]
	if pr == nil {
		return nil
	}
	pr.once.Do(func() {
		pr.rows = map[string][]Row{}
		for _, row := range pr.run() {
			pr.rows[row.Relation] = append(pr.rows[row.Relation], row)
		}
	})
	return pr.rows[rel]
}

func (r *Registry) isExtension(name string) bool {
	for _, rel := range r.extension {
		if rel.Name == name {
			return true
		}
	}
	return false
}
