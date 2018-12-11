// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	dbx "storj.io/storj/pkg/accounting/dbx"
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
	pointerdb   *pointerdb.Server
	overlay     pb.OverlayServer
	limit       int
	logger      *zap.Logger
	ticker      *time.Ticker
	db         	*accounting.Database
	bwAgreement bwagreement.DB // bwagreements database
}

func newTally(logger *zap.Logger, db *accounting.Database, bwAgreement bwagreement.DB, pointerdb *pointerdb.Server, overlay pb.OverlayServer, limit int, interval time.Duration) *tally {
	return &tally{
		pointerdb:   pointerdb,
		overlay:     overlay,
		limit:       limit,
		logger:      logger,
		ticker:      time.NewTicker(interval),
		db:          db,
		bwAgreement: bwAgreement,

	}
}

// Run the tally loop
func (t *tally) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = t.identifyActiveNodes(ctx)
		if err != nil {
			t.logger.Error("identifyActiveNodes failed", zap.Error(err))
		}
		err = t.Query(ctx)
		if err != nil {
			t.logger.Error("Query for bandwith failed", zap.Error(err))
		}

		select {
		case <-t.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the tally is canceled via context
			return ctx.Err()
		}
	}
}

// identifyActiveNodes iterates through pointerdb and identifies nodes that have storage on them
func (t *tally) identifyActiveNodes(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	t.logger.Debug("entering pointerdb iterate within identifyActiveNodes")
	var nodeData = make(map[string]int64)
	err = t.pointerdb.Iterate(ctx, &pb.IterateRequest{Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			lim := t.limit
			if lim <= 0 || lim > storage.LookupLimit {
				lim = storage.LookupLimit
			}
			for ; lim > 0 && it.Next(&item); lim-- {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.Wrap(err)
				}
				pieces := pointer.Remote.RemotePieces
				var nodeIDs storj.NodeIDList
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, p.NodeId)
				}
				nodeData, err = t.calculate(ctx, pointer, nodeIDs)
				if err != nil {
					return Error.Wrap(err)
				}
			}
			return nil
		},
	)
	return t.updateRawTable(ctx, nodeData)
}

func (t *tally) calculate(ctx context.Context, pointer *pb.Pointer, nodeIDs storj.NodeIDList) (nodeData map[string]int64, err error) {
	t.logger.Debug("entering categorize and calculate")
	nodeData = make(map[string]int64)
	
	segmentSize := pointer.GetSegmentSize()
	minReq := pointer.Remote.Redundancy.GetMinReq()
	if minReq <= 0 {
		return nodeData, Error.New("minReq must be an int greater than 0")
	}
	pieceSize := segmentSize / int64(minReq)

	responses, err := t.overlay.BulkLookup(ctx, pb.NodeIDsToLookupRequests(nodeIDs))
	if err != nil {
		return nodeData, Error.Wrap(err)
	}
	nodes := pb.LookupResponsesToNodes(responses)
	for i, n := range nodes {
		if n != nil {
			nodeData[n.Id.String()] += pieceSize
		} else {
			nodeData[nodeIDs[i].String()] = 0
		}
	}
	return nodeData, nil
}

func (t *tally) updateRawTable(ctx context.Context, nodeData map[string]int64) error {
	t.logger.Debug("entering updateRawTable")
	//TODO
	return nil
}

// Query bandwidth allocation database, selecting all new contracts since the last collection run time.
// Grouping by storage node ID and adding total of bandwidth to granular data table.
func (t *tally) Query(ctx context.Context) error {
	lastBwTally, err := t.db.FindLastBwTally(ctx)
	if err != nil {
		return err
	}
	var bwAgreements []bwagreement.Agreement
	if lastBwTally == nil {
		t.logger.Info("Tally found no existing bandwith tracking data")
		bwAgreements, err = t.bwAgreement.GetAgreements(ctx)
	} else {
		bwAgreements, err = t.bwAgreement.GetAgreementsSince(ctx, lastBwTally.Value)
	}
	if len(bwAgreements) == 0 {
		t.logger.Info("Tally found no new bandwidth allocations")
		return nil
	}

	// sum totals by node id ... todo: add nodeid as SQL column so DB can do this?
	bwTotals := make(map[string]int64)
	var latestBwa time.Time
	for _, baRow := range bwAgreements {
		rbad := &pb.RenterBandwidthAllocation_Data{}
		if err := proto.Unmarshal(baRow.Agreement, rbad); err != nil {
			t.logger.DPanic("Could not deserialize renter bwa in tally query")
			continue
		}
		if baRow.CreatedAt.After(latestBwa) {
			latestBwa = baRow.CreatedAt
		}
		bwTotals[rbad.StorageNodeId.String()] += rbad.GetTotal() // todo: check for overflow?
	}

	//todo:  consider if we actually need EndTime in granular
	if lastBwTally == nil {
		t.logger.Info("No previous bandwidth timestamp found in tally query")
		lastBwTally = &dbx.Value_Row{Value: latestBwa} //todo: something better here?
	}

	//insert all records in a transaction so if we fail, we don't have partial info stored
	//todo:  replace with a WithTx() method per DBX docs?
	tx, err := t.db.Open(ctx)
	if err != nil {
		t.logger.DPanic("Failed to create DB txn in tally query")
		return err
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			t.logger.Warn("DB txn was rolled back in tally query")
			err = tx.Rollback()
		}
	}()

	//todo:  switch to bulk update SQL?
	for k, v := range bwTotals {
		nID := dbx.Raw_NodeId(k)
		end := dbx.Raw_IntervalEndTime(latestBwa)
		total := dbx.Raw_DataTotal(v)
		dataType := dbx.Raw_DataType(accounting.Bandwith)
		_, err = tx.Create_Raw(ctx, nID, end, total, dataType)
		if err != nil {
			t.logger.DPanic("Create granular SQL failed in tally query")
			return err //todo: retry strategy?
		}
	}

	//todo:  move this into txn when we have masterdb?
	update := dbx.Timestamps_Update_Fields{Value: dbx.Timestamps_Value(latestBwa)}
	_, err = tx.Update_Timestamps_By_Name(ctx, accounting.LastBandwidthTally, update)
	if err != nil {
		t.logger.DPanic("Failed to update bandwith timestamp in tally query")
	}
	return err
}
