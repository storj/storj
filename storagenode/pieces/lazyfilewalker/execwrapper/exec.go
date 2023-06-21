// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package execwrapper

import (
	"context"
	"io"

	"golang.org/x/sys/execabs"
)

var _ Command = (*Cmd)(nil)

// Cmd is an external command being prepared or run.
type Cmd struct {
	cmd *execabs.Cmd
}

// SetIn sets the stdin of the command.
func (c *Cmd) SetIn(reader io.Reader) {
	c.cmd.Stdin = reader
}

// SetOut sets the stdout of the command.
func (c *Cmd) SetOut(writer io.Writer) {
	c.cmd.Stdout = writer
}

// SetErr sets the stderr of the command.
func (c *Cmd) SetErr(writer io.Writer) {
	c.cmd.Stderr = writer
}

// SetArgs sets arguments for the command including the command or executable path as the first argument.
func (c *Cmd) SetArgs(args []string) {
	c.cmd.Args = args
}

// CommandContext returns the Cmd struct to execute the named program with the given arguments.
func CommandContext(ctx context.Context, executable string, args ...string) *Cmd {
	return &Cmd{
		cmd: execabs.CommandContext(ctx, executable, args...),
	}
}

// Run starts the specified command and waits for it to complete.
func (c *Cmd) Run() error {
	return c.cmd.Run()
}

// Start starts the command but does not wait for it to complete.
// It returns an error if the command fails to start.
// The command must be started before calling Wait.
func (c *Cmd) Start() error {
	return c.cmd.Start()
}

// Wait waits for the command to exit and waits for any copying to stdin or copying
// from stdout or stderr to complete.
// Start must be called before calling Wait.
func (c *Cmd) Wait() error {
	return c.cmd.Wait()
}
