// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

// +build !windows,!linux,!darwin,!netbsd,!freebsd,!openbsd

package processgroup

import (
	"os"
	"os/exec"
)

func Setup(c *exec.Cmd) {}

func Kill(cmd *exec.Cmd) {
	proc := cmd.Process
	if proc == nil {
		return
	}
	proc.Signal(os.Interrupt)
	proc.Signal(os.Kill)
}
