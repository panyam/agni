package service

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// maskTree is a read mask parsed into the fields it keeps at each level. A field mapped to an empty
// tree keeps that field whole; a non-empty tree keeps only the named sub-fields of it. "*" at a
// level keeps every field there.
type maskTree map[string]maskTree

// parseMask turns field-mask paths into a tree, checking each path against the message it applies
// to. A path may pass through a repeated or map field, and then applies to every element, which is
// what makes `design.components.mpn` mean "every component's part number" (agni issue 836).
func parseMask(md protoreflect.MessageDescriptor, paths []string) (maskTree, error) {
	tree := maskTree{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := checkPath(md, strings.Split(p, ".")); err != nil {
			return nil, fmt.Errorf("%w: read_mask path %q: %v", ErrInvalidArgument, p, err)
		}
		node := tree
		for _, seg := range strings.Split(p, ".") {
			next, ok := node[seg]
			if !ok {
				next = maskTree{}
				node[seg] = next
			}
			node = next
		}
	}
	return tree, nil
}

// checkPath reports whether a path names fields that exist, descending through messages, repeated
// messages and map values.
func checkPath(md protoreflect.MessageDescriptor, segs []string) error {
	if len(segs) == 0 || segs[0] == "*" {
		if len(segs) > 1 {
			return fmt.Errorf("* must end a path")
		}
		return nil
	}
	fd := md.Fields().ByName(protoreflect.Name(segs[0]))
	if fd == nil {
		return fmt.Errorf("%s has no field %q", md.Name(), segs[0])
	}
	if len(segs) == 1 {
		return nil
	}
	sub := fd.Message()
	if fd.IsMap() {
		sub = fd.MapValue().Message()
	}
	if sub == nil {
		return fmt.Errorf("%q is not a message, so nothing is under it", segs[0])
	}
	return checkPath(sub, segs[1:])
}

// prune clears every field of m the tree does not keep, recursing into kept fields that name
// sub-fields. Elements of a repeated or map field are pruned one by one.
func prune(m protoreflect.Message, tree maskTree) {
	if len(tree) == 0 {
		return
	}
	if _, all := tree["*"]; all {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		sub, keep := tree[string(fd.Name())]
		switch {
		case !keep:
			m.Clear(fd)
		case len(sub) == 0:
		case fd.IsMap():
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, e protoreflect.Value) bool {
					prune(e.Message(), sub)
					return true
				})
			}
		case fd.IsList():
			if fd.Message() != nil {
				l := v.List()
				for i := 0; i < l.Len(); i++ {
					prune(l.Get(i).Message(), sub)
				}
			}
		case fd.Message() != nil:
			prune(v.Message(), sub)
		}
		return true
	})
}

// onlyUnder reports whether every path of the tree sits under one top-level field, so a request
// asking only for the IR need not read the drawing.
func onlyUnder(tree maskTree, field string) bool {
	if len(tree) == 0 {
		return false
	}
	for k := range tree {
		if k != field {
			return false
		}
	}
	return true
}

// keepNamed narrows a list to the entries whose name is in want, keeping order. An empty want keeps
// everything.
func keepNamed[T any](items []T, want []string, name func(T) string) []T {
	if len(want) == 0 {
		return items
	}
	set := make(map[string]bool, len(want))
	for _, w := range want {
		set[w] = true
	}
	out := items[:0:0]
	for _, it := range items {
		if set[name(it)] {
			out = append(out, it)
		}
	}
	return out
}
