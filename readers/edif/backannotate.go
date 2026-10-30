package edif

// backAnnotatedRefDes maps an instance id to the designator the design's
// (viewMap (instanceBackAnnotate (instanceRef ID ...) (designator "R1001"))) gives it (agni issue
// 1228). An export can renumber parts after layout and push the new ref-des here while the page
// instance keeps its old inline (designator "R4"), or carries none at all, so both the netlist read
// and the schematic geometry read take the back-annotated one. Taking it in one of the two reads
// alone would key a .eds's placements and its components differently.
//
// Only a top-level instance is mapped. An instanceRef nesting another instanceRef names an instance
// inside a block, and neither read keys a placement by that path. An id annotated with two different
// designators maps to neither, since nothing in the file says which one is current.
func backAnnotatedRefDes(root *node) map[string]string {
	dn := findFirst(root, "design")
	if dn == nil {
		return nil
	}
	out := map[string]string{}
	conflicted := map[string]bool{}
	for _, vm := range dn.Children("viewMap") {
		for _, ba := range vm.Children("instanceBackAnnotate") {
			ref := ba.Child("instanceRef")
			if ref == nil || ref.Child("instanceRef") != nil {
				continue
			}
			id := parseName(ref.Arg(1)).ID
			des := stringDisplayText(ba.Child("designator"))
			if id == "" || des == "" || conflicted[id] {
				continue
			}
			if prev, ok := out[id]; ok && prev != des {
				delete(out, id)
				conflicted[id] = true
				continue
			}
			out[id] = des
		}
	}
	return out
}
