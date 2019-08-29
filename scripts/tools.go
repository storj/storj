// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build tools

package tools

import (
	_ "github.com/AlekSi/gocov-xml"
	_ "github.com/axw/gocov/gocov"
	_ "github.com/ckaznocha/protoc-gen-lint"
	_ "github.com/go-bindata/go-bindata"
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/josephspurrier/goversioninfo"
	_ "github.com/loov/leakcheck"
	_ "github.com/mfridman/tparse"
	_ "github.com/nilslice/protolock/cmd/protolock"
	_ "gopkg.in/spacemonkeygo/dbx.v1"
)
