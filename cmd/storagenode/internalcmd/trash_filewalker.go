// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package internalcmd

import (
	"encoding/json"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/storagenode/iopriority"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

// NewTrashFilewalkerCmd creates a new cobra command for running a trash cleanup filewalker.
func NewTrashFilewalkerCmd() *LazyFilewalkerCmd {
	var cfg FilewalkerCfg
	var runOpts RunOptions

	cmd := &cobra.Command{
		Use:   lazyfilewalker.TrashCleanupFilewalkerCmdName,
		Short: "An internal subcommand used to run a trash cleanup filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts.normalize(cmd)
			runOpts.config = &cfg

			return trashCmdRun(&runOpts)
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

// trashCmdRun runs the TrashLazyFileWalker.
func trashCmdRun(opts *RunOptions) (err error) {
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
	var req lazyfilewalker.TrashCleanupRequest
	if err = json.NewDecoder(opts.stdin).Decode(&req); err != nil {
		return errs.New("Error decoding data from stdin: %v", err)
	}

	// Validate the request data
	switch {
	case req.SatelliteID.IsZero():
		return errs.New("SatelliteID is required")
	case req.DateBefore.IsZero():
		return errs.New("DateBefore is required")
	}

	log.Info("trash-filewalker started", zap.Time("dateBefore", req.DateBefore))

	db, err := storagenodedb.OpenExisting(opts.Ctx, log.Named("db"), opts.config.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	log.Info("Database started")
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	filewalker := pieces.NewFileWalker(log, db.Pieces(), db.V0PieceInfo(), db.GCFilewalkerProgress(), db.UsedSpacePerPrefix())
	bytesDeleted, keysDeleted, err := filewalker.WalkCleanupTrash(opts.Ctx, req.SatelliteID, req.DateBefore)
	if err != nil {
		return err
	}

	resp := lazyfilewalker.TrashCleanupResponse{
		BytesDeleted: bytesDeleted,
		KeysDeleted:  keysDeleted,
	}

	log.Info("trash-filewalker completed", zap.Int64("bytesDeleted", bytesDeleted), zap.Int("numKeysDeleted", len(keysDeleted)))

	// encode the response struct and write it to stdout
	return json.NewEncoder(opts.stdout).Encode(resp)
}
