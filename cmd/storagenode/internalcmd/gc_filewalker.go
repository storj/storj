// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package internalcmd

import (
	"encoding/json"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/iopriority"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

const piecesBatchSize = 1000

// NewGCFilewalkerCmd creates a new cobra command for running garbage collection filewalker.
func NewGCFilewalkerCmd() *LazyFilewalkerCmd {
	var cfg FilewalkerCfg
	var runOpts RunOptions

	cmd := &cobra.Command{
		Use:   lazyfilewalker.GCFilewalkerCmdName,
		Short: "An internal subcommand used to run garbage collection filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts.normalize(cmd)
			runOpts.config = &cfg

			return gcCmdRun(&runOpts)
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		Hidden: true,
		Args:   cobra.ExactArgs(0),
	}

	process.Bind(cmd, &cfg)

	return NewLazyFilewalkerCmd(cmd, &runOpts)
}

// Run runs the GCLazyFileWalker.
func gcCmdRun(g *RunOptions) (err error) {
	if g.config.LowerIOPriority {
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
	db, err := storagenodedb.OpenExisting(g.Ctx, log.Named("db"), g.config.DatabaseConfig())
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

	log.Info("gc-filewalker started", zap.Time("createdBefore", req.CreatedBefore), zap.Int("bloomFilterSize", len(req.BloomFilter)))

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo(), db.GCFilewalkerProgress(), nil)

	encoder := json.NewEncoder(g.stdout)
	numTrashed := 0
	pieceIDs := make([]storj.PieceID, 0, piecesBatchSize)

	flushPiecesToTrash := func() error {
		if len(pieceIDs) == 0 {
			return nil
		}

		resp := lazyfilewalker.GCFilewalkerResponse{
			PieceIDs: pieceIDs,
		}
		err := encoder.Encode(resp)
		if err != nil {
			log.Debug("failed to notify main process", zap.Error(err))
			return err
		}
		numTrashed += len(pieceIDs)
		pieceIDs = pieceIDs[:0]
		return nil
	}

	trashPiecesCount := 0
	piecesCount, piecesSkippedCount, err := filewalker.WalkSatellitePiecesToTrash(g.Ctx, req.SatelliteID, req.CreatedBefore, filter, func(pieceID storj.PieceID) error {
		log.Debug("found a trash piece", zap.Stringer("pieceID", pieceID))
		// we found a piece that needs to be trashed, so we notify the main process.
		// do it in batches to avoid sending too many messages.
		pieceIDs = append(pieceIDs, pieceID)
		trashPiecesCount++

		if len(pieceIDs) >= piecesBatchSize {
			return flushPiecesToTrash()
		}
		return nil
	})
	if err != nil {
		log.Debug("gc-filewalker failed", zap.Error(err))
		return err
	}

	if err := flushPiecesToTrash(); err != nil {
		log.Debug("failed to notify main process about pieces to trash", zap.Error(err))
		return err
	}

	resp := lazyfilewalker.GCFilewalkerResponse{
		PiecesCount:        piecesCount,
		PiecesSkippedCount: piecesSkippedCount,
		Completed:          true,
	}

	log.Info("gc-filewalker completed", zap.Int64("piecesCount", piecesCount), zap.Int("Total Pieces To Trash", trashPiecesCount), zap.Int("Trashed Pieces", numTrashed), zap.Int64("Pieces Skipped", piecesSkippedCount))

	// encode the response struct and write it to stdout
	err = json.NewEncoder(g.stdout).Encode(resp)
	if err != nil {
		log.Debug("failed to write to stdout", zap.Error(err))
		return errs.New("Error writing response to stdout: %v", err)
	}
	return nil
}
