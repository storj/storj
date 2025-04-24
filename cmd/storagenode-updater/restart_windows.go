// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows && service

package main

import (
	"context"
	"math"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"storj.io/common/process"
	"storj.io/common/sync2"
)

var unrecoverableErr = errs.Class("unable to recover binary from backup")

func cmdRestart(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	currentVersion, err := binaryVersion(runCfg.BinaryLocation)
	if err != nil {
		return errs.Wrap(err)
	}

	newVersionPath := args[0]

	var backupPath string
	if runCfg.ServiceName == updaterServiceName {
		// NB: don't include old version number for updater binary backup
		backupPath = prependExtension(runCfg.BinaryLocation, "old")
	} else {
		backupPath = prependExtension(runCfg.BinaryLocation, "old."+currentVersion.String())
	}

	// check if new binary exists
	if _, err := os.Stat(newVersionPath); err != nil {
		return errs.Wrap(err)
	}

	_, err = swapBinariesAndRestart(ctx, "", runCfg.ServiceName, runCfg.BinaryLocation, newVersionPath, backupPath)
	return err
}

func swapBinariesAndRestart(ctx context.Context, restartMethod, service, binaryLocation, newVersionPath, backupPath string) (exit bool, err error) {
	srvc, err := openService(service)
	if err != nil {
		return false, errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
	}
	defer func() {
		err = errs.Combine(err, errs.Wrap(srvc.Close()))
	}()

	status, err := srvc.Query()
	if err != nil {
		return false, errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
	}

	// stop service if it's not stopped
	if status.State != svc.Stopped && status.State != svc.StopPending {
		if err = serviceControl(ctx, srvc, svc.Stop, svc.Stopped, 10*time.Second); err != nil {
			return false, errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
		}
		// if it is stopping wait for it to complete
	} else if status.State == svc.StopPending {
		if err = serviceWaitForState(ctx, srvc, svc.Stopped, 10*time.Second); err != nil {
			return false, errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
		}
	}

	err = func() error {
		if err := os.Rename(binaryLocation, backupPath); err != nil {
			return errs.Combine(err, srvc.Start())
		}

		if err := os.Rename(newVersionPath, binaryLocation); err != nil {
			if rerr := os.Rename(backupPath, binaryLocation); rerr != nil {
				// unrecoverable error
				return unrecoverableErr.Wrap(errs.Combine(err, rerr))
			}

			return errs.Combine(err, srvc.Start())
		}

		return nil
	}()
	if err != nil {
		return false, errs.Combine(errs.Wrap(err), os.Remove(newVersionPath))
	}

	// successfully substituted binaries
	err = retry(ctx, 2,
		func() error {
			return srvc.Start()
		},
	)
	// if fail to start the service, try again with backup
	if err != nil {
		if rerr := os.Rename(backupPath, binaryLocation); rerr != nil {
			// unrecoverable error
			return false, unrecoverableErr.Wrap(errs.Combine(err, rerr))
		}

		return false, errs.Combine(err, srvc.Start())
	}

	return false, nil
}

func openService(name string) (_ *mgr.Service, err error) {
	manager, err := mgr.Connect()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, errs.Wrap(manager.Disconnect()))
	}()

	service, err := manager.OpenService(name)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return service, nil
}

func serviceControl(ctx context.Context, service *mgr.Service, cmd svc.Cmd, state svc.State, delay time.Duration) error {
	status, err := service.Control(cmd)
	if err != nil {
		return err
	}

	timeout := time.Now().Add(delay)

	for status.State != state {
		if err := ctx.Err(); err != nil {
			return err
		}
		if timeout.Before(time.Now()) {
			return errs.New("timeout")
		}

		status, err = service.Query()
		if err != nil {
			return err
		}
	}

	return nil
}

func serviceWaitForState(ctx context.Context, service *mgr.Service, state svc.State, delay time.Duration) error {
	status, err := service.Query()
	if err != nil {
		return err
	}

	timeout := time.Now().Add(delay)

	for status.State != state {
		if err := ctx.Err(); err != nil {
			return err
		}
		if timeout.Before(time.Now()) {
			return errs.New("timeout")
		}

		status, err = service.Query()
		if err != nil {
			return err
		}
	}

	return nil
}

func retry(ctx context.Context, count int, cb func() error) error {
	var err error

	if err = cb(); err == nil {
		return nil
	}

	for i := 1; i < count; i++ {
		delay := time.Duration(math.Pow10(i))

		if !sync2.Sleep(ctx, delay*time.Second) {
			return ctx.Err()
		}
		if err = cb(); err == nil {
			return nil
		}
	}

	return err
}
