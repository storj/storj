// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// + build

package main

import (
	"os"
	"syscall"
	"time"

	"storj.io/storj/pkg/process"
	"golang.org/x/sys/windows/svc"
)

func init() {
	err := svc.Run("storagenode", &myservice{})
	if err != nil {
		panic("service failed "+ err.Error())
	}
}

type myservice struct{}

func (m *myservice) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	go func() {
		process.Exec(rootCmd)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// TODO: find a way to cancel the context of the process
				break loop
			default:
				// elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}