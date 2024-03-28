// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package internalcmd

import (
	"encoding/json"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/process"
	"storj.io/storj/storagenode/iopriority"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

// NewUsedSpaceFilewalkerCmd creates a new cobra command for running used-space calculation filewalker.
func NewUsedSpaceFilewalkerCmd() *LazyFilewalkerCmd {
	var cfg FilewalkerCfg
	var runOpts RunOptions

	cmd := &cobra.Command{
		Use:   lazyfilewalker.UsedSpaceFilewalkerCmdName,
		Short: "An internal subcommand used to run used-space calculation filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts.normalize(cmd)
			runOpts.config = &cfg

			return usedSpaceCmdRun(&runOpts)
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

// Run runs the UsedSpaceLazyFileWalker.
func usedSpaceCmdRun(opts *RunOptions) (err error) {
	if opts.config.LowerIOPriority {
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
	log := opts.Logger

	// Decode the data struct received from the main process
	var req lazyfilewalker.UsedSpaceRequest
	if err = json.NewDecoder(opts.stdin).Decode(&req); err != nil {
		return errs.New("Error decoding data from stdin: %v", err)
	}

	if req.SatelliteID.IsZero() {
		return errs.New("SatelliteID is required")
	}

	// We still need the DB in this case because we still have to deal with v0 pieces.
	// Once we drop support for v0 pieces, we can remove this.
	db, err := storagenodedb.OpenExisting(opts.Ctx, log.Named("db"), opts.config.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	log.Info("Database started")
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo(), nil, db.UsedSpacePerPrefix())
	total, contentSize, pieceCount, err := filewalker.WalkAndComputeSpaceUsedBySatellite(opts.Ctx, req.SatelliteID)
	if err != nil {
		return err
	}
	resp := lazyfilewalker.UsedSpaceResponse{PiecesTotal: total, PiecesContentSize: contentSize, PieceCount: pieceCount}

	// encode the response struct and write it to stdout
	return json.NewEncoder(opts.stdout).Encode(resp)
}
