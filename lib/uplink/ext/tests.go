// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go run .
//go:generate go test .

package main

// NB: standard go tests cannot import "C"

import "storj.io/storj/lib/uplink/ext/testing"

var AllTests testing.Tests

func init() {
	AllTests.Register(
		testing.NewTest("TestMapping_Add", TestMapping_Add),
		testing.NewTest("TestMapping_Get", TestMapping_Get),
	)
}

func main() {
	AllTests.Run()
}
