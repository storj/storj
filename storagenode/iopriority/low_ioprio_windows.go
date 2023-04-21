// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package iopriority

import (
	"golang.org/x/sys/windows"
)

// SetLowIOPriority lowers the process I/O priority.
func SetLowIOPriority() (err error) {
	return windows.SetPriorityClass(windows.CurrentProcess(), windows.PROCESS_MODE_BACKGROUND_BEGIN)
}
