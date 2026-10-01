package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func houseModule(text string) []LibraryModule {
	return []LibraryModule{{Path: "house", Language: "datalog", Text: text, Source: "mount://p/lib/house.dl"}}
}

const housePMIC = `pmic_rail(?n: net) :- net.rail(?n), str.prefix(?n, "PMIC_");`

// TestAnOverlayWithoutALibraryUsesTheShippedVocabulary pins that nothing changes for a project that
// declares no library: the vocabulary is the process default itself, not a copy of it.
func TestAnOverlayWithoutALibraryUsesTheShippedVocabulary(t *testing.T) {
	reg, err := Overlay{}.Registry()
	if err != nil || reg != facts.DefaultRegistry() {
		t.Errorf("Registry() = %p, %v; want the process default", reg, err)
	}
}

// TestALibraryIsComposedOncePerContent is what keeps a server answering many queries over one
// project from composing and checking its library per query, and what makes an edited library
// compose again.
func TestALibraryIsComposedOncePerContent(t *testing.T) {
	a, err := Overlay{Library: houseModule(housePMIC)}.Registry()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Overlay{Library: houseModule(housePMIC)}.Registry()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("two overlays carrying the same library composed two registries")
	}
	if !a.Vocabulary().Has("house.pmic_rail") {
		t.Error("the composed registry lacks the library's member")
	}
	edited, err := Overlay{Library: houseModule(housePMIC + "\nother(?n: net) :- pmic_rail(?n);")}.Registry()
	if err != nil {
		t.Fatal(err)
	}
	if edited == a || !edited.Vocabulary().Has("house.other") {
		t.Error("an edited library reused the earlier composition")
	}
}

// TestALibraryPageIsServedAndAStrayPageIsRefused covers the optional lib/docs pages: one for a
// member reaches Registry.Doc, and one for a member nothing defines fails composition.
func TestALibraryPageIsServedAndAStrayPageIsRefused(t *testing.T) {
	page := "## house.pmic_rail\n\nThe PMIC's rails.\n"
	reg, err := Overlay{Library: houseModule(housePMIC), LibraryDocs: map[string]string{"house.pmic_rail": page}}.Registry()
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.Doc("house.pmic_rail"); got != page {
		t.Errorf("Doc(house.pmic_rail) = %q, want the library's page", got)
	}
	_, err = Overlay{Library: houseModule(housePMIC), LibraryDocs: map[string]string{"house.pmic_rails": page}}.Registry()
	if err == nil || !strings.Contains(err.Error(), "house.pmic_rails") {
		t.Errorf("err = %v, want a page for an undefined member refused by name", err)
	}
}

func inlineReq(mods ...*webapi.LibraryModule) *webapi.OverlayConfig {
	return &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{LibraryModules: mods}}
}

// TestAnInlineLibraryJoinsTheVocabulary covers agni issue 788 below the wire: modules sent as values
// compose with no resolver, so a host with none (the engine in WASM) still honours them.
func TestAnInlineLibraryJoinsTheVocabulary(t *testing.T) {
	o, err := OverlayFor(context.Background(), nil, nil, nil, nil, inlineReq(&webapi.LibraryModule{Path: "house", Text: housePMIC}), Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := o.Registry()
	if err != nil {
		t.Fatal(err)
	}
	if !reg.Vocabulary().Has("house.pmic_rail") {
		t.Error("the inline module's member is not in the vocabulary")
	}
	none, err := OverlayFor(context.Background(), nil, nil, nil, nil, &webapi.OverlayConfig{}, Overlay{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if reg, _ := none.Registry(); reg.Vocabulary().Has("house.pmic_rail") {
		t.Error("a request with no module still sees house.pmic_rail; the cache leaked a library")
	}
}

// TestModuleTextMovesTheIdentity is the cache half: two requests differing only in a module's text
// must not share a cached result.
func TestModuleTextMovesTheIdentity(t *testing.T) {
	a, okA := idFor(t, project("projects/p"), nil, inlineReq(&webapi.LibraryModule{Path: "house", Text: housePMIC}), nil, "")
	b, okB := idFor(t, project("projects/p"), nil, inlineReq(&webapi.LibraryModule{Path: "house", Text: housePMIC + "\nx(?n: net) :- pmic_rail(?n);"}), nil, "")
	if !okA || !okB {
		t.Fatalf("identity refused: %v %v", okA, okB)
	}
	if a == b {
		t.Error("two libraries differing in text share an identity, so a cache would reuse one's answer for the other")
	}
}

// TestTwoModulesDefiningOneMemberNameBothSources refuses a member path two library modules define,
// naming both, since the vocabulary's own message names neither.
func TestTwoModulesDefiningOneMemberNameBothSources(t *testing.T) {
	_, err := Overlay{Library: []LibraryModule{
		{Path: "house", Language: "datalog", Text: housePMIC, Source: "mount://p/lib/house.dl"},
		{Path: "house", Language: "datalog", Text: `pmic_rail(?n: net) :- net.rail(?n);`, Source: "request:house"},
	}}.Registry()
	if err == nil || !strings.Contains(err.Error(), "mount://p/lib/house.dl") || !strings.Contains(err.Error(), "request:house") {
		t.Errorf("err = %v, want both sources named", err)
	}
}

// TestTheSameModuleTwiceIsOneModule covers a project's lib/ that also arrives with the request, as
// `--lib` naming the project's own directory sends it. Identical text is one module, not a collision.
func TestTheSameModuleTwiceIsOneModule(t *testing.T) {
	reg, err := Overlay{Library: []LibraryModule{
		{Path: "house", Language: "datalog", Text: housePMIC, Source: "mount://p/lib/house.dl"},
		{Path: "house", Language: "datalog", Text: housePMIC, Source: "request:house"},
	}}.Registry()
	if err != nil || !reg.Vocabulary().Has("house.pmic_rail") {
		t.Errorf("the same module twice: reg %v, err %v; want one module", reg != nil, err)
	}
}

// TestListRelationsDescribesAnInlineModule: the catalog reads the overlay a request carries, so a
// module sent with it lists and describes as a query over it would read it.
func TestListRelationsDescribesAnInlineModule(t *testing.T) {
	svc := NewQueryService(fakeLoader{}, nil, nil)
	resp, err := svc.ListRelations(context.Background(), &webapi.ListRelationsRequest{
		Path: "house.pmic_rail", Overlay: inlineReq(&webapi.LibraryModule{Path: "house", Text: housePMIC}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if e := resp.GetEntry(); e.GetEntryKind() != "derived" || e.GetModule() != "house" {
		t.Errorf("entry = %v, want house.pmic_rail as a derived member of house", e)
	}
}
