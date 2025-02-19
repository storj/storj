// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows

package hashstore

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

func createFile(path string) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(tryFixLongPath(path))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
