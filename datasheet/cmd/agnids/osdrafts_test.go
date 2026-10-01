package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/dsservice"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func draftFor(mpn string, docs ...string) *dsapi.Draft {
	return &dsapi.Draft{Mpn: mpn, Spec: &parampb.PartSpec{Mpn: mpn}, DocumentUris: docs}
}

// Save is compare-and-swap on the draft's MPN: a first save asserts absence, a later one must carry
// the version it read, and Get gives back that version.
func TestOSDraftStoreCAS(t *testing.T) {
	st := newOSDraftStore(t.TempDir())
	ctx := context.Background()

	if d, found, err := st.Get(ctx, "LM1117"); err != nil || found || d != nil {
		t.Fatalf("absent => %v %v %v", d, found, err)
	}
	if _, err := st.Save(ctx, draftFor("LM1117"), "stale"); !errors.Is(err, dsservice.ErrConflict) {
		t.Fatalf("first save with a non-empty base should conflict, got %v", err)
	}
	v1, err := st.Save(ctx, draftFor("LM1117", "mount://ds/ti/LM1117.pdf"), "")
	if err != nil || v1 == "" {
		t.Fatalf("first save: %q %v", v1, err)
	}
	got, found, err := st.Get(ctx, "lm1117")
	if err != nil || !found || got.GetMpn() != "LM1117" || got.GetVersion() != v1 {
		t.Fatalf("readback (case-insensitive): found=%v draft=%v err=%v; want version %q", found, got, err, v1)
	}
	if _, err := st.Save(ctx, draftFor("LM1117"), ""); !errors.Is(err, dsservice.ErrConflict) {
		t.Fatalf("an empty base against an existing draft should conflict, got %v", err)
	}
	if v2, err := st.Save(ctx, draftFor("LM1117"), v1); err != nil || v2 == v1 {
		t.Fatalf("cas update: %q %v", v2, err)
	}
	if _, err := os.Stat(filepath.Join(st.dir, "drafts", "LM1117.draft.json")); err != nil {
		t.Fatalf("draft not under the corpus's drafts/: %v", err)
	}
}

// Opening a datasheet finds every draft citing it, in MPN order, and none that cite another.
func TestOSDraftStoreListsADocumentsDrafts(t *testing.T) {
	st := newOSDraftStore(t.TempDir())
	ctx := context.Background()
	family := "mount://ds/ti/lm1117-family.pdf"
	for _, d := range []*dsapi.Draft{draftFor("LM1117-5.0", family), draftFor("LM1117-3.3", family), draftFor("TXB0104", "mount://ds/ti/txb.pdf")} {
		if _, err := st.Save(ctx, d, ""); err != nil {
			t.Fatal(err)
		}
	}
	ds, err := st.ListByDocument(ctx, family)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 || ds[0].GetMpn() != "LM1117-3.3" || ds[1].GetMpn() != "LM1117-5.0" {
		t.Errorf("drafts citing the family datasheet = %v", ds)
	}
}

// Two MPNs that share a file name once unsafe characters are replaced are different drafts: the
// second is refused rather than overwriting the first, and neither reads as the other.
func TestOSDraftStoreKeepsCollidingMPNsApart(t *testing.T) {
	st := newOSDraftStore(t.TempDir())
	ctx := context.Background()
	if _, err := st.Save(ctx, draftFor("A/B"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Save(ctx, draftFor("A_B"), ""); err == nil {
		t.Fatal("a draft whose file name collides with another MPN's was saved over it")
	}
	if _, found, _ := st.Get(ctx, "A_B"); found {
		t.Error("A_B read A/B's draft")
	}
}

// Publishing validates the draft and writes the published spec and index; a draft that fails is
// refused with its problems and nothing is written.
func TestOSDraftStorePublishes(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	st := newOSDraftStore(dir)
	ctx := context.Background()
	p, err := st.Publish(ctx, "ACME-LDO-1V8")
	if err != nil || p.Generation != 2 || p.Replaced {
		t.Fatalf("Publish = %+v, %v; want generation 2, not a replacement", p, err)
	}
	set, err := param.LoadSet(os.DirFS(dir))
	if err != nil || set.Lookup("ACME-LDO-1V8") == nil {
		t.Fatalf("the published spec does not load: %v", err)
	}

	if _, err := st.Save(ctx, &dsapi.Draft{Mpn: "EMPTY-PART", Spec: unprovenanced("EMPTY-PART")}, ""); err != nil {
		t.Fatal(err)
	}
	_, err = st.Publish(ctx, "EMPTY-PART")
	var refused *dsservice.PublishRefused
	if !errors.As(err, &refused) || len(refused.Problems) == 0 {
		t.Fatalf("an empty draft published: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "EMPTY-PART.textproto")); !errors.Is(err, os.ErrNotExist) {
		t.Error("a refused publish wrote a spec")
	}
	if _, err := st.Publish(ctx, "NO-SUCH-DRAFT"); err == nil {
		t.Error("publishing an MPN with no draft succeeded")
	}
}

// unprovenanced is a draft spec Validate rejects: a parameter with no provenance, the shape
// corpus's own refusal tests use. A spec carrying only an MPN validates, so it cannot stand in.
func unprovenanced(mpn string) *parampb.PartSpec {
	s := &parampb.PartSpec{}
	if err := protojson.Unmarshal([]byte(`{"mpn": "`+mpn+`", "parameters": [{"name": "VIN",
		"limitKind": "LIMIT_KIND_ABSOLUTE_MAX", "value": {"max": 5}, "unit": "V"}]}`), s); err != nil {
		panic(err)
	}
	return s
}
