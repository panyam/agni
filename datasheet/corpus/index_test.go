package corpus

import (
	"strings"
	"testing"
	"testing/fstest"
)

func twoSpecCorpus(t *testing.T) fstest.MapFS {
	return fstest.MapFS{
		"ti/lm1117.textproto":    {Data: fixtureBytes(t, "lm1117.textproto")},
		"nexperia/bss.textproto": {Data: fixtureBytes(t, "bss138.textproto")},
		// A draft beside the seeded files is never indexed, as LoadSet never reads one.
		"ti/LM1117.partspec.json": {Data: []byte(`{}`)},
	}
}

// Build indexes exactly what LoadSet seeds, sorted by MPN, and finds an entry case-insensitively.
func TestBuildIndexesEverySeededSpec(t *testing.T) {
	ix, err := Build(twoSpecCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Entries) != 2 {
		t.Fatalf("entries = %+v, want the two seeded specs and not the draft", ix.Entries)
	}
	if ix.Entries[0].MPN > ix.Entries[1].MPN {
		t.Errorf("entries not sorted by MPN: %s before %s", ix.Entries[0].MPN, ix.Entries[1].MPN)
	}
	e, ok := ix.Lookup("lm1117")
	if !ok || e.File != "ti/lm1117.textproto" || !strings.HasPrefix(e.Hash, "sha256:") {
		t.Errorf("Lookup(lm1117) = %+v, %v", e, ok)
	}
	if _, ok := ix.Lookup("NOT-SEEDED"); ok {
		t.Error("Lookup found an MPN nothing seeds")
	}
}

// A rebuild refuses a corpus LoadSet would refuse, rather than indexing around a bad file.
func TestBuildRefusesWhatLoadSetRefuses(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"a file that does not parse": {"bad.textproto": {Data: []byte("mpn: ")}},
		"one MPN in two files": {
			"a.textproto": {Data: fixtureBytes(t, "lm1117.textproto")},
			"b.textproto": {Data: fixtureBytes(t, "lm1117.textproto")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Build(fsys); err == nil {
				t.Fatal("Build indexed a corpus LoadSet refuses")
			}
		})
	}
}

// The generation is how a reader will learn its cache is stale, so it advances on a change and
// holds still on a rebuild that found nothing new.
func TestRefreshAdvancesTheGenerationOnlyOnAChange(t *testing.T) {
	fsys := twoSpecCorpus(t)
	first, changed, err := Refresh(fsys, nil)
	if err != nil || !changed || first.Generation != 1 {
		t.Fatalf("first index: gen %d, changed %v, err %v; want gen 1, changed", first.Generation, changed, err)
	}
	same, changed, err := Refresh(fsys, first)
	if err != nil || changed || same.Generation != 1 {
		t.Errorf("unchanged corpus: gen %d, changed %v, err %v; want gen 1, unchanged", same.Generation, changed, err)
	}
	// A hand edit changes the bytes, so the hash, so the generation.
	edited := fstest.MapFS{}
	for k, v := range fsys {
		edited[k] = v
	}
	edited["ti/lm1117.textproto"] = &fstest.MapFile{Data: append(fixtureBytes(t, "lm1117.textproto"), []byte("\n# reviewed\n")...)}
	next, changed, err := Refresh(edited, first)
	if err != nil || !changed || next.Generation != 2 {
		t.Errorf("edited corpus: gen %d, changed %v, err %v; want gen 2, changed", next.Generation, changed, err)
	}
	if d := Diff(first, next); len(d) != 1 || !strings.HasPrefix(d[0], "changed LM1117") {
		t.Errorf("Diff = %q, want one changed LM1117", d)
	}
}

// What Marshal writes, Read gives back.
func TestIndexRoundTrips(t *testing.T) {
	ix, _, err := Refresh(twoSpecCorpus(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ix.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(fstest.MapFS{IndexFile: {Data: b}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Generation != ix.Generation || !sameEntries(got.Entries, ix.Entries) {
		t.Errorf("round trip changed the index:\n got %+v\nwant %+v", got, ix)
	}
	if _, err := Read(fstest.MapFS{}); err != ErrNoIndex {
		t.Errorf("Read of an unindexed corpus = %v, want ErrNoIndex", err)
	}
}

// Promotion hands back the index to write: the promoted entry with the hash of the text it writes,
// and the generation after the one already written.
func TestPromoteReturnsTheNextIndex(t *testing.T) {
	_, draft := draftOf(t, "txb0104.textproto")
	fsys := twoSpecCorpus(t)
	prev, _, err := Refresh(fsys, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Promote(draft, fsys, prev)
	if err != nil {
		t.Fatal(err)
	}
	if p.Index.Generation != prev.Generation+1 || len(p.Index.Entries) != 3 {
		t.Fatalf("index after promotion: gen %d, %d entries; want gen %d, 3 entries", p.Index.Generation, len(p.Index.Entries), prev.Generation+1)
	}
	e, ok := p.Index.Lookup(p.Spec.GetMpn())
	if !ok || e.File != p.File || e.Hash != hashOf(p.Text) {
		t.Errorf("promoted entry = %+v, want file %s hashed from the written text", e, p.File)
	}
	// The index Promote returns is the one a rebuild of the written corpus produces.
	fsys[p.File] = &fstest.MapFile{Data: p.Text}
	rebuilt, err := Build(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if !sameEntries(rebuilt.Entries, p.Index.Entries) {
		t.Errorf("promotion's index disagrees with a rebuild:\n%q", Diff(rebuilt, p.Index))
	}
}
