// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !service || (!windows && !linux && !freebsd && !dragonfly && !netbsd && !openbsd && !solaris && !darwin && service)

package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
)

func cmdRestart(cmd *cobra.Command, args []string) error {
	return nil
}

func swapBinariesAndRestart(ctx context.Context, restartMethod, service, binaryLocation, newVersionPath, backupPath string) (exit bool, err error) {
	if err := os.Rename(binaryLocation, backupPath); err != nil {
		return false, errs.Wrap(err)
	}

	if err := os.Rename(newVersionPath, binaryLocation); err != nil {
		return false, errs.Combine(err, os.Rename(backupPath, binaryLocation), os.Remove(newVersionPath))
	}

	return false, nil
}
