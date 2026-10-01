package param

import (
	"context"
	"errors"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func setOf(specs ...*parampb.PartSpec) ParamSet {
	s := ParamSet{}
	for _, sp := range specs {
		s[sp.GetMpn()] = sp
	}
	return s
}

// The project decides every part it seeds, the shared corpus answers for the rest, and each spec is
// attributed to the corpus that actually answered, including an MPN both seed.
func TestLayeredProjectWinsPerMPN(t *testing.T) {
	projLDO := &parampb.PartSpec{Mpn: "LDO"}
	sharedLDO := &parampb.PartSpec{Mpn: "LDO"}
	sharedBuck := &parampb.PartSpec{Mpn: "BUCK"}
	l := Layered{{Name: CorpusProject, Provider: setOf(projLDO)}, {Name: CorpusShared, Provider: setOf(sharedLDO, sharedBuck)}}

	if got := l.Lookup("ldo"); got != projLDO {
		t.Error("an MPN both corpora seed was not answered by the project")
	}
	if got := l.Lookup("BUCK"); got != sharedBuck {
		t.Error("a part only the shared corpus seeds was not answered by it")
	}
	if l.Lookup("NONE") != nil {
		t.Error("an unseeded part was answered")
	}
	if c := l.CorpusOf(projLDO); c != CorpusProject {
		t.Errorf("CorpusOf(project's LDO) = %q", c)
	}
	if c := l.CorpusOf(sharedBuck); c != CorpusShared {
		t.Errorf("CorpusOf(shared BUCK) = %q", c)
	}
	// The shared LDO is shadowed, so no Lookup hands it to a model, but asked about directly it still
	// belongs to the shared corpus: attribution is by the object, never by the MPN.
	if c := l.CorpusOf(sharedLDO); c != CorpusShared {
		t.Errorf("CorpusOf(the shared corpus's own LDO) = %q, want shared", c)
	}
}

// A nil layer is skipped, and a remote layer's prefetch error is the Layered's.
func TestLayeredSkipsNilAndPrefetchesThrough(t *testing.T) {
	f := corpusOf("BUCK")
	f.err = errors.New("connection refused")
	l := Layered{{Name: CorpusProject, Provider: nil}, {Name: CorpusShared, Provider: NewRemote(f, 0)}}
	if err := l.Prefetch(context.Background(), []string{"BUCK"}); err == nil {
		t.Error("an unreachable shared corpus was not reported through the layering")
	}
	f.err = nil
	if err := l.Prefetch(context.Background(), []string{"BUCK"}); err != nil {
		t.Fatal(err)
	}
	if l.Lookup("BUCK") == nil {
		t.Error("the remote layer's prefetched spec was not answered")
	}
}
