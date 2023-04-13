// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"go.uber.org/zap"
	"golang.org/x/sys/execabs"
)

// process is a subprocess that can be used to perform filewalker operations.
type process struct {
	log        *zap.Logger
	executable string
	args       []string
}

func newProcess(log *zap.Logger, executable string, args []string) *process {
	return &process{
		log:        log,
		executable: executable,
		args:       args,
	}
}

// run runs the process and decodes the response into the value pointed by `resp`.
// It returns an error if the Process fails to start, or if the Process exits with a non-zero status.
// NOTE: the `resp` value must be a pointer to a struct.
func (p *process) run(ctx context.Context, req, resp interface{}) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.log.Info("starting subprocess")

	var buf, outbuf bytes.Buffer
	writer := &zapWrapper{p.log.Named("subprocess")}

	// encode the struct and write it to the buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(req); err != nil {
		return errLazyFilewalker.Wrap(err)
	}

	cmd := execabs.CommandContext(ctx, p.executable, p.args...)
	cmd.Stdin = &buf
	cmd.Stdout = &outbuf
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		p.log.Error("failed to start subprocess", zap.Error(err))
		return errLazyFilewalker.Wrap(err)
	}

	p.log.Info("subprocess started")

	if err := cmd.Wait(); err != nil {
		var exitErr *execabs.ExitError
		if errors.As(err, &exitErr) {
			p.log.Info("subprocess exited with status", zap.Int("status", exitErr.ExitCode()), zap.Error(exitErr))
		} else {
			p.log.Error("subprocess exited with error", zap.Error(err))
		}
		return errLazyFilewalker.Wrap(err)
	}

	p.log.Info("subprocess finished successfully")

	// Decode and receive the response data struct from the subprocess
	decoder := json.NewDecoder(&outbuf)
	if err := decoder.Decode(&resp); err != nil {
		p.log.Error("failed to decode response from subprocess", zap.Error(err))
		return errLazyFilewalker.Wrap(err)
	}

	return nil
}
