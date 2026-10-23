package report

import "strings"

// Witness is how one fact behind an answer row holds (agni issue 794), as a document renders it: a
// leaf the design's relations gave, with its citations, or a derived fact with the rule that derived
// it and that rule's body witnesses in the order the rule is written. A `not` that held is a negated
// leaf, which proves an absence and so cites nothing.
type Witness struct {
	Relation string
	// Values are the proved tuple's values as cells spell them; "" is an argument a negation left
	// unbound.
	Values   []string
	Rule     string
	Negated  bool
	Cites    []string
	Children []Witness
}

// witnessIndent is the indentation of one level of a witness tree.
const witnessIndent = "  "

// WitnessLines renders a row's witnesses as an indented tree, one line per fact, each child one level
// deeper than the fact it proves. A derived fact names its rule after `via`, a leaf its citations in
// brackets, and a negated leaf reads `not ... (absent)`.
func WitnessLines(ws []Witness) []string {
	var out []string
	for _, w := range ws {
		out = w.lines(out, 0)
	}
	return out
}

func (w Witness) lines(out []string, depth int) []string {
	vals := make([]string, len(w.Values))
	for i, v := range w.Values {
		if v == "" && w.Negated {
			v = "_"
		}
		vals[i] = v
	}
	line := strings.Repeat(witnessIndent, depth) + w.Relation + "(" + strings.Join(vals, ", ") + ")"
	switch {
	case w.Negated:
		line = strings.Repeat(witnessIndent, depth) + "not " + strings.TrimLeft(line, " ") + "  (absent)"
	case w.Rule != "":
		line += "  via " + strings.Join(strings.Fields(w.Rule), " ")
	}
	if len(w.Cites) > 0 {
		line += "  [" + strings.Join(w.Cites, " ; ") + "]"
	}
	out = append(out, line)
	for _, c := range w.Children {
		out = c.lines(out, depth+1)
	}
	return out
}
