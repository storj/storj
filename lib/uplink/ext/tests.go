// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go test ./main_test.go
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
		testing.NewTest("TestSendToGo_error", TestSendToGo_error),
		testing.NewTest("TestMapping_Add", TestMapping_Add),
		testing.NewTest("TestMapping_Get", TestMapping_Get),
		testing.NewTest("TestSendToGo_success", TestSendToGo_success),
	)
}

func main() {
	AllTests.Run()
}
