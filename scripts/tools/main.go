// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	_ "github.com/ckaznocha/protoc-gen-lint"
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "github.com/josephspurrier/goversioninfo"
	_ "github.com/mfridman/tparse"
	_ "github.com/nilslice/protolock/cmd/protolock"

	_ "github.com/AlekSi/gocov-xml"
	_ "github.com/axw/gocov/gocov"

	_ "gopkg.in/spacemonkeygo/dbx.v1"
)

func main() {}
