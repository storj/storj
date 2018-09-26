// +build windows

package diskstore

import "syscall"

var errSharingViolation = syscall.Errno(32)

func isBusy(err error) bool {
	err = underlyingError(err)
	return err == errSharingViolation
}
