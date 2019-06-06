// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
import "C"
import (
	"errors"
	"io"
	"unsafe"
)

type File C.FILE

func (f *File) Read(buf []byte) (int, error) {
	n := int(C.fread(unsafe.Pointer(&buf[0]), C.size_t(1), C.size_t(len(buf)), (*C.FILE)(f)))
	
	if n > 0 {
		return n, nil
	}

	if f.Eof() != 0 {
		return 0, io.EOF
	}

	return 0, errors.New(C.GoString(C.strerror(C.int(f.Error()))))
}

func (f *File) Close() error {
	n := int(C.fclose((*C.FILE)(f)))

	if n != 0 {
		return io.EOF
	}

	return nil
}

func (f *File) Eof() int {
	return int(C.feof((*C.FILE)(f)))
}

func (f *File) Error() int {
	return int(C.ferror((*C.FILE)(f)))
}
