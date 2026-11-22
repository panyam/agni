//go:build windows

package records

import (
	"errors"
	"io/fs"
	"syscall"
)

// errSharingViolation is ERROR_SHARING_VIOLATION, which windows returns when another handle has the
// file open in a mode that forbids the call. Go doesn't map it to fs.ErrPermission.
const errSharingViolation syscall.Errno = 32

// openElsewhere reports whether err is windows refusing a rename, remove or create because another
// handle has the file open, or has just removed it and not yet closed it.
func openElsewhere(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, errSharingViolation)
}
