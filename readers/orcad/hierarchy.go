package orcad

import "fmt"

// occurrence is one part or block along an instantiation path. target is the db id of the placed
// part or block it stands for on its parent page. A block's occurrence names the child schematic
// folder and holds the child's own occurrences.
type occurrence struct {
	target uint32
	folder string // "" for a part
	ref    string // the occurrence's designator, "" when the design annotates instances instead
	nested *scope
}

// scope is the occurrences at one point of an instantiation path.
type scope struct {
	occ map[uint32]*occurrence // by target
}

func (s *scope) find(target uint32) *occurrence {
	if s == nil {
		return nil
	}
	return s.occ[target]
}

// parseHierarchy reads a root schematic's Hierarchy stream, the occurrence tree. It names every
// part's designator along each instantiation path, which an occurrence-annotated design keeps
// nowhere else: a placed part's own field may still read "C?".
func parseHierarchy(b []byte) (*scope, error) {
	c := &cursor{b: b}
	if c.u8() != typeOccurrence {
		return nil, fmt.Errorf("the Hierarchy stream does not open with an occurrence record")
	}
	c.u32()
	c.u32()
	c.lzt() // the view's name
	c.skip(7)
	for n := int(c.u16()); n > 0 && c.err == nil; n-- { // the root's power table
		c.frame()
		c.u32()
		c.lzt()
	}
	s := readScope(c, 0)
	if c.err != nil {
		return nil, fmt.Errorf("the Hierarchy stream: %w", c.err)
	}
	return s, nil
}

// maxDepth bounds the occurrence tree, so a damaged stream cannot recurse without end.
const maxDepth = 64

func readScope(c *cursor, depth int) *scope {
	s := &scope{occ: map[uint32]*occurrence{}}
	if depth > maxDepth {
		c.fail("occurrence tree is more than %d deep", maxDepth)
		return s
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- { // net names along this path
		c.frame()
		c.u32()
		c.lzt()
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- { // title blocks
		c.frame()
		c.skip(8)
	}
	for n := int(c.u32()); n > 0 && c.err == nil; n-- { // globals and off-page connectors
		c.frame()
		c.skip(8)
	}
	if c.err == nil && c.p+4 <= len(c.b) && string(c.b[c.p:c.p+4]) == string(preamble) { // a bare separator
		c.p += 4
		c.skip(int(c.u32()))
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		c.frame()
		c.u32() // the occurrence's own id
		o := &occurrence{target: c.u32()}
		if c.u8() != typeOccurrence && c.err == nil {
			c.fail("occurrence has no inner marker")
		}
		c.skip(8)
		o.folder = c.lzt()
		o.ref = c.lzt()
		c.u32() // the unit this occurrence overrides
		for pins := int(c.u16()); pins > 0 && c.err == nil; pins-- {
			c.frame()
			c.skip(6)
		}
		o.nested = readScope(c, depth+1)
		if c.err == nil {
			s.occ[o.target] = o
		}
	}
	return s
}
