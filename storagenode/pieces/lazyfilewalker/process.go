// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"go.uber.org/zap"
	"golang.org/x/sys/execabs"

	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
)

// process is a subprocess that can be used to perform filewalker operations.
type process struct {
	log        *zap.Logger
	executable string
	args       []string

	stderr io.Writer

	cmd execwrapper.Command
}

// newProcess creates a new process.
// The cmd argument can be used to replace the subprocess with a runner for testing, it can be nil.
func newProcess(cmd execwrapper.Command, log *zap.Logger, executable string, args []string) *process {
	return &process{
		cmd:        cmd,
		log:        log,
		executable: executable,
		args:       args,
		stderr:     &zapWrapper{log.Named("subprocess")},
	}
}

// run runs the process.
// It returns an error if the Process fails to start, or if the Process exits with a non-zero status.
func (p *process) run(ctx context.Context, stdout io.Writer, req interface{}) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.log.Info("starting subprocess")

	var buf bytes.Buffer

	// encode the struct and write it to the buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(req); err != nil {
		return errLazyFilewalker.Wrap(err)
	}

	if p.cmd == nil {
		p.cmd = execwrapper.CommandContext(ctx, p.executable, p.args...)
	} else {
		args := append([]string{p.executable}, p.args...)
		p.cmd.SetArgs(args)
	}

	p.cmd.SetIn(&buf)
	p.cmd.SetOut(stdout)
	p.cmd.SetErr(p.stderr)

	if err := p.cmd.Start(); err != nil {
		p.log.Error("failed to start subprocess", zap.Error(err))
		return errLazyFilewalker.Wrap(err)
	}

	p.log.Info("subprocess started")

	if err := p.cmd.Wait(); err != nil {
		var exitErr *execabs.ExitError
		if errors.As(err, &exitErr) {
			p.log.Info("subprocess exited with status", zap.Int("status", exitErr.ExitCode()), zap.Error(exitErr))
		} else {
			p.log.Error("subprocess exited with error", zap.Error(err))
		}
		return errLazyFilewalker.Wrap(err)
	}

	p.log.Info("subprocess finished successfully")

	return nil
}
