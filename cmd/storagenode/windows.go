// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// + build

package main

import (
	"time"

	"storj.io/storj/pkg/process"
	"golang.org/x/sys/windows/svc"
)

func init(){
	run := svc.Run
	err := run("storagenode", &myservice{})
	if err != nil {
		// elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
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
				// golang.org/x/sys/windows/svc.TestExample is verifying this output.
				// testOutput := strings.Join(ar/gs, "-")
				// testOutput += fmt.Sprintf("-%d", c.Context)
				// elog.Info(1, testOutput)
				break loop
			default:
				// fmt.Println(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}