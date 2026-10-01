package param

import (
	"strings"
	"testing"
)

// prototext picks one or two spaces after a field name per build. normalizeText collapses exactly that
// run and nothing inside a value, so the corpus text is the same from every binary.
func TestNormalizeTextCollapsesOnlyTheSpaceAfterAFieldName(t *testing.T) {
	in := "mpn:  \"A  B\"\nparameters  {\n  name:  \"x:  y\"\n  [ext.field]:  1\n  value {\n    max: 5\n  }\n}\n"
	want := "mpn: \"A  B\"\nparameters {\n  name: \"x:  y\"\n  [ext.field]: 1\n  value {\n    max: 5\n  }\n}\n"
	if got := string(normalizeText([]byte(in))); got != want {
		t.Errorf("normalizeText:\n got %q\nwant %q", got, want)
	}
	if got := string(normalizeText([]byte(want))); got != want {
		t.Error("normalizeText is not idempotent on already-normal text")
	}
}

func TestMarshalSpecTextHasOneSpaceAfterEveryFieldName(t *testing.T) {
	spec, err := Load(strings.NewReader(string(fixtureBytes(t, "lm1117.textproto"))))
	if err != nil {
		t.Fatal(err)
	}
	b, err := MarshalSpecText(spec)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(b), "\n") {
		if m := fieldSpacing.FindString(line); strings.HasSuffix(m, "  ") {
			t.Errorf("line %d keeps build-dependent spacing: %q", i+1, line)
		}
	}
}
