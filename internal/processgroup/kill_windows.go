// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

// +build windows

package processgroup

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func Setup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func Kill(cmd *exec.Cmd) {
	proc := cmd.Process
	if proc == nil {
		return
	}

	exec.Command("taskkill", "/f", "/pid", strconv.Itoa(proc.Pid)).Run()

	// just in case
	forcekill(proc.Pid)
	proc.Signal(os.Interrupt)
	proc.Signal(os.Kill)
}

func forcekill(pid int) {
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, true, uint32(pid))
	if err != nil {
		return
	}

	syscall.TerminateProcess(handle, 0)
	syscall.CloseHandle(handle)
}
