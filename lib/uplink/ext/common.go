// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import (
	"bytes"
	"storj.io/storj/pkg/storj"
	"unsafe"
)

var structRefMap = newMapping()

//CMalloc allocates C memory
func CMalloc(size uintptr) uintptr {
	CMem := C.malloc(C.size_t(size))
	return uintptr(CMem)
}

//export GetIDVersion
func GetIDVersion(number C.uint, cErr **C.char) (cIDVersion C.IDVersion_t) {
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cIDVersion
	}

	return C.IDVersion_t{
		number: C.uint16_t(goIDVersion.Number),
	}
}

//export NewBuffer
func NewBuffer() (cBuffer C.BufferRef_t) {
	return C.BufferRef_t(structRefMap.Add(new(bytes.Buffer)))
}

//export WriteBuffer
func WriteBuffer(cBuffer C.BufferRef_t, cData *C.Bytes_t, cErr **C.char) {
	buf, ok := structRefMap.Get(token(cBuffer)).(*bytes.Buffer)
	if !ok {
		*cErr = C.CString("invalid buffer")
		return
	}

	data := C.GoBytes(unsafe.Pointer(cData.bytes), C.int(cData.length))
	if _, err := buf.Write(data); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

//export ReadBuffer
func ReadBuffer(cBuffer C.BufferRef_t, cData *C.Bytes_t, cErr **C.char) {
	buf, ok := structRefMap.Get(token(cBuffer)).(*bytes.Buffer)
	if !ok {
		*cErr = C.CString("invalid buffer")
		return
	}

	bufLen := buf.Len()
	cData.length = C.int32_t(bufLen)

	//cDataPtr := unsafe.Pointer(cData)
	//cBytesPtr := unsafe.Pointer(cData.bytes)
	//if cDataPtr == nil || cBytesPtr == nil {
	//	*cErr = C.CString("null Bytes_t pointer")
	//	return
	//}
	ptr := CMalloc(uintptr(bufLen))
	mem := unsafe.Pointer(ptr)
	data := buf.Bytes()
	for i := 0; i < bufLen; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = data[i]
	}
	cData.bytes = (*C.uint8_t)(mem)
}
