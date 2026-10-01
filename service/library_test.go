package service

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/facts"
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
