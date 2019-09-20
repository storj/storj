// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Implements support for running the storage node as a Windows Service.
//
// The Windows Service can be created with sc.exe, e.g.
//
// sc.exe create storagenode binpath= "C:\Users\MyUser\storagenode.exe run --config-dir C:\Users\MyUser\"
//
// The --config-dir argument can be omitted if the config.yaml is available at
// C:\Windows\System32\config\systemprofile\AppData\Roaming\Storj\Storagenode\config.yaml

// +build windows

package main

import (
	"os"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/windows/svc"

	"storj.io/storj/pkg/process"
)

func init() {
	// Check if session is interactive
	interactive, err := svc.IsAnInteractiveSession()
	if err != nil {
		zap.S().Fatalf("Failed to determine if session is interactive: %v", err)
	}

	if interactive {
		return
	}

	// Check if the 'run' command is invoked
	if len(os.Args) < 2 {
		return
	}

	if os.Args[1] != "run" {
		return
	}

	// Initialize the Windows Service handler
	err = svc.Run("storagenode", &service{})
	if err != nil {
		zap.S().Fatalf("Service failed: %v", err)
	}
}

type service struct{}

func (m *service) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	go func() {
		process.Exec(rootCmd)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				zap.S().Info("Interrogate request received.")
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				zap.S().Info("Stop/Shutdown request received.")
				changes <- svc.Status{State: svc.StopPending}
				// Cancel the command's root context to cleanup resources
				_, cancel := process.Ctx(runCmd)
				cancel()
				// Sleep some time to give chance for goroutines finish cleanup after cancelling the context
				time.Sleep(15 * time.Second)
				// After returning the Windows Service is stopped and the process terminates
				return
			default:
				zap.S().Infof("Unexpected control request: %d\n", c)
			}
		}
	}
}
