// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package main

import "github.com/spf13/cobra"

func startAsService(*cobra.Command) bool {
	return false
}
