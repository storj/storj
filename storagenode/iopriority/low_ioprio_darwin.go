// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package iopriority

// #include <sys/resource.h>
import "C"

// SetLowIOPriority lowers the process I/O priority.
func SetLowIOPriority() error {
	r1, err := C.setiopolicy_np(C.IOPOL_TYPE_DISK, C.IOPOL_SCOPE_PROCESS, C.IOPOL_THROTTLE)
	if r1 != 0 {
		return err
	}
	return nil
}
