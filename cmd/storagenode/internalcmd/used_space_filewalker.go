// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package internalcmd

import (
	"context"
	"encoding/json"
	"io"
	"runtime"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/iopriority"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
	"storj.io/storj/storagenode/storagenodedb"
)

// UsedSpaceLazyFileWalker is an execwrapper.Command for the used-space-filewalker.
type UsedSpaceLazyFileWalker struct {
	*RunOptions
}

var _ execwrapper.Command = (*UsedSpaceLazyFileWalker)(nil)

// NewUsedSpaceLazyFilewalker creates a new UsedSpaceLazyFileWalker instance.
func NewUsedSpaceLazyFilewalker(ctx context.Context, logger *zap.Logger, config lazyfilewalker.Config) *UsedSpaceLazyFileWalker {
	return NewUsedSpaceLazyFilewalkerWithConfig(ctx, logger, &FilewalkerCfg{config})
}

// NewUsedSpaceLazyFilewalkerWithConfig creates a new UsedSpaceLazyFileWalker instance with the given config.
func NewUsedSpaceLazyFilewalkerWithConfig(ctx context.Context, logger *zap.Logger, config *FilewalkerCfg) *UsedSpaceLazyFileWalker {
	return &UsedSpaceLazyFileWalker{
		RunOptions: DefaultRunOpts(ctx, logger, config),
	}
}

// Run runs the UsedSpaceLazyFileWalker.
func (u *UsedSpaceLazyFileWalker) Run() (err error) {
	if u.Config.LowerIOPriority {
		if runtime.GOOS == "linux" {
			// Pin the current goroutine to the current OS thread, so we can set the IO priority
			// for the current thread.
			// This is necessary because Go does use CLONE_IO when creating new threads,
			// so they do not share a single IO context.
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}

		err = iopriority.SetLowIOPriority()
		if err != nil {
			return err
		}
	}
	log := u.Logger

	// Decode the data struct received from the main process
	var req lazyfilewalker.UsedSpaceRequest
	if err = json.NewDecoder(u.stdin).Decode(&req); err != nil {
		return errs.New("Error decoding data from stdin: %v", err)
	}

	if req.SatelliteID.IsZero() {
		return errs.New("SatelliteID is required")
	}

	// We still need the DB in this case because we still have to deal with v0 pieces.
	// Once we drop support for v0 pieces, we can remove this.
	db, err := storagenodedb.OpenExisting(u.Ctx, log.Named("db"), u.Config.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	log.Info("Database started")
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	log.Info("used-space-filewalker started")

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo())
	total, contentSize, err := filewalker.WalkAndComputeSpaceUsedBySatellite(u.Ctx, req.SatelliteID)
	if err != nil {
		return err
	}
	resp := lazyfilewalker.UsedSpaceResponse{PiecesTotal: total, PiecesContentSize: contentSize}

	log.Info("used-space-filewalker completed", zap.Int64("pieces_total", total), zap.Int64("content_size", contentSize))

	// encode the response struct and write it to stdout
	return json.NewEncoder(u.stdout).Encode(resp)
}

// Start starts the GCLazyFileWalker, assuming it behaves like the Start method on exec.Cmd.
// This is a no-op and only exists to satisfy the execwrapper.Command interface.
// Wait must be called to actually run the command.
func (u *UsedSpaceLazyFileWalker) Start() error {
	return nil
}

// Wait waits for the GCLazyFileWalker to finish, assuming it behaves like the Wait method on exec.Cmd.
func (u *UsedSpaceLazyFileWalker) Wait() error {
	return u.Run()
}

// SetIn sets the stdin of the UsedSpaceLazyFileWalker.
func (u *UsedSpaceLazyFileWalker) SetIn(reader io.Reader) {
	u.RunOptions.SetIn(reader)
}

// SetOut sets the stdout of the UsedSpaceLazyFileWalker.
func (u *UsedSpaceLazyFileWalker) SetOut(writer io.Writer) {
	u.RunOptions.SetOut(writer)
}

// SetErr sets the stderr of the UsedSpaceLazyFileWalker.
func (u *UsedSpaceLazyFileWalker) SetErr(writer io.Writer) {
	u.RunOptions.SetErr(writer)
}
