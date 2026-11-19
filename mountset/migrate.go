package mountset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/panyam/goapplib/storage/blobs"
	"github.com/panyam/goapplib/storage/mounts"
)

// oldRecord is a runtime mount record as agni wrote it before goapplib storage (agni issue 1735). It
// names the commit and lists every file of the snapshot with its hash.
type oldRecord struct {
	Name    string    `yaml:"name"`
	Kind    string    `yaml:"kind"`
	Source  string    `yaml:"source"`
	Ref     string    `yaml:"ref,omitempty"`
	Commit  string    `yaml:"commit"`
	Version string    `yaml:"version"`
	Created time.Time `yaml:"created"`
	Files   []struct {
		Path   string `yaml:"path"`
		Sha256 string `yaml:"sha256"`
	} `yaml:"files"`
}

// newRecord is the record goapplib's fsstore reads, with its keys as fsstore writes them.
type newRecord struct {
	Name     string    `yaml:"name"`
	Kind     string    `yaml:"kind"`
	Source   string    `yaml:"source"`
	Ref      string    `yaml:"ref,omitempty"`
	Version  string    `yaml:"version"`
	Mode     string    `yaml:"mode"`
	Manifest string    `yaml:"manifest"`
	Created  time.Time `yaml:"created"`
}

// migrate rewrites each record an older agni wrote into the form goapplib's fsstore reads (agni
// issue 1759). The commit becomes the version, the mode becomes snapshot, and the file list becomes
// a manifest blob. A record already in the new form is left alone, so migrate runs at every start.
// A record whose files the blobs lost can't be rewritten. It is renamed to <name>.yaml.old and
// reported on log, so the other mounts still serve.
func migrate(ctx context.Context, dir string, b *blobs.Dir, log io.Writer) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path) // #nosec G304 -- a record in the store's own folder
		if err != nil {
			return err
		}
		var old oldRecord
		if err := yaml.Unmarshal(raw, &old); err != nil || old.Version != "" || old.Commit == "" {
			continue
		}
		rec, err := rewrite(ctx, old, b)
		if err != nil {
			if log != nil {
				fmt.Fprintf(log, "note: the mount record %s can't be read by this agni and is kept as %s.old: %v\n", e.Name(), e.Name(), err)
			}
			if err := os.Rename(path, path+".old"); err != nil {
				return err
			}
			continue
		}
		out, err := yaml.Marshal(rec)
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(dir, ".migrate-*")
		if err != nil {
			return err
		}
		if _, err := tmp.Write(out); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmp.Name())
			return err
		}
		if err := os.Rename(tmp.Name(), path); err != nil {
			os.Remove(tmp.Name())
			return err
		}
		if log != nil {
			fmt.Fprintf(log, "note: rewrote the mount record %s for goapplib storage\n", e.Name())
		}
	}
	return nil
}

// rewrite builds the new record for old, storing its file list as a manifest blob. Every file's
// blob must be in b, since a manifest records each file's size.
func rewrite(ctx context.Context, old oldRecord, b *blobs.Dir) (newRecord, error) {
	if err := mounts.ValidName(old.Name); err != nil {
		return newRecord{}, err
	}
	var man blobs.Manifest
	for _, f := range old.Files {
		h, err := blobs.ParseHash(f.Sha256)
		if err != nil {
			return newRecord{}, err
		}
		p, err := b.Path(h)
		if err != nil {
			return newRecord{}, err
		}
		fi, err := os.Stat(p)
		if err != nil {
			return newRecord{}, fmt.Errorf("%s: the uploads lost its blob %s", f.Path, h)
		}
		man = append(man, blobs.ManifestEntry{Path: f.Path, Hash: h, Size: fi.Size()})
	}
	h, err := blobs.PutManifest(ctx, b, man)
	if err != nil {
		return newRecord{}, err
	}
	return newRecord{Name: old.Name, Kind: string(mounts.KindGitHub), Source: old.Source, Ref: old.Ref, Version: old.Commit,
		Mode: string(mounts.ModeSnapshot), Manifest: string(h), Created: old.Created}, nil
}
