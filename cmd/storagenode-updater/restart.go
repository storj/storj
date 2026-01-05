// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows && !linux && !freebsd && !dragonfly && !netbsd && !openbsd && !solaris && !darwin && service

package main

import (
	"context"

	"github.com/spf13/cobra"
)

func cmdRestart(cmd *cobra.Command, args []string) error {
	return nil
}

func swapBinariesAndRestart(ctx context.Context, standalone bool, restartMethod, service, binaryLocation, newVersionPath, backupPath string) (exit bool, err error) {
	return false, swapBinaries(ctx, binaryLocation, newVersionPath, backupPath)
}
