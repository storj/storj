// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows

package platform

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Adds `\\?` prefix to ensure that API recognizes it as a long path.
// see https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
func tryFixLongPath(path string) string {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return `\\?\` + abspath
}

// CreateFile creates a file in read/write mode that errors if it already exists.
func CreateFile(path string) (*os.File, error) {
	return createFileFlags(path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.CREATE_NEW)
}

// OpenFileReadWrite opens a file in read/write mode.
func OpenFileReadWrite(path string) (*os.File, error) {
	return createFileFlags(path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.OPEN_EXISTING)
}

// OpenFileReadOnly opens a file in read-only mode.
func OpenFileReadOnly(path string) (*os.File, error) {
	return createFileFlags(path, windows.GENERIC_READ, windows.OPEN_EXISTING)
}

func createFileFlags(path string, access, createMode uint32) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(tryFixLongPath(path))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pathPtr,
		access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		createMode,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

// Rename atomically renames a file, replacing the destination if it exists.
func Rename(oldpath, newpath string) error {
	oldpathPtr, err := windows.UTF16PtrFromString(tryFixLongPath(oldpath))
	if err != nil {
		return err
	}
	newpathPtr, err := windows.UTF16PtrFromString(tryFixLongPath(newpath))
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		oldpathPtr,
		newpathPtr,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
