// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

// +build windows

package processgroup

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Setup sets up exec.Cmd such that it can be properly terminated
func Setup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// Kill tries to forcefully kill the process
func Kill(cmd *exec.Cmd) {
	proc := cmd.Process
	if proc == nil {
		return
	}

	_ = exec.Command("taskkill", "/f", "/pid", strconv.Itoa(proc.Pid)).Run()

	// just in case
	forcekill(proc.Pid)
	_ = proc.Signal(os.Interrupt)
	_ = proc.Signal(os.Kill)
}

func forcekill(pid int) {
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, true, uint32(pid))
	if err != nil {
		return
	}

	_ = syscall.TerminateProcess(handle, 0)
	_ = syscall.CloseHandle(handle)
}
