package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
)

// The corpus store keeps drafts beside the published specs (agni issue 749). This is the layout
// every reader of it shares, so the service's store and the status report cannot disagree about
// where a draft lives or what one is. Writing stays with the store that owns it.

// DraftsDir is where drafts live in the corpus. param's corpus walk reads *.textproto only, so a
// draft never seeds a check and never enters the index.
const DraftsDir = "drafts"

// DraftSuffix is a draft file's extension: protojson of a dsapi.Draft, with its version left out,
// since the version is the file's own content hash.
const DraftSuffix = ".draft.json"

// DraftFile is the corpus-relative file a draft for mpn lives in: the MPN upper-cased, since a draft
// is keyed case-insensitively as LoadSet matches, then made file-safe as a published spec's name is.
// The MPN inside the file is what keys it, so two MPNs sharing a name are told apart on read.
func DraftFile(mpn string) string {
	name := strings.TrimSuffix(SpecFileName(strings.ToUpper(strings.TrimSpace(mpn))), ".textproto")
	return path.Join(DraftsDir, name+DraftSuffix)
}

// ReadDraft reads the draft file at a corpus-relative path, with its version set to the file's
// content hash, or reports found=false when there is none.
func ReadDraft(corpus fs.FS, file string) (*dsapi.Draft, bool, error) {
	data, err := fs.ReadFile(corpus, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	d := &dsapi.Draft{}
	if err := protojson.Unmarshal(data, d); err != nil {
		return nil, false, fmt.Errorf("%s: %w", file, err)
	}
	d.Version = hashOf(data)
	return d, true, nil
}

// Drafts returns every draft in the corpus, ordered by MPN. A corpus with no drafts directory has
// none, which is not an error.
func Drafts(corpus fs.FS) ([]*dsapi.Draft, error) {
	entries, err := fs.ReadDir(corpus, DraftsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*dsapi.Draft
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), DraftSuffix) {
			continue
		}
		d, found, err := ReadDraft(corpus, path.Join(DraftsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		if found {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToUpper(out[i].GetMpn()) < strings.ToUpper(out[j].GetMpn()) })
	return out, nil
}

// VersionOf is the version token of a stored file's bytes, its content hash, as a draft's
// compare-and-swap carries it.
func VersionOf(data []byte) string { return hashOf(data) }
