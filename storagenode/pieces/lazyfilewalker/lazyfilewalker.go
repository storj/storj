// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sys/execabs"

	"storj.io/common/storj"
)

var (
	errLazyFilewalker = errs.Class("lazyfilewalker")

	mon = monkit.Package()
)

// Supervisor performs filewalker operations in a subprocess with lower I/O priority.
type Supervisor struct {
	log *zap.Logger

	executable    string
	gcArgs        []string
	usedSpaceArgs []string
}

// NewSupervisor creates a new lazy filewalker Supervisor.
func NewSupervisor(log *zap.Logger, executable string, args []string) *Supervisor {
	return &Supervisor{
		log:           log,
		gcArgs:        append([]string{"gc-filewalker"}, args...),
		usedSpaceArgs: append([]string{"used-space-filewalker"}, args...),
		executable:    executable,
	}
}

// UsedSpaceRequest is the request struct for the used-space-filewalker process.
type UsedSpaceRequest struct {
	SatelliteID storj.NodeID `json:"satelliteID"`
}

// UsedSpaceResponse is the response struct for the used-space-filewalker process.
type UsedSpaceResponse struct {
	PiecesTotal       int64 `json:"piecesTotal"`
	PiecesContentSize int64 `json:"piecesContentSize"`
}

// WalkAndComputeSpaceUsedBySatellite returns the total used space by satellite.
func (fw *Supervisor) WalkAndComputeSpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (piecesTotal int64, piecesContentSize int64, err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req := UsedSpaceRequest{
		SatelliteID: satelliteID,
	}

	fw.log.Info("starting subprocess", zap.String("satelliteID", satelliteID.String()))
	cmd := execabs.CommandContext(ctx, fw.executable, fw.usedSpaceArgs...)

	var buf, outbuf bytes.Buffer
	writer := &zapWrapper{fw.log.Named("subprocess")}

	// encode the struct and write it to the buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(req); err != nil {
		return 0, 0, errLazyFilewalker.Wrap(err)
	}
	cmd.Stdin = &buf
	cmd.Stdout = &outbuf
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		fw.log.Error("failed to start subprocess", zap.Error(err))
		return 0, 0, err
	}

	fw.log.Info("subprocess started", zap.String("satelliteID", satelliteID.String()))

	if err := cmd.Wait(); err != nil {
		var exitErr *execabs.ExitError
		if errors.As(err, &exitErr) {
			fw.log.Info("subprocess exited with status", zap.Int("status", exitErr.ExitCode()), zap.Error(exitErr), zap.String("satelliteID", satelliteID.String()))
		} else {
			fw.log.Error("subprocess exited with error", zap.Error(err), zap.String("satelliteID", satelliteID.String()))
		}
		return 0, 0, errLazyFilewalker.Wrap(err)
	}

	fw.log.Info("subprocess finished successfully", zap.String("satelliteID", satelliteID.String()), zap.Int64("piecesTotal", piecesTotal), zap.Int64("piecesContentSize", piecesContentSize))

	// Decode and receive the response data struct from the subprocess
	var resp UsedSpaceResponse
	decoder := json.NewDecoder(&outbuf)
	if err := decoder.Decode(&resp); err != nil {
		fw.log.Error("failed to decode response from subprocess", zap.String("satelliteID", satelliteID.String()), zap.Error(err))
		return 0, 0, errLazyFilewalker.Wrap(err)
	}

	return resp.PiecesTotal, resp.PiecesContentSize, nil
}
