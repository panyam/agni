package formats

import (
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"slices"
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
)

// railVocab builds a role lexicon that treats one extra name as a power rail, standing in for a
// project whose rails are named function-first ("PMIC_VDD_LPM_1V8") and so match none of the
// start-anchored built-ins.
func railVocab(t *testing.T, pattern string) *classify.Lexicon {
	t.Helper()
	rv, err := classify.BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{pattern}}}})
	if err != nil {
		t.Fatalf("BuildRoleVocab: %v", err)
	}
	return &classify.Lexicon{Role: rv}
}

func rolesOf(d *ir.Design, net string) []ir.Role {
	for _, n := range d.GetNets() {
		if n.GetName() == net {
			return classify.NetRoles(n)
		}
	}
	return nil
}

// TestReadDesignStampsPerLoaderLexicon is the property the process global could not provide. Two
// loaders carrying different project conventions read the SAME file in ONE process and stamp
// different net roles. Before WS3-106 the vocabulary lived in a package var, so the second read
// would have overwritten the first's conventions (and on a server, another request's).
func TestReadDesignStampsPerLoaderLexicon(t *testing.T) {
	const fixture = "../edif/testdata/basic.edn"

	plain, err := (&Loader{}).ReadDesign(fixture)
	if err != nil {
		t.Fatalf("ReadDesign (default lexicon): %v", err)
	}
	if got := rolesOf(plain, "SIG"); len(got) != 0 {
		t.Fatalf("SIG roles with the default lexicon = %v, want none (it matches no built-in)", got)
	}

	project, err := (&Loader{Lexicon: railVocab(t, "^SIG$")}).ReadDesign(fixture)
	if err != nil {
		t.Fatalf("ReadDesign (project lexicon): %v", err)
	}
	if got := rolesOf(project, "SIG"); !slices.Contains(got, ir.Role_ROLE_RAIL) {
		t.Errorf("SIG roles with the project lexicon = %v, want it to carry %q", got, ir.Role_ROLE_RAIL)
	}

	// The first read's design is untouched by the second loader, because the vocabulary travelled
	// with each read rather than being installed process-wide.
	if got := rolesOf(plain, "SIG"); len(got) != 0 {
		t.Errorf("the project read leaked into the default read: SIG roles = %v, want none", got)
	}
	if classify.ActiveRoleVocab().IsRail("SIG") {
		t.Error("a per-loader lexicon must not mutate the process-level vocabulary")
	}

	// A name every vocabulary agrees on is stamped the same either way, so the per-read lexicon
	// EXTENDS the conventions rather than replacing them.
	for _, d := range []*ir.Design{plain, project} {
		if got := rolesOf(d, "GND"); !slices.Contains(got, ir.Role_ROLE_GROUND) {
			t.Errorf("GND roles = %v, want it to carry %q under both lexicons", got, ir.Role_ROLE_GROUND)
		}
	}
}

// TestNilLexiconReadsAsDefaults pins the degrade-safe contract three ways: a nil Lexicon field, a
// partially-filled Lexicon (role set, class unset), and a nil *Loader, which ResolveGeometry reaches.
func TestNilLexiconReadsAsDefaults(t *testing.T) {
	const fixture = "../edif/testdata/basic.edn"
	for _, tc := range []struct {
		name string
		l    *Loader
	}{
		{"nil lexicon field", &Loader{}},
		{"lexicon with only the role half", &Loader{Lexicon: &classify.Lexicon{Role: classify.DefaultRoleVocab()}}},
		{"nil loader", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := tc.l.ReadDesign(fixture)
			if err != nil {
				t.Fatalf("ReadDesign: %v", err)
			}
			if got := rolesOf(d, "GND"); !slices.Contains(got, ir.Role_ROLE_GROUND) {
				t.Errorf("GND roles = %v, want the built-in ground vocabulary to still apply", got)
			}
		})
	}
}

// TestParseOnlyStopsBeforeTheConfiguredPasses is the parse level of agni issue 1061: a loader asked
// for the parse returns the design before any vocabulary reaches it, and deriving that parse is the
// whole of ReadDesign, over every reader the tutorial board uses.
func TestParseOnlyStopsBeforeTheConfiguredPasses(t *testing.T) {
	for _, f := range []string{"../edif/testdata/basic.edn", "../../examples/tutorial-project/designs/gateway/gateway.edn", "../kicad/testdata/hier_root.kicad_sch"} {
		read, err := (&Loader{}).ReadDesign(f)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := (&Loader{ParseOnly: true}).ReadDesign(f)
		if err != nil {
			t.Fatal(err)
		}
		if roled, classed := stamped(read); roled == 0 || classed == 0 {
			t.Fatalf("%s: a full read gave %d nets a role and %d parts a class, so an unstamped parse proves nothing", f, roled, classed)
		}
		if roled, classed := stamped(parsed); roled != 0 || classed != 0 {
			t.Errorf("%s: the parse carries %d net roles and %d part classes, so a vocabulary reached it", f, roled, classed)
		}
		DeriveDesign(parsed, nil, nil)
		if !proto.Equal(parsed, read) {
			t.Errorf("%s: deriving the parse differs from ReadDesign", f)
		}
	}
}

func stamped(d *ir.Design) (roled, classed int) {
	for _, n := range d.GetNets() {
		if len(n.GetRoles()) > 0 {
			roled++
		}
	}
	for _, c := range d.GetComponents() {
		if len(c.GetDeviceClasses()) > 0 {
			classed++
		}
	}
	return roled, classed
}
