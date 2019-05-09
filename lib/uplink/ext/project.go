// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "headers/main.h"
// #endif
import "C"
import (
	// "context"
	// "fmt"
	// "unsafe"
	// "storj.io/storj/lib/uplink"
)

func CreateBucket(project C.Project, name string, cfg uintptr, err *C.char) (b uintptr) {
	// convert project to go type
	// project.CreateBucket
	// check err
	// convert bucket to ptr
	return b
}