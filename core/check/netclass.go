package check

import (
	"math"
	"slices"
	"sort"
	"strconv"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// NetClassCascade resolves what a project DECLARED for a net from its net-class definitions, the way
// KiCad composes an effective netclass: PER FIELD, not per class. A net's classes are sorted by
// priority ascending, the Default class pinned last and applying to every net including unclassed
// ones, and each field comes from the first class that states it. So a net in a high-priority class
// declaring only a clearance still takes its track width from the next class down, and there is no
// single winning class to pick.
//
// It is the one implementation the net.declared_* relations and the netclass-conformance rules share.
// Each used to carry its own, and the two agreed only because nobody had edited either (agni 698).
type NetClassCascade struct {
	byName       map[string]*ir.Constraint
	defaultClass string
}

// NewNetClassCascade indexes a design's net-class definitions (Model.NetClassDefs). With none, every
// lookup reports nothing stated.
func NewNetClassCascade(defs []*ir.Constraint) *NetClassCascade {
	c := &NetClassCascade{byName: make(map[string]*ir.Constraint, len(defs))}
	for _, d := range defs {
		c.byName[d.GetName()] = d
		if c.defaultClass == "" && d.GetParams()["is_default"] == "true" {
			c.defaultClass = d.GetName()
		}
	}
	return c
}

// Declared resolves one numeric field for a net belonging to classes: the value from the first class
// in cascade order that states it, and that class's name, so a finding can say whose limit it is.
// ok is false when no class states the field, which is a net the project did not constrain rather
// than one that passes. A class the net names but the project never defined states nothing.
func (c *NetClassCascade) Declared(classes []string, param string) (value float64, class string, ok bool) {
	for _, cls := range c.order(classes) {
		s := c.byName[cls].GetParams()[param]
		if s == "" {
			continue
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v, cls, true
		}
	}
	return 0, "", false
}

func (c *NetClassCascade) order(classes []string) []string {
	out := append([]string(nil), classes...)
	sort.SliceStable(out, func(i, j int) bool { return c.priority(out[i]) < c.priority(out[j]) })
	if c.defaultClass != "" && !slices.Contains(out, c.defaultClass) {
		out = append(out, c.defaultClass)
	}
	return out
}

// priority reads a class's cascade rank. An undefined class, or one stating no rank, sorts last: it
// states nothing about its rank, so it must never outrank a class that does.
func (c *NetClassCascade) priority(class string) int {
	p, err := strconv.Atoi(c.byName[class].GetParams()["priority"])
	if err != nil {
		return math.MaxInt32
	}
	return p
}
