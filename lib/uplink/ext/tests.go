// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go run .

package main

// NB: standard go tests cannot import "C"

import "storj.io/storj/lib/uplink/ext/testing"

type simple struct {
	Str1  string
	Int2  int
	Uint3 uint
}

type nested struct {
	Simple simple
	Int4   int
}

var AllTests testing.Tests

func init() {
	AllTests.Register(
		testing.NewTest("TestGoToCStruct_success", TestGoToCStruct_success),
		testing.NewTest("TestGoToCStruct_error", TestGoToCStruct_error),
		testing.NewTest("TestCToGoStruct_success", TestCToGoStruct_success),
		testing.NewTest("TestCToGoStruct_error", TestCToGoStruct_error),
	)
}

func main() {
	AllTests.Run()
}
