package corpus

import (
	"errors"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// fixtureBytes reads one of core/param's seeded fixtures, which are the contract's own examples of a
// valid spec, rather than copying them here.
func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../core/param/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// draftOf renders a seeded fixture as the workbench would save it: protojson.
func draftOf(t *testing.T, fixture string) (*parampb.PartSpec, []byte) {
	t.Helper()
	spec, err := param.Load(strings.NewReader(string(fixtureBytes(t, fixture))))
	if err != nil {
		t.Fatal(err)
	}
	b, err := protojson.MarshalOptions{Multiline: true}.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	return spec, b
}

func TestPromoteWritesAValidDraftAsASeededSpec(t *testing.T) {
	want, draft := draftOf(t, "lm1117.textproto")
	got, err := Promote(draft, fstest.MapFS{}, nil)
	if err != nil {
		t.Fatalf("a valid draft was refused: %v", err)
	}
	if got.File != "LM1117.textproto" || got.Replaces {
		t.Errorf("File = %q, Replaces = %v; want LM1117.textproto into an empty corpus", got.File, got.Replaces)
	}
	// What was written loads back through the same path every check uses, and is the same spec.
	set, err := param.LoadSet(fstest.MapFS{got.File: {Data: got.Text}})
	if err != nil {
		t.Fatalf("the promoted text does not load: %v", err)
	}
	if !proto.Equal(set.Lookup("LM1117"), want) {
		t.Error("the promoted spec does not round-trip: provenance or values changed on the way into the corpus")
	}
}

// A draft is unvalidated on purpose, and the workbench seeds an EMPTY one for every document browsed,
// so the refusal has to name every problem at once rather than the first.
func TestPromoteRefusesADraftValidateRejects(t *testing.T) {
	for name, draft := range map[string][]byte{
		"empty, as the workbench seeds one": []byte(`{}`),
		"a parameter with no provenance": []byte(`{"mpn": "X1", "parameters": [{"name": "VIN",
			"limitKind": "LIMIT_KIND_ABSOLUTE_MAX", "value": {"max": 5}, "unit": "V"}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Promote(draft, fstest.MapFS{}, nil)
			var pe *PromoteError
			if !errors.As(err, &pe) || len(pe.Problems) == 0 {
				t.Fatalf("want a refusal listing the problems, got %v", err)
			}
			if len(pe.Problems) != len(param.Problems(mustDraft(t, draft))) {
				t.Errorf("listed %d problems, Validate finds %d; a refusal must name all of them", len(pe.Problems), len(param.Problems(mustDraft(t, draft))))
			}
		})
	}
}

func mustDraft(t *testing.T, b []byte) *parampb.PartSpec {
	t.Helper()
	s := &parampb.PartSpec{}
	if err := protojson.Unmarshal(b, s); err != nil {
		t.Fatal(err)
	}
	return s
}

// One MPN in two corpus files fails every LoadSet, so promotion refuses to create the second one and
// names the file the corpus already seeds it in.
func TestPromoteRefusesAnMPNAnotherFileSeeds(t *testing.T) {
	_, draft := draftOf(t, "lm1117.textproto")
	corpus := fstest.MapFS{"vendor/ti-ldo.textproto": {Data: fixtureBytes(t, "lm1117.textproto")}}
	_, err := Promote(draft, corpus, nil)
	if err == nil || !strings.Contains(err.Error(), "vendor/ti-ldo.textproto") || !strings.Contains(err.Error(), "LM1117.textproto") {
		t.Fatalf("want a refusal naming both files, got %v", err)
	}
}

// Re-promoting a draft edited after its first promotion replaces the file it went to.
func TestPromoteReplacesItsOwnFile(t *testing.T) {
	_, draft := draftOf(t, "lm1117.textproto")
	corpus := fstest.MapFS{"LM1117.textproto": {Data: fixtureBytes(t, "lm1117.textproto")}}
	got, err := Promote(draft, corpus, nil)
	if err != nil {
		t.Fatalf("re-promotion into its own file was refused: %v", err)
	}
	if !got.Replaces {
		t.Error("Replaces = false for a file that already holds this MPN")
	}
}

// A corpus that does not load now would not load after the write either, and the refusal should
// say the corpus is the problem rather than the draft.
func TestPromoteRefusesACorpusThatDoesNotLoad(t *testing.T) {
	_, draft := draftOf(t, "lm1117.textproto")
	_, err := Promote(draft, fstest.MapFS{"broken.textproto": {Data: []byte("mpn: ")}}, nil)
	if err == nil || !strings.Contains(err.Error(), "corpus does not load") {
		t.Fatalf("want a refusal blaming the corpus, got %v", err)
	}
}

func TestSpecFileNameKeepsAnMPNToOneFile(t *testing.T) {
	for mpn, want := range map[string]string{
		"LM1117":          "LM1117.textproto",
		"LM1117-3.3/NOPB": "LM1117-3.3_NOPB.textproto",
		`A\B C`:           "A_B_C.textproto",
	} {
		if got := SpecFileName(mpn); got != want {
			t.Errorf("SpecFileName(%q) = %q, want %q", mpn, got, want)
		}
	}
}
