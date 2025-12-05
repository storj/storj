// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows

package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"storj.io/storj/storagenode/blobstore"
)

// DiskInfoFromPath returns the disk info for the given path.
func DiskInfoFromPath(path string) (info blobstore.DiskInfo, err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	info, err = getDiskFreeSpace(absPath)
	if err != nil {
		return blobstore.DiskInfo{TotalSpace: -1, AvailableSpace: -1}, err
	}

	return info, nil
}

func getDiskFreeSpace(path string) (info blobstore.DiskInfo, err error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return blobstore.DiskInfo{}, err
	}

	var freeBytesAvailableToCaller, totalNumberOfBytes uint64

	// See https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdiskfreespaceexw
	err = windows.GetDiskFreeSpaceEx(path16, &freeBytesAvailableToCaller, &totalNumberOfBytes, nil)

	info.AvailableSpace = int64(freeBytesAvailableToCaller)
	info.TotalSpace = int64(totalNumberOfBytes)

	return info, err
}

// windows api occasionally returns.
func ignoreSuccess(err error) error {
	if errors.Is(err, windows.Errno(0)) {
		return nil
	}
	return err
}

// Adds `\\?` prefix to ensure that API recognizes it as a long path.
// see https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
func tryFixLongPath(path string) string {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return `\\?\` + abspath
}

// rename implements atomic file rename on windows.
func rename(oldpath, newpath string) error {
	oldpathp, err := windows.UTF16PtrFromString(tryFixLongPath(oldpath))
	if err != nil {
		return &os.LinkError{Op: "replace", Old: oldpath, New: newpath, Err: err}
	}
	newpathp, err := windows.UTF16PtrFromString(tryFixLongPath(newpath))
	if err != nil {
		return &os.LinkError{Op: "replace", Old: oldpath, New: newpath, Err: err}
	}

	err = windows.MoveFileEx(oldpathp, newpathp, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	if err != nil {
		return &os.LinkError{Op: "replace", Old: oldpath, New: newpath, Err: err}
	}

	return nil
}

// rmDir removes the directory named by path.
func rmDir(path string) error {
	pathp, err := windows.UTF16PtrFromString(tryFixLongPath(path))
	if err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	err = windows.RemoveDirectory(pathp)
	if err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	return nil
}

// openFileReadOnly opens the file with read only.
// Custom implementation, because os.Open doesn't support specifying FILE_SHARE_DELETE.
func openFileReadOnly(path string, perm os.FileMode) (*os.File, error) {
	pathp, err := windows.UTF16PtrFromString(tryFixLongPath(path))
	if err != nil {
		return nil, err
	}

	access := uint32(windows.GENERIC_READ)
	sharemode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)

	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1

	createmode := uint32(windows.OPEN_EXISTING)

	handle, err := windows.CreateFile(pathp, access, sharemode, &sa, createmode, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(handle), path), nil
}
