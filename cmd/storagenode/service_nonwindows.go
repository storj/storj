// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows
// +build !windows

package main

func startAsService() bool {
	return false
}
