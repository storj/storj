// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO: (turn back on) go:generate go test ./main_test.go
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
		testing.NewTest("TestSendToGo_success", TestSendToGo_success),
		testing.NewTest("TestSendToGo_error", TestSendToGo_error),
	)
}

func main() {
	AllTests.Run()
}
