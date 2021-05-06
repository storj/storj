// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/storj/cmd/uplinkng/ulloc"
)

// Setup returns some State that can be run multiple times with different command
// line arguments.
func Setup(cmds func(clingy.Commands, clingy.Flags), opts ...ExecuteOption) State {
	return State{
		cmds: cmds,
		opts: opts,
	}
}

// State represents some state and environment for a command to execute in.
type State struct {
	cmds func(clingy.Commands, clingy.Flags)
	opts []ExecuteOption
}

// With appends the provided options and returns a new State.
func (st State) With(opts ...ExecuteOption) State {
	st.opts = append([]ExecuteOption(nil), st.opts...)
	st.opts = append(st.opts, opts...)
	return st
}

// Succeed is the same as Run followed by result.RequireSuccess.
func (st State) Succeed(t *testing.T, args ...string) Result {
	result := st.Run(t, args...)
	result.RequireSuccess(t)
	return result
}

// Fail is the same as Run followed by result.RequireFailure.
func (st State) Fail(t *testing.T, args ...string) Result {
	result := st.Run(t, args...)
	result.RequireFailure(t)
	return result
}

// Run executes the command specified by the args and returns a Result.
func (st State) Run(t *testing.T, args ...string) Result {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var stdin bytes.Buffer
	var ops []Operation
	var ran bool

	ok, err := clingy.Environment{
		Name: "uplink-test",
		Args: args,

		Stdin:  &stdin,
		Stdout: &stdout,
		Stderr: &stderr,

		Wrap: func(ctx clingy.Context, cmd clingy.Cmd) error {
			tfs := newTestFilesystem()
			for _, opt := range st.opts {
				if err := opt.fn(ctx, tfs); err != nil {
					return errs.Wrap(err)
				}
			}
			tfs.ops = nil

			if len(tfs.stdin) > 0 {
				_, _ = stdin.WriteString(tfs.stdin)
			}

			if setter, ok := cmd.(interface {
				SetTestFilesystem(ulfs.Filesystem)
			}); ok {
				setter.SetTestFilesystem(tfs)
			}

			ran = true
			err := cmd.Execute(ctx)
			ops = tfs.ops
			return err
		},
	}.Run(context.Background(), st.cmds)

	if ok && err == nil {
		require.True(t, ran, "no command was executed: %q", args)
	}
	return Result{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		Ok:         ok,
		Err:        err,
		Operations: ops,
	}
}

// ExecuteOption allows one to control the environment that a command executes in.
type ExecuteOption struct {
	fn func(ctx clingy.Context, tfs *testFilesystem) error
}

// WithStdin sets the command to execute with the provided string as standard input.
func WithStdin(stdin string) ExecuteOption {
	return ExecuteOption{func(_ clingy.Context, tfs *testFilesystem) error {
		tfs.stdin = stdin
		return nil
	}}
}

// WithFile sets the command to execute with a file created at the given location.
func WithFile(location string) ExecuteOption {
	return ExecuteOption{func(ctx clingy.Context, tfs *testFilesystem) error {
		loc, err := ulloc.Parse(location)
		if err != nil {
			return err
		}
		if bucket, _, ok := loc.RemoteParts(); ok {
			tfs.ensureBucket(bucket)
		}
		wh, err := tfs.Create(ctx, loc)
		if err != nil {
			return err
		}
		return wh.Commit()
	}}
}

// WithPendingFile sets the command to execute with a pending upload happening to
// the provided location.
func WithPendingFile(location string) ExecuteOption {
	return ExecuteOption{func(ctx clingy.Context, tfs *testFilesystem) error {
		loc, err := ulloc.Parse(location)
		if err != nil {
			return err
		}
		if bucket, _, ok := loc.RemoteParts(); ok {
			tfs.ensureBucket(bucket)
		}
		_, err = tfs.Create(ctx, loc)
		return err
	}}
}
