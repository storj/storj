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

	"storj.io/common/bloomfilter"
	"storj.io/storj/storagenode/iopriority"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
	"storj.io/storj/storagenode/storagenodedb"
)

// GCLazyFileWalker is an execwrapper.Command for the gc-filewalker.
type GCLazyFileWalker struct {
	*RunOptions
}

var _ execwrapper.Command = (*GCLazyFileWalker)(nil)

// NewGCLazyFilewalker creates a new GCLazyFileWalker instance.
func NewGCLazyFilewalker(ctx context.Context, logger *zap.Logger, config lazyfilewalker.Config) *GCLazyFileWalker {
	return NewGCLazyFilewalkerWithConfig(ctx, logger, &FilewalkerCfg{config})
}

// NewGCLazyFilewalkerWithConfig creates a new GCLazyFileWalker instance with the given config.
func NewGCLazyFilewalkerWithConfig(ctx context.Context, logger *zap.Logger, config *FilewalkerCfg) *GCLazyFileWalker {
	return &GCLazyFileWalker{
		RunOptions: DefaultRunOpts(ctx, logger, config),
	}
}

// Run runs the GCLazyFileWalker.
func (g *GCLazyFileWalker) Run() (err error) {
	if g.Config.LowerIOPriority {
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

	log := g.Logger

	// Decode the data struct received from the main process
	var req lazyfilewalker.GCFilewalkerRequest
	if err = json.NewDecoder(g.stdin).Decode(&req); err != nil {
		return errs.New("Error decoding data from stdin: %v", err)
	}

	// Validate the request data
	switch {
	case req.SatelliteID.IsZero():
		return errs.New("SatelliteID is required")
	case req.CreatedBefore.IsZero():
		return errs.New("CreatedBefore is required")
	}

	// We still need the DB in this case because we still have to deal with v0 pieces.
	// Once we drop support for v0 pieces, we can remove this.
	db, err := storagenodedb.OpenExisting(g.Ctx, log.Named("db"), g.Config.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	log.Info("Database started")
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// Decode the bloom filter
	filter, err := bloomfilter.NewFromBytes(req.BloomFilter)
	if err != nil {
		return err
	}

	log.Info("gc-filewalker started", zap.Time("created_before", req.CreatedBefore), zap.Int("bloom_filter_size", len(req.BloomFilter)))

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo())
	pieceIDs, piecesCount, piecesSkippedCount, err := filewalker.WalkSatellitePiecesToTrash(g.Ctx, req.SatelliteID, req.CreatedBefore, filter)
	if err != nil {
		return err
	}

	resp := lazyfilewalker.GCFilewalkerResponse{
		PieceIDs:           pieceIDs,
		PiecesCount:        piecesCount,
		PiecesSkippedCount: piecesSkippedCount,
	}

	log.Info("gc-filewalker completed", zap.Int64("pieces_count", piecesCount), zap.Int64("pieces_skipped_count", piecesSkippedCount))

	// encode the response struct and write it to stdout
	return json.NewEncoder(g.stdout).Encode(resp)
}

// Start starts the GCLazyFileWalker, assuming it behaves like the Start method on exec.Cmd.
// This is a no-op and only exists to satisfy the execwrapper.Command interface.
// Wait must be called to actually run the command.
func (g *GCLazyFileWalker) Start() error {
	return nil
}

// Wait waits for the GCLazyFileWalker to finish, assuming it behaves like the Wait method on exec.Cmd.
func (g *GCLazyFileWalker) Wait() error {
	return g.Run()
}

// SetIn sets the stdin of the GCLazyFileWalker.
func (g *GCLazyFileWalker) SetIn(reader io.Reader) {
	g.RunOptions.SetIn(reader)
}

// SetOut sets the stdout of the GCLazyFileWalker.
func (g *GCLazyFileWalker) SetOut(writer io.Writer) {
	g.RunOptions.SetOut(writer)
}

// SetErr sets the stderr of the GCLazyFileWalker.
func (g *GCLazyFileWalker) SetErr(writer io.Writer) {
	g.RunOptions.SetErr(writer)
}
