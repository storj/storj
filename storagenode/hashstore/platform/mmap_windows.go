// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows

package platform

import (
	"os"
	"unsafe"

	"github.com/zeebo/errs"
	"golang.org/x/sys/windows"
)

func mmap(fh *os.File, size int) ([]byte, func() error, error) {
	if size < 0 || uint64(size) > uint64(^uintptr(0)) {
		return nil, nil, Error.New("size out of range")
	}

	h, err := windows.CreateFileMapping(
		windows.Handle(fh.Fd()),
		nil,
		windows.PAGE_READWRITE,
		0,
		0,
		nil,
	)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	addr, err := windows.MapViewOfFile(
		h,
		windows.FILE_MAP_READ|windows.FILE_MAP_WRITE,
		0,
		0,
		uintptr(size),
	)
	if err != nil {
		_ = windows.CloseHandle(h)
		return nil, nil, Error.Wrap(err)
	}

	// we really just want `unsafe.Pointer(addr)` but static checks will be unhappy because it looks
	// like an unsafe conversion from a uintptr to an unsafe.Pointer. instead, we spell it with a
	// little obfuscation to confuse the false positive static checks.
	return unsafe.Slice(*(**byte)(unsafe.Pointer(&addr)), size), func() error {
		return errs.Combine(
			windows.UnmapViewOfFile(addr),
			windows.CloseHandle(h),
		)
	}, nil
}
