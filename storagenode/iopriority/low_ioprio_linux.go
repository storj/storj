// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package iopriority

import (
	"syscall"
)

// These constants come from the definitions in linux's ioprio.h.
// See https://github.com/torvalds/linux/blob/61d325dcbc05d8fef88110d35ef7776f3ac3f68b/include/uapi/linux/ioprio.h
const (
	ioprioClassShift uint32 = 13
	ioprioClassMask  uint32 = 0x07
	ioprioPrioMask   uint32 = (1 << ioprioClassShift) - 1

	ioprioWhoProcess uint32 = 1
	ioprioClassBE    uint32 = 2
)

// SetLowIOPriority lowers the process I/O priority.
//
// On linux, this sets the I/O priority to "best effort" with a priority class data of 7.
func SetLowIOPriority() error {
	ioprioPrioValue := ioprioPrioClassValue(ioprioClassBE, 7)
	_, _, err := syscall.Syscall(syscall.SYS_IOPRIO_SET, uintptr(ioprioWhoProcess), 0, uintptr(ioprioPrioValue))
	if err != 0 {
		return err
	}
	return nil
}

// ioprioPrioClassValue returns the class value based on the definition for the IOPRIO_PRIO_VALUE
// macro in Linux's ioprio.h
// See https://github.com/torvalds/linux/blob/61d325dcbc05d8fef88110d35ef7776f3ac3f68b/include/uapi/linux/ioprio.h#L15-L17
func ioprioPrioClassValue(class, data uint32) uint32 {
	return (((class) & ioprioClassMask) << ioprioClassShift) | ((data) & ioprioPrioMask)
}
