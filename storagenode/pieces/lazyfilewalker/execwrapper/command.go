// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package execwrapper

import "io"

// A Command is an external command being prepared or run.
// It tries to mimic the exec.Cmd.
type Command interface {
	// Start starts the command but does not wait for it to complete.
	// It returns an error if the command fails to start.
	// The command must be started before calling Wait.
	Start() error
	// Wait waits for the command to exit and waits for any copying to stdin or copying
	// from stdout or stderr to complete.
	// Start must be called before calling Wait.
	Wait() error
	// Run starts the specified command and waits for it to complete.
	Run() error
	// SetIn sets the stdin of the command.
	SetIn(io.Reader)
	// SetOut sets the stdout of the command.
	SetOut(io.Writer)
	// SetErr sets the stderr of the command.
	SetErr(io.Writer)
	// SetArgs sets arguments for the command including the command or executable path as the first argument.
	SetArgs([]string)
}
