package service

import (
	"context"
	"errors"
	"testing"

	gproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// A design's snapshot id with no packs is the one agni issue 1695 fixed, so a viewer holding it keeps
// working, and each pack set gives the design another id.
func TestDesignSnapshotIDNamesThePacks(t *testing.T) {
	if got := DesignSnapshotID("designs/gw"); got != "d-2312bc1ff3cef818" {
		t.Errorf("DesignSnapshotID(designs/gw) = %s; want d-2312bc1ff3cef818, the id it had before packs", got)
	}
	one := DesignSnapshotID("designs/gw", "packs/a")
	two := DesignSnapshotID("designs/gw", "packs/a", "packs/b")
	swapped := DesignSnapshotID("designs/gw", "packs/b", "packs/a")
	if one == DesignSnapshotID("designs/gw") || one == two || two == swapped {
		t.Errorf("ids = %s, %s, %s; want each pack set, in its order, to give its own id", one, two, swapped)
	}
}

// A pack refuses config it cannot be read under: an extends, a path out of the pack, or a path no
// file of its own sits at.
func TestAPackRefusesConfigItCannotRead(t *testing.T) {
	files := []*webapi.SnapshotFile{{Path: "profiles/can.yaml"}}
	for name, c := range map[string]struct {
		cfg *webapi.AnalysisConfig
		ok  bool
	}{
		"a folder of its files":   {&webapi.AnalysisConfig{ProfileUris: []string{"profiles"}}, true},
		"one of its files":        {&webapi.AnalysisConfig{ProfileUris: []string{"profiles/can.yaml"}}, true},
		"a mount uri":             {&webapi.AnalysisConfig{ParamUris: []string{"mount://team/params"}}, true},
		"an extends":              {&webapi.AnalysisConfig{Extends: "projects/base"}, false},
		"a path out of the pack":  {&webapi.AnalysisConfig{ProfileUris: []string{"../profiles"}}, false},
		"a path naming no file":   {&webapi.AnalysisConfig{ParamUris: []string{"params"}}, false},
		"a prefix of a file name": {&webapi.AnalysisConfig{ProfileUris: []string{"prof"}}, false},
	} {
		err := checkPack(&webapi.Pack{Config: c.cfg, Files: files})
		if c.ok != (err == nil) || (!c.ok && !errors.Is(err, ErrInvalidArgument)) {
			t.Errorf("%s: checkPack = %v; want ok=%v", name, err, c.ok)
		}
	}
}

// A pack's relative paths become URIs under its own folder, and a mount:// URI stays as written.
func TestAPacksPathsResolveUnderItsFolder(t *testing.T) {
	cfg, err := packConfigURIs(&webapi.Pack{
		Name:   "packs/house",
		Uri:    "mount://uploads/@packs/house/abc123def456",
		Config: &webapi.AnalysisConfig{ProfileUris: []string{"profiles"}, ParamUris: []string{"mount://team/params"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetProfileUris(); len(got) != 1 || got[0] != "mount://uploads/@packs/house/abc123def456/profiles" {
		t.Errorf("profile uris = %v; want the folder under the pack's uri", got)
	}
	if got := cfg.GetParamUris(); len(got) != 1 || got[0] != "mount://team/params" {
		t.Errorf("param uris = %v; want the mount uri as written", got)
	}
	if _, err := packConfigURIs(&webapi.Pack{Name: "packs/x", Config: &webapi.AnalysisConfig{ProfileUris: []string{"profiles"}}}); !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("a relative path on a pack with no files = %v; want ErrFailedPrecondition", err)
	}
}

type packMap map[string]*webapi.Pack

func (m packMap) GetPack(_ context.Context, req *webapi.GetPackRequest) (*webapi.Pack, error) {
	if p, ok := m[req.GetName()]; ok {
		return p, nil
	}
	return nil, ErrNotFound
}

func TestResolvePacksRefusesWhatItCannotRead(t *testing.T) {
	ctx := context.Background()
	if _, err := ResolvePacks(ctx, nil, []string{"packs/a"}); !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("a host with no packs = %v; want ErrFailedPrecondition", err)
	}
	m := packMap{"packs/a": {Name: "packs/a"}}
	if _, err := ResolvePacks(ctx, m, []string{"packs/a", "packs/a"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a pack named twice = %v; want ErrInvalidArgument", err)
	}
	if _, err := ResolvePacks(ctx, m, []string{"packs/none"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a pack the host lacks = %v; want ErrNotFound", err)
	}
}

// Packs layer over the design in order, and the request layers over them. Each nearer tier's
// convention and intent replace the one before, and the lexicon names which tier supplied it.
func TestPacksLayerBetweenTheDesignAndTheRequest(t *testing.T) {
	ctx := context.Background()
	conv := func(name, pat string) *configpb.NamingConvention {
		return &configpb.NamingConvention{Name: name, Lexicon: &configpb.NamingLexicon{
			Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{pat}}}}}
	}
	intentFor := func(domain string) *configpb.DesignIntent {
		return &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{"VCC": {Domain: domain, Nominal: gproto.Float64(3.3)}}}
	}
	design := &webapi.Design{Name: "designs/gw", Config: &webapi.AnalysisConfig{Conventions: conv("board", "^D$"), Intent: intentFor("design")}}
	a := &webapi.Pack{Name: "packs/a", Config: &webapi.AnalysisConfig{Conventions: conv("a", "^A$"), Intent: intentFor("a")}}
	b := &webapi.Pack{Name: "packs/b", Config: &webapi.AnalysisConfig{Checklists: []*webapi.NamedChecklist{{Name: "b"}}}}
	for _, c := range []struct {
		name   string
		packs  []*webapi.Pack
		req    *webapi.OverlayConfig
		conv   string
		src    webapi.LexiconSource
		domain string
	}{
		{"no packs", nil, nil, "board", webapi.LexiconSource_LEXICON_SOURCE_DESIGN, "design"},
		{"one pack", []*webapi.Pack{a}, nil, "a", webapi.LexiconSource_LEXICON_SOURCE_PACK, "a"},
		{"a later pack with no convention keeps the earlier one's", []*webapi.Pack{a, b}, nil, "a", webapi.LexiconSource_LEXICON_SOURCE_PACK, "a"},
		{"the request over the packs", []*webapi.Pack{a}, &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conv("try", "^R$"), Intent: intentFor("request")}},
			"try", webapi.LexiconSource_LEXICON_SOURCE_REQUEST, "request"},
	} {
		ov, err := overlayFor(ctx, nil, nil, nil, design, c.packs, c.req, "")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if ov.Convention.GetName() != c.conv || ov.ConventionSource != c.src {
			t.Errorf("%s: convention = %s from %v; want %s from %v", c.name, ov.Convention.GetName(), ov.ConventionSource, c.conv, c.src)
		}
		if got := ov.DesignIntent.GetNets()["VCC"].GetDomain(); got != c.domain {
			t.Errorf("%s: intent domain = %q; want %q", c.name, got, c.domain)
		}
	}
}

// PackService checks ids, masks and names before it reaches the store, and a server with no store
// says which flag it needs.
func TestPackServiceRefusesBadRequests(t *testing.T) {
	ctx := context.Background()
	none := NewPackService(nil, nil)
	if _, err := none.CreatePack(ctx, &webapi.CreatePackRequest{PackId: "a"}); !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("create with no store = %v; want ErrFailedPrecondition", err)
	}
	if _, err := none.GetPack(ctx, &webapi.GetPackRequest{Name: "packs/a"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("get with no store = %v; want ErrNotFound", err)
	}
	if resp, err := none.ListPacks(ctx, &webapi.ListPacksRequest{}); err != nil || len(resp.GetPacks()) != 0 {
		t.Errorf("list with no store = %v, %v; want empty", resp, err)
	}
	if _, err := none.GetPack(ctx, &webapi.GetPackRequest{Name: "projects/a"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("get projects/a = %v; want ErrInvalidArgument", err)
	}
	svc := NewPackService(&nopPacks{}, nil)
	if _, err := svc.CreatePack(ctx, &webapi.CreatePackRequest{PackId: "Bad Id"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("create with a bad id = %v; want ErrInvalidArgument", err)
	}
	if _, err := svc.UpdatePack(ctx, &webapi.UpdatePackRequest{Pack: &webapi.Pack{Name: "packs/a"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"hash"}}}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("update of hash = %v; want ErrInvalidArgument", err)
	}
}

// nopPacks is a PackStore that holds nothing, for checks that never reach it.
type nopPacks struct{}

func (nopPacks) Pack(context.Context, string) (*webapi.Pack, error) { return nil, ErrNotFound }
func (nopPacks) Packs(context.Context) ([]*webapi.Pack, error)      { return nil, nil }
func (nopPacks) DeletePack(context.Context, string) error           { return ErrNotFound }
func (nopPacks) CreatePack(context.Context, string, *webapi.Pack, BlobSource) (*webapi.Pack, error) {
	return nil, errors.New("unreached")
}
func (nopPacks) UpdatePack(context.Context, string, func(*webapi.Pack) error, BlobSource) (*webapi.Pack, error) {
	return nil, errors.New("unreached")
}

// A stored design keeps its intent alone, and an intent that does not compile fails the write.
func TestAStoredDesignKeepsItsIntentAlone(t *testing.T) {
	if err := checkDesignConfig(&webapi.AnalysisConfig{Intent: &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{"VCC": {Domain: "io", Nominal: gproto.Float64(3.3)}}}}); err != nil {
		t.Errorf("an intent = %v; want it kept", err)
	}
	if err := checkDesignConfig(&webapi.AnalysisConfig{ProfileUris: []string{"mount://x/p"}}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("profiles on a design = %v; want ErrInvalidArgument naming packs", err)
	}
	if err := checkDesignConfig(&webapi.AnalysisConfig{Intent: &configpb.DesignIntent{}}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("an intent declaring nothing = %v; want ErrInvalidArgument", err)
	}
}
