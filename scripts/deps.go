// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	_ "net/http/httputil"
	_ "testing"

	_ "github.com/gogo/protobuf/types"
	_ "github.com/hanwen/go-fuse/fuse"
	_ "github.com/jtolds/go-luar"
	_ "github.com/jtolds/monkit-hw"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/minio/minio/cmd"
)

func main() {}
