// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// List of dependencies that should be included in go.mod
// These by default are ignored due to +ignore
import (
	_ "github.com/ckaznocha/protoc-gen-lint"
	_ "golang.org/x/tools/go/packages"
)

func main() {}
