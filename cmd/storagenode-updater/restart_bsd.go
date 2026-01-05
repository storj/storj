// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build darwin || freebsd || dragonfly || netbsd || openbsd || solaris

package main

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
)

func cmdRestart(cmd *cobra.Command, args []string) error {
	return nil
}

func swapBinariesAndRestart(ctx context.Context, standalone bool, restartMethod, service, binaryLocation, newVersionPath, backupPath string) (exit bool, err error) {
	if err := swapBinaries(ctx, binaryLocation, newVersionPath, backupPath); err != nil {
		return false, errs.Wrap(err)
	}
	if standalone {
		return false, nil
	}

	if service == updaterServiceName {
		return true, nil
	}

	if restartMethod == "service" {
		c := exec.Command("service", service, "restart")
		output, err := c.CombinedOutput()
		if err != nil {
			return false, errs.New("Couldn't restart %s service: %s %v", service, string(output), err)
		}
		return false, nil
	}

	if err := stopProcess(service); err != nil {
		err = errs.New("error stopping %s service: %v", service, err)
		return false, errs.Combine(err, os.Rename(backupPath, binaryLocation))
	}

	return false, nil
}

func stopProcess(service string) (err error) {
	pid, err := getServicePID(service)
	if err != nil {
		return err
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}

func getServicePID(service string) (int, error) {
	args := []string{
		"-n", // only return the newest process (though we expect only one to be running)
		"-x", // match the whole name
		service,
	}

	cmd := exec.Command("pgrep", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, errs.New("Error retrieving service pid: pgrep: %s %v", string(out), err)
	}

	trimmed := strings.TrimSuffix(string(out), "\n")

	pid, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}

	return pid, nil
}
