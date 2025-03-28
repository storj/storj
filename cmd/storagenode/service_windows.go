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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"

	"storj.io/common/process"
)

// ideally these would use a custom error map, but it seems annoying to integrate with Go.
const (
	eventSuccess              = 0
	errorBadArguments         = 0x000000A1
	errorServiceNotActive     = 0x00000426
	errorServiceSpecificError = 0x0000042A
)

func startAsService(cmd *cobra.Command) bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		zap.L().Fatal("Failed to determine if session is a service.", zap.Error(err))
	}
	if !isService {
		return false
	}

	var log EventLog

	if elog, err := eventlog.Open("Storj V3 Storage Node"); err == nil {
		log = elog
	} else {
		log = &ZapEventLog{log: zap.L()}
	}

	defer func() { _ = log.Close() }()

	// Check if the 'run' command is invoked
	if len(os.Args) < 2 || os.Args[1] != "run" {
		_ = log.Error(errorBadArguments, "run argument not specified for service")
		return false
	}

	// Initialize the Windows Service handler
	err = svc.Run("storagenode", &service{
		log:     log,
		rootCmd: cmd,
	})
	if err != nil {
		_ = log.Error(errorServiceNotActive, fmt.Sprintf("Service failed: %+v", err))
		zap.L().Fatal("Service failed.", zap.Error(err))
	}

	return true
}

type service struct {
	log     EventLog
	rootCmd *cobra.Command
}

func (m *service) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	var runCmd *cobra.Command
	for _, c := range m.rootCmd.Commands() {
		if c.Use == "run" {
			runCmd = c
		}
	}

	if runCmd == nil {
		_ = m.log.Error(errorBadArguments, "runCmd not found on root")
		panic("Assertion is failed: 'run' sub-command is not found.")
	}

	_ = m.log.Info(eventSuccess, "starting service")
	changes <- svc.Status{State: svc.StartPending}

	var group errgroup.Group
	group.Go(func() error {
		defer func() {
			if err := recover(); err != nil {
				_ = m.log.Error(errorServiceSpecificError, fmt.Sprintf("Panic: %+v", err))
				zap.L().Error("PANIC", zap.Any("error", err))
				panic(err) // re-panic
			}
		}()

		process.Exec(m.rootCmd)
		return nil
	})
	defer func() { _ = group.Wait() }()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

cmdloop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			_ = m.log.Info(eventSuccess, "Interrogate request received")
			zap.L().Info("Interrogate request received.")
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			if c.Cmd == svc.Stop {
				_ = m.log.Info(eventSuccess, "Stop request received.")
			} else {
				_ = m.log.Info(eventSuccess, "Shutdown request received.")
			}
			zap.L().Info("Stop/Shutdown request received.")

			// Cancel the command's root context to cleanup resources
			_, cancel := process.Ctx(runCmd)
			cancel()

			changes <- svc.Status{State: svc.StopPending, Accepts: cmdsAccepted}

			break cmdloop
		default:
			_ = m.log.Info(eventSuccess, fmt.Sprintf("Unexpected control request: %v", c.EventType))
			zap.L().Info("Unexpected control request.", zap.Uint32("Event Type", c.EventType))
		}
	}

	return false, 0
}

// EventLog implements interface for eventlog.Log.
type EventLog interface {
	Close() error
	Info(eid uint32, msg string) error
	Warning(eid uint32, msg string) error
	Error(eid uint32, msg string) error
}

// ZapEventLog implements EventLog interface, so we can use some sort of destination
// when we fail to open eventlog.
type ZapEventLog struct {
	log *zap.Logger
}

// Close closes event log.
func (log *ZapEventLog) Close() error { return nil }

// Info writes an information event msg with event id eid to the end of event log l.
func (log *ZapEventLog) Info(eid uint32, msg string) error {
	log.log.Info(msg, zap.Uint32("eid", eid))
	return nil
}

// Warning writes an warning event msg with event id eid to the end of event log l.
func (log *ZapEventLog) Warning(eid uint32, msg string) error {
	log.log.Warn(msg, zap.Uint32("eid", eid))
	return nil
}

// Error writes an error event msg with event id eid to the end of event log l.
func (log *ZapEventLog) Error(eid uint32, msg string) error {
	log.log.Error(msg, zap.Uint32("eid", eid))
	return nil
}
