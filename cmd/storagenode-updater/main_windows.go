// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Implements support for running the storagenode-updater as a Windows Service.
//
// The Windows Service can be created with sc.exe, e.g.
//
// sc.exe create storagenode-updater binpath= "C:\Users\MyUser\storagenode-updater.exe run ..."

//go:build windows

package main

import (
	"os"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"

	"storj.io/common/process"
)

func isRunCmd() bool {
	return len(os.Args) > 1 && os.Args[1] == "run"
}

func main() {
	logger, _, _ := process.NewLogger("storagenode-updater")
	zap.ReplaceGlobals(logger)

	isService, err := svc.IsWindowsService()
	if err != nil {
		zap.L().Fatal("Failed to determine if session is interactive.", zap.Error(err))
	}

	if !isService || !isRunCmd() {
		process.Exec(rootCmd)
		return
	}

	err = svc.Run("storagenode-updater", &service{})
	if err != nil {
		zap.L().Fatal("Service failed.", zap.Error(err))
	}
}

type service struct{}

func (m *service) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	var group errgroup.Group
	group.Go(func() error {
		process.Exec(rootCmd)
		return nil
	})

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			zap.L().Info("Interrogate request received.")
			changes <- c.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			zap.L().Info("Stop/Shutdown request received.")
			changes <- svc.Status{State: svc.StopPending}
			// Cancel the command's root context to cleanup resources
			_, cancel := process.Ctx(runCmd)
			cancel()
			_ = group.Wait() // process.Exec does not return an error
			// After returning the Windows Service is stopped and the process terminates
			return false, 0
		default:
			zap.L().Info("Unexpected control request.", zap.Uint32("Event Type", c.EventType))
		}
	}

	return false, 0
}
