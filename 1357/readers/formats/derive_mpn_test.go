package formats

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/classify"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A host keeps one parse across projects, so the part number pass runs in DeriveDesign (agni issues
// 1061, 1285). Deriving that parse under two lexicons answers each lexicon's part numbers.
func TestDeriveReadsPartNumbersWithItsOwnLexicon(t *testing.T) {
	parsed := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Attributes: map[string]string{"Manufacturer_No": "TPS62133"}}}}
	v, err := classify.BuildRoleVocab(&configpb.NamingLexicon{Attribute: &configpb.AttributeVocab{
		Mpn: &configpb.AttributeKeys{Keys: []string{"Manufacturer_No"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	plain := proto.Clone(parsed).(*ir.Design)
	DeriveDesign(plain, classify.DefaultLexicon(), nil)
	house := proto.Clone(parsed).(*ir.Design)
	DeriveDesign(house, &classify.Lexicon{Role: v}, nil)
	if got := plain.Components[0].GetMpn(); got != "" {
		t.Errorf("the default lexicon read a house key: %q", got)
	}
	if got := house.Components[0].GetMpn(); got != "TPS62133" {
		t.Errorf("the house lexicon's derive read %q", got)
	}
}
