// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package iopriority

import (
	"syscall"
)

// These constants come from the definitions in linux's ioprio.h.
// See https://github.com/torvalds/linux/blob/master/include/uapi/linux/ioprio.h
const (
	ioprioClassShift = uint32(13)
	ioprioPrioMask   = (uint32(1) << ioprioClassShift) - 1

	ioprioWhoProcess = 1
	ioprioClassIdle  = 3
)

// SetLowIOPriority lowers the process I/O priority.
func SetLowIOPriority() error {
	// from the definition for the IOPRIO_PRIO_VALUE macro in Linux's ioprio.h
	ioprioPrioValue := ioprioClassIdle<<ioprioClassShift | (0 & ioprioPrioMask)
	_, _, err := syscall.Syscall(syscall.SYS_IOPRIO_SET, uintptr(ioprioWhoProcess), 0, uintptr(ioprioPrioValue))
	if err != 0 {
		return err
	}
	return nil
}
