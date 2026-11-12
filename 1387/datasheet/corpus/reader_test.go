package corpus

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"
	"time"
)

// indexed returns the two-spec corpus with its index written in, as `agnids index` leaves it.
func indexed(t *testing.T) fstest.MapFS {
	t.Helper()
	fsys := twoSpecCorpus(t)
	ix, _, err := Refresh(fsys, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ix.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	fsys[IndexFile] = &fstest.MapFile{Data: b, ModTime: time.Unix(1, 0)}
	return fsys
}

// The Reader answers seeded parts through the index, omits unseeded ones, and reports the generation.
func TestReaderAnswersThroughTheIndex(t *testing.T) {
	r, err := NewReader(indexed(t))
	if err != nil {
		t.Fatal(err)
	}
	specs, gen, err := r.BatchGet(context.Background(), []string{"lm1117", "UNSEEDED", "BSS138"})
	if err != nil {
		t.Fatal(err)
	}
	if gen != 1 || len(specs) != 2 {
		t.Fatalf("got %d specs at generation %d; want the 2 seeded parts at generation 1", len(specs), gen)
	}
}

// A corpus with no index cannot be served: every part would read as unseeded.
func TestReaderRefusesAnUnindexedCorpus(t *testing.T) {
	if _, err := NewReader(twoSpecCorpus(t)); !errors.Is(err, ErrNoIndex) {
		t.Fatalf("NewReader over an unindexed corpus = %v, want ErrNoIndex", err)
	}
}

// A file edited or removed since it was indexed is reported as stale, never served unvalidated and
// never skipped as if the part were unseeded.
func TestReaderReportsAStaleFile(t *testing.T) {
	for name, damage := range map[string]func(fstest.MapFS){
		"edited by hand": func(f fstest.MapFS) {
			f["ti/lm1117.textproto"] = &fstest.MapFile{Data: append(fixtureBytes(t, "lm1117.textproto"), '\n')}
		},
		"removed": func(f fstest.MapFS) { delete(f, "ti/lm1117.textproto") },
	} {
		t.Run(name, func(t *testing.T) {
			fsys := indexed(t)
			r, err := NewReader(fsys)
			if err != nil {
				t.Fatal(err)
			}
			damage(fsys)
			if _, _, err := r.BatchGet(context.Background(), []string{"LM1117"}); !errors.Is(err, ErrStale) {
				t.Fatalf("BatchGet = %v, want ErrStale", err)
			}
		})
	}
}

// A promotion rewrites the index, and the Reader picks it up on the next request without a restart.
func TestReaderRereadsAChangedIndex(t *testing.T) {
	fsys := indexed(t)
	r, err := NewReader(fsys)
	if err != nil {
		t.Fatal(err)
	}
	_, draft := draftOf(t, "txb0104.textproto")
	prev, err := Read(fsys)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Promote(draft, fsys, prev)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Index.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	fsys[p.File] = &fstest.MapFile{Data: p.Text}
	fsys[IndexFile] = &fstest.MapFile{Data: b, ModTime: time.Unix(2, 0)}
	specs, gen, err := r.BatchGet(context.Background(), []string{p.Spec.GetMpn()})
	if err != nil {
		t.Fatal(err)
	}
	if gen != 2 || len(specs) != 1 {
		t.Errorf("after promotion: %d specs at generation %d; want the promoted part at generation 2", len(specs), gen)
	}
}
