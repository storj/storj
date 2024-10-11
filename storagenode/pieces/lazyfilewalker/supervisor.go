// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
)

const (
	// UsedSpaceFilewalkerCmdName is the name of the used-space-filewalker subcommand.
	UsedSpaceFilewalkerCmdName = "used-space-filewalker"
	// GCFilewalkerCmdName is the name of the gc-filewalker subcommand.
	GCFilewalkerCmdName = "gc-filewalker"
	// TrashCleanupFilewalkerCmdName is the name of the trash-cleanup-filewalker subcommand.
	TrashCleanupFilewalkerCmdName = "trash-cleanup-filewalker"
)

var (
	errLazyFilewalker = errs.Class("lazyfilewalker")

	mon = monkit.Package()
)

// Supervisor manages the lazyfilewalker subprocesses.
//
// TODO: we should keep track of the number of subprocesses we have running and
// limit it to a configurable number, and queue them, since they are run per satellite.
type Supervisor struct {
	log *zap.Logger

	executable       string
	gcArgs           []string
	usedSpaceArgs    []string
	trashCleanupArgs []string

	testingGCCmd           execwrapper.Command
	testingUsedSpaceCmd    execwrapper.Command
	testingTrashCleanupCmd execwrapper.Command
}

// NewSupervisor creates a new lazy filewalker Supervisor.
func NewSupervisor(log *zap.Logger, config Config, executable string) *Supervisor {
	return &Supervisor{
		log:              log,
		gcArgs:           append([]string{GCFilewalkerCmdName}, config.Args()...),
		usedSpaceArgs:    append([]string{UsedSpaceFilewalkerCmdName}, config.Args()...),
		trashCleanupArgs: append([]string{TrashCleanupFilewalkerCmdName}, config.Args()...),
		executable:       executable,
	}
}

// TestingSetGCCmd sets the command for the gc-filewalker subprocess.
// The cmd acts as a replacement for the subprocess.
func (fw *Supervisor) TestingSetGCCmd(cmd execwrapper.Command) {
	fw.testingGCCmd = cmd
}

// TestingSetUsedSpaceCmd sets the command for the used-space-filewalker subprocess.
// The cmd acts as a replacement for the subprocess.
func (fw *Supervisor) TestingSetUsedSpaceCmd(cmd execwrapper.Command) {
	fw.testingUsedSpaceCmd = cmd
}

// TestingSetTrashCleanupCmd sets the command for the trash cleanup filewalker subprocess.
// The cmd acts as a replacement for the subprocess.
func (fw *Supervisor) TestingSetTrashCleanupCmd(cmd execwrapper.Command) {
	fw.testingTrashCleanupCmd = cmd
}

// UsedSpaceRequest is the request struct for the used-space-filewalker process.
type UsedSpaceRequest struct {
	SatelliteID storj.NodeID `json:"satelliteID"`
}

// UsedSpaceResponse is the response struct for the used-space-filewalker process.
type UsedSpaceResponse struct {
	PiecesTotal       int64 `json:"piecesTotal"`
	PiecesContentSize int64 `json:"piecesContentSize"`
	PieceCount        int64 `json:"pieceCount"`
}

// GCFilewalkerRequest is the request struct for the gc-filewalker process.
type GCFilewalkerRequest struct {
	SatelliteID   storj.NodeID `json:"satelliteID"`
	BloomFilter   []byte       `json:"bloomFilter"`
	CreatedBefore time.Time    `json:"createdBefore"`
}

// GCFilewalkerResponse is the response struct for the gc-filewalker process.
type GCFilewalkerResponse struct {
	// PieceIDs is the list of trash pieces that were found.
	// Final message will not return any pieceIDs.
	PieceIDs           []storj.PieceID `json:"pieceIDs"`
	PiecesSkippedCount int64           `json:"piecesSkippedCount"`
	PiecesCount        int64           `json:"piecesCount"`
	// Completed indicates if this is the final message.
	Completed bool `json:"completed"`
}

// TrashCleanupRequest is the request struct for the trash-cleanup-filewalker process.
type TrashCleanupRequest struct {
	SatelliteID storj.NodeID `json:"satelliteID"`
	DateBefore  time.Time    `json:"dateBefore"`
}

// TrashCleanupResponse is the response struct for the trash-cleanup-filewalker process.
type TrashCleanupResponse struct {
	BytesDeleted int64           `json:"bytesDeleted"`
	KeysDeleted  []storj.PieceID `json:"keysDeleted"`
}

// WalkAndComputeSpaceUsedBySatellite returns the total used space by satellite.
func (fw *Supervisor) WalkAndComputeSpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (piecesTotal int64, piecesContentSize int64, pieceCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	req := UsedSpaceRequest{
		SatelliteID: satelliteID,
	}
	var resp UsedSpaceResponse

	log := fw.log.Named(UsedSpaceFilewalkerCmdName).With(zap.Stringer("satelliteID", satelliteID))

	stdout := newGenericWriter(log)
	err = newProcess(fw.testingUsedSpaceCmd, log, fw.executable, fw.usedSpaceArgs).run(ctx, stdout, req)
	if err != nil {
		return 0, 0, 0, err
	}

	if err := stdout.Decode(&resp); err != nil {
		return 0, 0, 0, err
	}

	return resp.PiecesTotal, resp.PiecesContentSize, resp.PieceCount, nil
}

// WalkSatellitePiecesToTrash walks the satellite pieces and moves the pieces that are trash to the trash using the trashFunc provided.
func (fw *Supervisor) WalkSatellitePiecesToTrash(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, filter *bloomfilter.Filter, trashFunc func(pieceID storj.PieceID) error) (piecesCount, piecesSkipped int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if filter == nil {
		return 0, 0, nil
	}

	req := GCFilewalkerRequest{
		SatelliteID:   satelliteID,
		BloomFilter:   filter.Bytes(),
		CreatedBefore: createdBefore,
	}
	var resp GCFilewalkerResponse

	log := fw.log.Named(GCFilewalkerCmdName).With(zap.Stringer("satelliteID", satelliteID))

	stdout := NewTrashHandler(log, trashFunc)
	err = newProcess(fw.testingGCCmd, log, fw.executable, fw.gcArgs).run(ctx, stdout, req)
	if err != nil {
		return 0, 0, err
	}

	if err := stdout.Decode(&resp); err != nil {
		return 0, 0, err
	}

	if !resp.Completed {
		// Something went wrong. The filewalker did not complete
		log.Warn("gc-filewalker did not complete")
	}

	return resp.PiecesCount, resp.PiecesSkippedCount, nil
}

// WalkCleanupTrash deletes per-day trash directories which are older than the given time.
// The lazyfilewalker does not update the space used by the trash so the caller should update the space used
// after the filewalker completes.
func (fw *Supervisor) WalkCleanupTrash(ctx context.Context, satelliteID storj.NodeID, dateBefore time.Time) (bytesDeleted int64, keysDeleted []storj.PieceID, err error) {
	defer mon.Task()(&ctx)(&err)

	req := TrashCleanupRequest{
		SatelliteID: satelliteID,
		DateBefore:  dateBefore,
	}
	var resp TrashCleanupResponse

	log := fw.log.Named(TrashCleanupFilewalkerCmdName).With(zap.Stringer("satelliteID", satelliteID))

	stdout := newGenericWriter(log)
	err = newProcess(fw.testingTrashCleanupCmd, log, fw.executable, fw.trashCleanupArgs).run(ctx, stdout, req)
	if err != nil {
		return 0, nil, err
	}
	if err := stdout.Decode(&resp); err != nil {
		return 0, nil, err
	}

	return resp.BytesDeleted, resp.KeysDeleted, nil
}
