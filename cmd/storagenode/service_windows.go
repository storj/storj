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

package main

import (
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"

	"storj.io/private/process"
)

var rootCmd, runCmd *cobra.Command

func startAsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		zap.L().Fatal("Failed to determine if session is a service.", zap.Error(err))
	}
	if !isService {
		return false
	}

	// Check if the 'run' command is invoked
	if len(os.Args) < 2 {
		return false
	}

	if os.Args[1] != "run" {
		return false
	}

	var factory *Factory
	rootCmd, factory = newRootCmd(true)
	runCmd = newRunCmd(factory)

	// Initialize the Windows Service handler
	err = svc.Run("storagenode", &service{})
	if err != nil {
		zap.L().Fatal("Service failed.", zap.Error(err))
	}

	return true
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
	defer func() { _ = group.Wait() }()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

cmdloop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			zap.L().Info("Interrogate request received.")
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			zap.L().Info("Stop/Shutdown request received.")

			// Cancel the command's root context to cleanup resources
			_, cancel := process.Ctx(runCmd)
			cancel()

			changes <- svc.Status{State: svc.StopPending, Accepts: cmdsAccepted}

			break cmdloop
		default:
			zap.L().Info("Unexpected control request.", zap.Uint32("Event Type", c.EventType))
		}
	}

	return false, 0
}
