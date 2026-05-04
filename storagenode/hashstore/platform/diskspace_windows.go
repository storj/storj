// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// GetDiskInfo returns disk space information for the given directory.
// The directory is opened as a handle so that the volume serial number (DiskID)
// is derived from the same filesystem object as the space query.
func GetDiskInfo(dir string) (DiskInfo, error) {
	pathPtr, err := windows.UTF16PtrFromString(tryFixLongPath(dir))
	if err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}

	// FILE_FLAG_BACKUP_SEMANTICS is required to open a directory handle.
	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fileInfo); err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}

	return DiskInfo{
		AvailableSpace: freeBytesAvailable,
		DiskID:         fmt.Sprintf("%08X", fileInfo.VolumeSerialNumber),
	}, nil
}
