// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/private/process"
	"storj.io/storj/storagenode/iopriority"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

type filewalkerCfg struct {
	lazyfilewalker.Config
}

// DatabaseConfig returns the storagenodedb.Config that should be used with this LazyFilewalkerConfig.
func (config *filewalkerCfg) DatabaseConfig() storagenodedb.Config {
	return storagenodedb.Config{
		Storage:   config.Storage,
		Info:      config.Info,
		Info2:     config.Info2,
		Pieces:    config.Pieces,
		Filestore: config.Filestore,
		Driver:    config.Driver,
	}
}

func newUsedSpaceFilewalkerCmd() *cobra.Command {
	var cfg filewalkerCfg

	cmd := &cobra.Command{
		Use:   lazyfilewalker.UsedSpaceFilewalkerCmdName,
		Short: "An internal subcommand used to run used-space calculation filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUsedSpaceFilewalker(cmd, &cfg)
		},
		Hidden: true,
		Args:   cobra.ExactArgs(0),
	}

	process.Bind(cmd, &cfg)

	return cmd
}

func newGCFilewalkerCmd() *cobra.Command {
	var cfg filewalkerCfg

	cmd := &cobra.Command{
		Use:   lazyfilewalker.GCFilewalkerCmdName,
		Short: "An internal subcommand used to run garbage collection filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdGCFilewalker(cmd, &cfg)
		},
		Hidden: true,
		Args:   cobra.ExactArgs(0),
	}

	process.Bind(cmd, &cfg)

	return cmd
}

func cmdUsedSpaceFilewalker(cmd *cobra.Command, cfg *filewalkerCfg) (err error) {
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

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	// We still need the DB in this case because we still have to deal with v0 pieces.
	// Once we drop support for v0 pieces, we can remove this.
	db, err := storagenodedb.OpenExisting(ctx, log.Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	log.Info("Database started")
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// Decode the data struct received from the main process
	var req lazyfilewalker.UsedSpaceRequest
	if err = json.NewDecoder(io.Reader(os.Stdin)).Decode(&req); err != nil {
		return errs.New("Error decoding data from stdin: %v", err)
	}

	if req.SatelliteID.IsZero() {
		return errs.New("SatelliteID is required")
	}

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo())

	total, contentSize, err := filewalker.WalkAndComputeSpaceUsedBySatellite(ctx, req.SatelliteID)
	if err != nil {
		return err
	}
	resp := lazyfilewalker.UsedSpaceResponse{PiecesTotal: total, PiecesContentSize: contentSize}

	// encode the response struct and write it to stdout
	return json.NewEncoder(io.Writer(os.Stdout)).Encode(resp)
}

func cmdGCFilewalker(cmd *cobra.Command, cfg *filewalkerCfg) (err error) {
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

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	// We still need the DB in this case because we still have to deal with v0 pieces.
	// Once we drop support for v0 pieces, we can remove this.
	db, err := storagenodedb.OpenExisting(ctx, log.Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	log.Info("Database started")
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// Decode the data struct received from the main process
	var req lazyfilewalker.GCFilewalkerRequest
	if err = json.NewDecoder(io.Reader(os.Stdin)).Decode(&req); err != nil {
		return errs.New("Error decoding data from stdin: %v", err)
	}

	// Validate the request data
	switch {
	case req.SatelliteID.IsZero():
		return errs.New("SatelliteID is required")
	case req.CreatedBefore.IsZero():
		return errs.New("CreatedBefore is required")
	}

	// Decode the bloom filter
	filter, err := bloomfilter.NewFromBytes(req.BloomFilter)
	if err != nil {
		return err
	}

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo())
	pieceIDs, piecesCount, piecesSkippedCount, err := filewalker.WalkSatellitePiecesToTrash(ctx, req.SatelliteID, req.CreatedBefore, filter)
	if err != nil {
		return err
	}

	resp := lazyfilewalker.GCFilewalkerResponse{
		PieceIDs:           pieceIDs,
		PiecesCount:        piecesCount,
		PiecesSkippedCount: piecesSkippedCount,
	}

	// encode the response struct and write it to stdout
	return json.NewEncoder(io.Writer(os.Stdout)).Encode(resp)
}
