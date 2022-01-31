// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows
// +build !windows

package main

import "storj.io/private/process"

func main() {
	process.Exec(rootCmd)
}
