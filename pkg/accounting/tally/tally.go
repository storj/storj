// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Config contains configurable values for tally
type Config struct {
	Interval time.Duration `help:"how frequently tally should run" default:"30s"`
}

// Tally is the service for accounting for data stored on each storage node
type Tally struct { // TODO: rename Tally to Service
	pointerdb     *pointerdb.Service
	overlay       pb.OverlayServer // TODO: this should be *overlay.Service
	limit         int
	logger        *zap.Logger
	ticker        *time.Ticker
	accountingDB  accounting.DB
	bwAgreementDB bwagreement.DB // bwagreements database
}

// New creates a new Tally
func New(logger *zap.Logger, accountingDB accounting.DB, bwAgreementDB bwagreement.DB, pointerdb *pointerdb.Service, overlay pb.OverlayServer, limit int, interval time.Duration) *Tally {
	return &Tally{
		pointerdb:     pointerdb,
		overlay:       overlay,
		limit:         limit,
		logger:        logger,
		ticker:        time.NewTicker(interval),
		accountingDB:  accountingDB,
		bwAgreementDB: bwAgreementDB,
	}
}

// Run the Tally loop
func (t *Tally) Run(ctx context.Context) (err error) {
	t.logger.Info("Tally service starting up")
	defer mon.Task()(&ctx)(&err)
	for {
		if err = t.Tally(ctx); err != nil {
			t.logger.Error("Tally failed", zap.Error(err))
		}
		select {
		case <-t.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the Tally is canceled via context
			return ctx.Err()
		}
	}
}

//Tally calculates data-at-rest and bandwidth usage once
func (t *Tally) Tally(ctx context.Context) error {
	//data at rest
	var errAtRest, errBWA error
	latestTally, nodeData, err := t.calculateAtRestData(ctx)
	if err != nil {
		errAtRest = errs.New("Query for data-at-rest failed : %v", err)
	} else if len(nodeData) > 0 {
		err = t.SaveAtRestRaw(ctx, latestTally, time.Now().UTC(), nodeData)
		if err != nil {
			errAtRest = errs.New("Saving data-at-rest failed : %v", err)
		}
	}
	//bandwdith
	tallyEnd, bwTotals, err := t.QueryBW(ctx)
	if err != nil {
		errBWA = errs.New("Query for bandwidth failed: %v", err)
	} else if len(bwTotals) > 0 {
		err = t.SaveBWRaw(ctx, tallyEnd, time.Now().UTC(), bwTotals)
		if err != nil {
			errBWA = errs.New("Saving for bandwidth failed : %v", err)
		}
	}
	return errs.Combine(errAtRest, errBWA)
}

// calculateAtRestData iterates through the pieces on pointerdb and calculates
// the amount of at-rest data stored on each respective node
func (t *Tally) calculateAtRestData(ctx context.Context) (latestTally time.Time, nodeData map[storj.NodeID]float64, err error) {
	defer mon.Task()(&ctx)(&err)

	latestTally, err = t.accountingDB.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return latestTally, nodeData, Error.Wrap(err)
	}
	nodeData = make(map[storj.NodeID]float64)

	err = t.pointerdb.Iterate("", "", true, false,
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.Wrap(err)
				}
				remote := pointer.GetRemote()
				if remote == nil {
					continue
				}
				pieces := remote.GetRemotePieces()
				if pieces == nil {
					t.logger.Debug("no pieces on remote segment")
					continue
				}
				segmentSize := pointer.GetSegmentSize()
				redundancy := remote.GetRedundancy()
				if redundancy == nil {
					t.logger.Debug("no redundancy scheme present")
					continue
				}
				minReq := redundancy.GetMinReq()
				if minReq <= 0 {
					t.logger.Debug("pointer minReq must be an int greater than 0")
					continue
				}
				pieceSize := segmentSize / int64(minReq)
				for _, piece := range pieces {
					t.logger.Info("found piece on Node ID" + piece.NodeId.String())
					nodeData[piece.NodeId] += float64(pieceSize)
				}
			}
			return nil
		},
	)
	if len(nodeData) == 0 {
		return latestTally, nodeData, nil
	}
	if err != nil {
		return latestTally, nodeData, Error.Wrap(err)
	}
	//store byte hours, not just bytes
	numHours := time.Now().Sub(latestTally).Hours()
	if latestTally.IsZero() {
		numHours = 1.0 //todo: something more considered?
	}
	latestTally = time.Now()
	for k := range nodeData {
		nodeData[k] *= numHours //calculate byte hours
	}
	return latestTally, nodeData, err
}

// SaveAtRestRaw records raw tallies of at-rest-data and updates the LastTimestamp
func (t *Tally) SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error {
	return t.accountingDB.SaveAtRestRaw(ctx, latestTally, created, nodeData)
}

// QueryBW queries bandwidth allocation database, selecting all new contracts since the last collection run time.
// Grouping by action type, storage node ID and adding total of bandwidth to granular data table.
func (t *Tally) QueryBW(ctx context.Context) (time.Time, map[storj.NodeID][]int64, error) {
	var bwTotals map[storj.NodeID][]int64
	now := time.Now()
	lastBwTally, err := t.accountingDB.LastTimestamp(ctx, accounting.LastBandwidthTally)
	if err != nil {
		return now, bwTotals, Error.Wrap(err)
	}
	bwTotals, err = t.bwAgreementDB.GetTotals(ctx, lastBwTally, now)
	if err != nil {
		return now, bwTotals, Error.Wrap(err)
	}
	if len(bwTotals) == 0 {
		t.logger.Info("Tally found no new bandwidth allocations")
		return now, bwTotals, nil
	}
	return now, bwTotals, nil
}

// SaveBWRaw records granular tallies (sums of bw agreement values) to the database and updates the LastTimestamp
func (t *Tally) SaveBWRaw(ctx context.Context, tallyEnd time.Time, created time.Time, bwTotals map[storj.NodeID][]int64) error {
	return t.accountingDB.SaveBWRaw(ctx, tallyEnd, created, bwTotals)
}
