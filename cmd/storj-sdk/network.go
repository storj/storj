// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/processgroup"
)

func networkExec(flags *Flags, args []string, command string) error {
	processes, err := NewProcesses(flags.Directory, flags.SatelliteCount, flags.StorageNodeCount)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	err = processes.Exec(ctx, command)
	closeErr := processes.Close()

	return errs.Combine(err, closeErr)
}

func networkTest(flags *Flags, command string, args []string) error {
	processes, err := NewProcesses(flags.Directory, flags.SatelliteCount, flags.StorageNodeCount)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())

	var group errgroup.Group
	processes.Start(ctx, &group, command)

	time.Sleep(2 * time.Second)

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = processes.Env()
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	processgroup.Setup(cmd)

	errRun := cmd.Run()

	cancel()
	return errs.Combine(errRun, processes.Close(), group.Wait())
}

func networkDestroy(flags *Flags, args []string) error {
	if fpath.IsRoot(flags.Directory) {
		return errors.New("safety check: disallowed to remove root directory " + flags.Directory)
	}

	fmt.Println("exec: rm -rf", flags.Directory)
	return os.RemoveAll(flags.Directory)
}
