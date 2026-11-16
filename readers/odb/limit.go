package odb

import (
	"errors"
	"io"
)

// maxArchiveBytes bounds what one decoded member of a job may expand to, a compress(1) member or an
// ODX file transcoded to UTF-8. A job is text, and a large board's job runs to a few hundred
// megabytes. So a gigabyte leaves room and still refuses a decompression bomb (C1's input-is-hostile
// rule). An archive holding a job is bounded by internal/archivefs.
const maxArchiveBytes = 1 << 30

func limitedRead(r io.Reader) ([]byte, error) {
	out, err := readAll(io.LimitReader(r, maxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if len(out) > maxArchiveBytes {
		return nil, errors.New("odb: expands past 1 GiB")
	}
	return out, nil
}
