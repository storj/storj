// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package iopriority

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// SetLowIOPriority lowers the process I/O priority.
func SetLowIOPriority() (err error) {
	err = windows.SetPriorityClass(windows.CurrentProcess(), windows.PROCESS_MODE_BACKGROUND_BEGIN)

	var errNo syscall.Errno
	if errors.As(err, &errNo) {
		if errNo == windows.ERROR_PROCESS_MODE_ALREADY_BACKGROUND {
			return nil
		}
	}

	return err
}
