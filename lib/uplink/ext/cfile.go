// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"errors"
	"io"
	"unsafe"
)


func (f *CFile) Read(buf []byte) (int, error) {
	n := int(CFRead(unsafe.Pointer(&buf[0]), CSize(1), CSize(len(buf)), (*CFile)(f)))
	
	if n > 0 {
		return n, nil
	}

	if f.Eof() != 0 {
		return 0, io.EOF
	}

	return 0, errors.New(CGoString(CStrError(CInt(f.Error()))))
}

func (f *CFile) Close() error {
	n := int(CFClose((*CFile)(f)))

	if n != 0 {
		return io.EOF
	}

	return nil
}

func (f *CFile) Eof() int {
	return int(CFEOF((*CFile)(f)))
}

func (f *CFile) Error() int {
	return int(CFError((*CFile)(f)))
}
