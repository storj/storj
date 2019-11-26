// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

// +build !windows,!linux,!darwin,!netbsd,!freebsd,!openbsd

package processgroup

import (
	"os"
	"os/exec"
)

// Setup sets up exec.Cmd such that it can be properly terminated
func Setup(c *exec.Cmd) {}

// Kill tries to forcefully kill the process
func Kill(cmd *exec.Cmd) {
	proc := cmd.Process
	if proc == nil {
		return
	}
	_ = proc.Signal(os.Interrupt)
	_ = proc.Signal(os.Kill)
}
