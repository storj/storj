// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

// +build linux darwin netbsd freebsd openbsd

package processgroup

import (
	"os"
	"os/exec"
	"syscall"
)

// Setup sets up exec.Cmd such that it can be properly terminated
func Setup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// Kill tries to forcefully kill the process
func Kill(cmd *exec.Cmd) {
	proc := cmd.Process
	if proc == nil {
		return
	}

	pgid, err := syscall.Getpgid(proc.Pid)
	if err != nil {
		_ = syscall.Kill(-pgid, 15)
	}

	// just in case
	_ = proc.Signal(os.Interrupt)
	_ = proc.Signal(os.Kill)
}
