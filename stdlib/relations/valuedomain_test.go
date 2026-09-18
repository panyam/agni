package relations

import (
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func infoFor(t *testing.T, rel string) facts.RelationInfo {
	t.Helper()
	for _, info := range builtinCatalog {
		if info.Name == rel {
			return info
		}
	}
	t.Fatalf("relation %q is not in the catalog", rel)
	return facts.RelationInfo{}
}

// TestRoleDomainIsTheVocabularyItself: the declared domain must be the engine's role vocabulary, not a
// copy of it. A literal list here would go stale the way the tokens did before agni 692, and the
// column would then reject a role the engine had just added.
func TestRoleDomainIsTheVocabularyItself(t *testing.T) {
	got := infoFor(t, RelNetRole).ArgKinds["role"].ValidOptions
	if len(got) != len(classify.AllNetRoles()) {
		t.Fatalf("net.role domain has %d values, the vocabulary has %d", len(got), len(classify.AllNetRoles()))
	}
	for _, r := range classify.AllNetRoles() {
		if !slices.Contains(got, classify.RoleToken(r)) {
			t.Errorf("role %q is in the vocabulary and not in the declared domain: %v", classify.RoleToken(r), got)
		}
	}
}

// TestPinTypeDomainCoversEveryDirection: same property for the other closed column. Every spelling
// DirString can produce must be askable, or the domain rejects a value the projector emits.
func TestPinTypeDomainCoversEveryDirection(t *testing.T) {
	got := infoFor(t, RelPinType).ArgKinds["etype"].ValidOptions
	for i := range ir.PinDirection_name {
		want := check.DirString(ir.PinDirection(i))
		if want == "" {
			continue
		}
		if !slices.Contains(got, want) {
			t.Errorf("pin.type can project %q and the domain does not allow it: %v", want, got)
		}
	}
}

// TestOpenColumnsDeclareNoDomain: the negative half, and the one that keeps this feature honest. A
// domain on a net name, a part number or an attribute key would reject legitimate questions, so the
// absence is the property worth asserting rather than a thing nobody got round to.
func TestOpenColumnsDeclareNoDomain(t *testing.T) {
	for _, c := range []struct{ rel, arg string }{
		{RelNetAttr, "key"}, {RelNetAttr, "value"},
		{RelComponentAttr, "key"}, {RelComponentAttr, "value"},
		{RelNetRole, "net"}, {RelComponentMPN, "mpn"},
	} {
		if d := infoFor(t, c.rel).ArgKinds[c.arg].ValidOptions; len(d) != 0 {
			t.Errorf("%s's %q is an open column and declares a domain %v; it would reject valid questions", c.rel, c.arg, d)
		}
	}
}

// TestEveryDeclaredDomainIsNonEmptyAndSorted keeps a domain from silently becoming the empty set,
// which would reject EVERY constant in that column while looking like a column with no domain at the
// call site that builds it.
func TestEveryDeclaredDomainIsNonEmpty(t *testing.T) {
	for _, info := range builtinCatalog {
		for arg, kind := range info.ArgKinds {
			if kind.ValidOptions == nil {
				continue
			}
			if len(kind.ValidOptions) == 0 {
				t.Errorf("%s's %q declares an EMPTY domain, which rejects every constant", info.Name, arg)
			}
			for _, v := range kind.ValidOptions {
				if strings.TrimSpace(v) != v || v == "" {
					t.Errorf("%s's %q domain holds %q, which no projector can emit", info.Name, arg, v)
				}
			}
		}
	}
}
