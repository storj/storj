// +build !windows

package diskstore

import "syscall"

func isBusy(err error) bool {
	err = underlyingError(err)
	return err == syscall.EBUSY
}
