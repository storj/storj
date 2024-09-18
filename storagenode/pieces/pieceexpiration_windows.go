// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func openHourFile(fileName string) (*os.File, error) {
	utf16Path, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return nil, err
	}
	access := uint32(windows.GENERIC_WRITE | windows.FILE_APPEND_DATA)
	shareMode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	createMode := uint32(windows.OPEN_ALWAYS)
	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	handle, err := syscall.CreateFile(utf16Path, access, shareMode, nil, createMode, attrs, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(handle), fileName)
	return f, nil
}
