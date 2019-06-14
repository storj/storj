// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
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

func (f *File) Write(buf []byte) (int, error) {
	n := int(C.fwrite(unsafe.Pointer(&buf[0]), C.size_t(1), C.size_t(len(buf)), (*C.FILE)(f)))
	if n > 0 {
		return n, nil
	}

	if f.Eof() != 0 {
		return 0, io.EOF
	}


	return 0, errors.New(C.GoString(C.strerror(C.int(f.Error()))))
}

func (f *File) Seek(off int64, origin int) (int64, error) {
	n := int(C.fseek((*C.FILE)(f), C.long(off), C.int(origin)))
	if n == 0 {
		return f.Tell(), nil
	}

	return 0, errors.New(C.GoString(C.strerror(C.int(f.Error()))))
}

func (f *File) Tell() int64 {
	return int64(C.ftell((*C.FILE)(f)))
}

func (f *File) Eof() int {
	return int(C.feof((*C.FILE)(f)))
}

func (f *File) Error() int {
	return int(C.ferror((*C.FILE)(f)))
}
