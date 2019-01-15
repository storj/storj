// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Tally is the service for accounting for data stored on each storage node
type Tally interface {
	Run(ctx context.Context) error
}

type tally struct {
	pointerdb     *pointerdb.Server
	overlay       pb.OverlayServer
	limit         int
	logger        *zap.Logger
	ticker        *time.Ticker
	accountingDB  accounting.DB
	bwAgreementDB bwagreement.DB // bwagreements database
}

func newTally(logger *zap.Logger, accountingDB accounting.DB, bwAgreementDB bwagreement.DB, pointerdb *pointerdb.Server, overlay pb.OverlayServer, limit int, interval time.Duration) *tally {
	return &tally{
		pointerdb:     pointerdb,
		overlay:       overlay,
		limit:         limit,
		logger:        logger,
		ticker:        time.NewTicker(interval),
		accountingDB:  accountingDB,
		bwAgreementDB: bwAgreementDB,
	}
}

// Run the tally loop
func (t *tally) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = t.calculateAtRestData(ctx)
		if err != nil {
			t.logger.Error("calculateAtRestData failed", zap.Error(err))
		}
		err = t.queryBW(ctx)
		if err != nil {
			t.logger.Error("Query for bandwidth failed", zap.Error(err))
		}

		select {
		case <-t.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the tally is canceled via context
			return ctx.Err()
		}
	}
}

// calculateAtRestData iterates through the pieces on pointerdb and calculates
// the amount of at-rest data stored on each respective node
func (t *tally) calculateAtRestData(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	latestTally, isNil, err := t.accountingDB.LastRawTime(ctx, accounting.LastAtRestTally)
	if err != nil {
		return Error.Wrap(err)
	}

	var nodeData = make(map[storj.NodeID]float64)
	err = t.pointerdb.Iterate(ctx, &pb.IterateRequest{Recurse: true},
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
					nodeData[piece.NodeId] += float64(pieceSize)
				}
			}
			return nil
		},
	)
	if len(nodeData) == 0 {
		return nil
	}
	if err != nil {
		return Error.Wrap(err)
	}
	//store byte hours, not just bytes
	numHours := 1.0 //todo: something more considered?
	if !isNil {
		numHours = time.Now().UTC().Sub(latestTally).Hours()
	}
	for k := range nodeData {
		nodeData[k] *= numHours
	}
	return Error.Wrap(t.accountingDB.SaveAtRestRaw(ctx, latestTally, nodeData))
}

// queryBW queries bandwidth allocation database, selecting all new contracts since the last collection run time.
// Grouping by action type, storage node ID and adding total of bandwidth to granular data table.
func (t *tally) queryBW(ctx context.Context) error {
	lastBwTally, isNil, err := t.accountingDB.LastRawTime(ctx, accounting.LastBandwidthTally)
	if err != nil {
		return Error.Wrap(err)
	}

	var bwAgreements []bwagreement.Agreement
	if isNil {
		t.logger.Info("Tally found no existing bandwidth tracking data")
		bwAgreements, err = t.bwAgreementDB.GetAgreements(ctx)
	} else {
		bwAgreements, err = t.bwAgreementDB.GetAgreementsSince(ctx, lastBwTally)
	}
	if err != nil {
		return Error.Wrap(err)
	}
	if len(bwAgreements) == 0 {
		t.logger.Info("Tally found no new bandwidth allocations")
		return nil
	}

	// sum totals by node id ... todo: add nodeid as SQL column so DB can do this?
	var bwTotals accounting.BWTally
	for i := range bwTotals {
		bwTotals[i] = make(map[storj.NodeID]int64)
	}
	var latestBwa time.Time
	for _, baRow := range bwAgreements {
		rbad := &pb.RenterBandwidthAllocation_Data{}
		if err := proto.Unmarshal(baRow.Agreement, rbad); err != nil {
			t.logger.DPanic("Could not deserialize renter bwa in tally query")
			continue
		}
		pbad := &pb.PayerBandwidthAllocation_Data{}
		if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
			return err
		}
		if baRow.CreatedAt.After(latestBwa) {
			latestBwa = baRow.CreatedAt
		}
		bwTotals[pbad.GetAction()][rbad.StorageNodeId] += rbad.GetTotal()
	}
	return Error.Wrap(t.accountingDB.SaveBWRaw(ctx, lastBwTally, bwTotals))
}
