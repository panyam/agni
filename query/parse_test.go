package query

import "testing"

// TestParse (WS3-029): the surface syntax parses atoms, comparisons, term kinds, and the
// projection into the IR the evaluator runs.
func TestParse(t *testing.T) {
	q, err := Parse(`component.mpn(?ref,"REG-24"), net.max_voltage(?net,?v), ?v < 30 => ?ref, ?net`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(q.Goal.Literals) != 3 {
		t.Fatalf("literals = %d, want 3", len(q.Goal.Literals))
	}
	// atom with a variable and a string constant
	a0 := q.Goal.Literals[0].Pos
	if a0 == nil || a0.Relation != "component.mpn" || len(a0.Args) != 2 {
		t.Fatalf("literal 0 = %+v, want component.mpn/2", a0)
	}
	if a0.Args[0].Var != "ref" || a0.Args[1].Const == nil || a0.Args[1].Const.S != "REG-24" {
		t.Errorf("component.mpn args = %+v, want ?ref, \"REG-24\"", a0.Args)
	}
	// comparison against a numeric literal (Num set, so it compares numerically)
	c := q.Goal.Literals[2].Compare
	if c == nil || c.Op != "<" || c.Right.Const == nil || c.Right.Const.Num == nil || *c.Right.Const.Num != 30 {
		t.Errorf("comparison = %+v, want ?v < 30 (numeric)", c)
	}
	if len(q.Select) != 2 || q.Select[0].Var != "ref" || q.Select[1].Var != "net" {
		t.Errorf("select = %v, want [?ref ?net]", q.Select)
	}
}

// TestParseNegationAndAggregate (WS3-029 fast-follow): the surface parses a `not R(...)` literal
// and a func(?x) aggregate projection column.
func TestParseNegationAndAggregate(t *testing.T) {
	q, err := Parse(`component.mpn(?r,?m), not param(?m,"VIN",?v) => ?m`)
	if err != nil {
		t.Fatalf("Parse negation: %v", err)
	}
	if q.Goal.Literals[1].Neg == nil || q.Goal.Literals[1].Neg.Relation != "param" {
		t.Errorf("literal 1 = %+v, want a negated param atom", q.Goal.Literals[1])
	}

	q2, err := Parse(`component-on-net(?r,?n) => ?n, count(?r)`)
	if err != nil {
		t.Fatalf("Parse aggregate: %v", err)
	}
	if len(q2.Select) != 2 || q2.Select[0].Var != "n" || q2.Select[1].Agg == nil ||
		q2.Select[1].Agg.Func != "count" || q2.Select[1].Agg.Var != "r" {
		t.Errorf("select = %+v, want [?n count(?r)]", q2.Select)
	}
}

// TestParseErrors (WS3-029): malformed queries are rejected with an error, not silently mis-parsed.
func TestParseErrors(t *testing.T) {
	for name, text := range map[string]string{
		"empty":            ``,
		"bad term":         `component.mpn(bareword) => ?x`,
		"unterminated str": `component.mpn(?r,"REG) => ?r`,
		"bad projection":   `component.mpn(?r,?m) => notavar`,
		"double arrow":     `component.mpn(?r,?m) => ?r => ?m`,
		"junk literal":     `this is not a query`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(text); err == nil {
				t.Errorf("Parse(%q) succeeded; want an error", text)
			}
		})
	}
}
