package fshost

import (
	"io/fs"

	"github.com/panyam/goutils/mountfs"
)

// Mount is one named tree. A client addresses it by Name in a mounts.URI. It is goutils' mountfs
// mount, which this package's own copy became (agni issue 1761).
type Mount = mountfs.Mount

// Root composes ms into one fs.FS whose top-level directories are the mounts, in name order. A name
// in no mount does not exist, and of two mounts with one name the first wins. The composed FS
// answers ContentSum from the mount that holds a file, so a mount that knows its files' hashes is
// checked by them (agni issue 1515).
func Root(ms []Mount) fs.FS { return mountfs.Root(ms) }
