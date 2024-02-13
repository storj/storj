// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
)

const (
	// UsedSpaceFilewalkerCmdName is the name of the used-space-filewalker subcommand.
	UsedSpaceFilewalkerCmdName = "used-space-filewalker"
	// GCFilewalkerCmdName is the name of the gc-filewalker subcommand.
	GCFilewalkerCmdName = "gc-filewalker"
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

	executable    string
	gcArgs        []string
	usedSpaceArgs []string

	testingGCCmd        execwrapper.Command
	testingUsedSpaceCmd execwrapper.Command
}

// NewSupervisor creates a new lazy filewalker Supervisor.
func NewSupervisor(log *zap.Logger, config Config, executable string) *Supervisor {
	return &Supervisor{
		log:           log,
		gcArgs:        append([]string{GCFilewalkerCmdName}, config.Args()...),
		usedSpaceArgs: append([]string{UsedSpaceFilewalkerCmdName}, config.Args()...),
		executable:    executable,
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

// UsedSpaceRequest is the request struct for the used-space-filewalker process.
type UsedSpaceRequest struct {
	SatelliteID storj.NodeID `json:"satelliteID"`
}

// UsedSpaceResponse is the response struct for the used-space-filewalker process.
type UsedSpaceResponse struct {
	PiecesTotal       int64 `json:"piecesTotal"`
	PiecesContentSize int64 `json:"piecesContentSize"`
}

// GCFilewalkerRequest is the request struct for the gc-filewalker process.
type GCFilewalkerRequest struct {
	SatelliteID   storj.NodeID `json:"satelliteID"`
	BloomFilter   []byte       `json:"bloomFilter"`
	CreatedBefore time.Time    `json:"createdBefore"`
}

// GCFilewalkerResponse is the response struct for the gc-filewalker process.
type GCFilewalkerResponse struct {
	PieceIDs           []storj.PieceID `json:"pieceIDs"`
	PiecesSkippedCount int64           `json:"piecesSkippedCount"`
	PiecesCount        int64           `json:"piecesCount"`
}

// WalkAndComputeSpaceUsedBySatellite returns the total used space by satellite.
func (fw *Supervisor) WalkAndComputeSpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (piecesTotal int64, piecesContentSize int64, err error) {
	defer mon.Task()(&ctx)(&err)

	req := UsedSpaceRequest{
		SatelliteID: satelliteID,
	}
	var resp UsedSpaceResponse

	log := fw.log.Named(UsedSpaceFilewalkerCmdName).With(zap.String("satelliteID", satelliteID.String()))

	err = newProcess(fw.testingUsedSpaceCmd, log, fw.executable, fw.usedSpaceArgs).run(ctx, req, &resp)
	if err != nil {
		return 0, 0, err
	}

	return resp.PiecesTotal, resp.PiecesContentSize, nil
}

// WalkSatellitePiecesToTrash returns a list of pieceIDs that need to be trashed for the given satellite.
func (fw *Supervisor) WalkSatellitePiecesToTrash(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, filter *bloomfilter.Filter) (pieceIDs []storj.PieceID, piecesCount, piecesSkipped int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if filter == nil {
		return
	}

	req := GCFilewalkerRequest{
		SatelliteID:   satelliteID,
		BloomFilter:   filter.Bytes(),
		CreatedBefore: createdBefore,
	}
	var resp GCFilewalkerResponse

	log := fw.log.Named(GCFilewalkerCmdName).With(zap.String("satelliteID", satelliteID.String()))

	err = newProcess(fw.testingGCCmd, log, fw.executable, fw.gcArgs).run(ctx, req, &resp)
	if err != nil {
		return nil, 0, 0, err
	}

	return resp.PieceIDs, resp.PiecesCount, resp.PiecesSkippedCount, nil
}
