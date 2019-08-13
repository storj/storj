package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type BatchUpdateStats struct {
	DB
	flushInterval time.Duration
	batchSize     int
	updateReqCh   chan *overlay.UpdateRequest
}

func NewBatchUpdateStats(db DB, flushInterval time.Duration, batchSize int) *BatchUpdateStats {
	return &BatchUpdateStats{
		DB:            db,
		flushInterval: flushInterval,
		batchSize:     batchSize,
		updateReqCh:   make(chan *overlay.UpdateRequest),
	}
}

func (b *BatchUpdateStats) UpdateStats(ctx context.Context, request *overlay.UpdateRequest) (err error) {
	b.updateReqCh <- request
	return nil
}

func (b *BatchUpdateStats) worker(ctx context.Context) {
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	var updateRequests []*overlay.UpdateRequest

	for {
		select {
		case <-ctx.Done():
			return
		case updateRequest := <-b.updateReqCh:
			updateRequests = append(updateRequests, updateRequest)
		case <-ticker.C:
			b.commitRequests(ctx, updateRequests)
			updateRequests = updateRequests[:0]
		}
	}
}

func dbNodeToUpdateStats(dbNode *dbx.Node) (*updateNodeStats, error) {
	nodeID, err := storj.NodeIDFromBytes(dbNode.Id)
	if err != nil {
		return nil, err
	}
	return &updateNodeStats{
		NodeID:                nodeID,
		TotalAuditCount:       int64Field{value: dbNode.TotalAuditCount},
		TotalUptimeCount:      int64Field{value: dbNode.TotalUptimeCount},
		AuditReputationAlpha:  float64Field{value: dbNode.AuditReputationAlpha},
		AuditReputationBeta:   float64Field{value: dbNode.AuditReputationBeta},
		UptimeReputationAlpha: float64Field{value: dbNode.UptimeReputationAlpha},
		UptimeReputationBeta:  float64Field{value: dbNode.UptimeReputationBeta},
		Disqualified:          timeField{value: *dbNode.Disqualified},
		UptimeSuccessCount:    int64Field{value: dbNode.UptimeSuccessCount},
		LastContactSuccess:    timeField{value: dbNode.LastContactSuccess},
		LastContactFailure:    timeField{value: dbNode.LastContactFailure},
		AuditSuccessCount:     int64Field{value: dbNode.AuditSuccessCount},
		Contained:             boolField{value: dbNode.Contained},
	}, nil
}

func reduceUpdateRequests(dbNode *dbx.Node, updateRequests []*overlay.UpdateRequest) (*updateNodeStats, error) {
	stats, err := dbNodeToUpdateStats(dbNode)
	if err != nil {
		return nil, err
	}

	for _, updateRequest := range updateRequests {
		populateUpdateNodeStats(stats, updateRequest)
	}

	return stats, nil
}

func (b *BatchUpdateStats) commitRequests(ctx context.Context, updateRequests []*overlay.UpdateRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Sort the update requests into separate slices for each NodeID, so that
	// each NodeID is only updated once.
	updateRequestsByNodeID := make(map[storj.NodeID][]*overlay.UpdateRequest)
	for _, updateRequest := range updateRequests {
		updateRequestsByNodeID[updateRequest.NodeID] = append(updateRequestsByNodeID[updateRequest.NodeID], updateRequest)
	}

	// Create slice of nodeIDs so we can easily batch the process
	nodeIDs := make([]storj.NodeID, len(updateRequestsByNodeID))
	i := 0
	for nodeID := range updateRequestsByNodeID {
		nodeIDs[i] = nodeID
		i++
	}

	doUpdate := func(nodeIDs []storj.NodeID) (err error) {
		tx, err := b.db.Open(ctx)
		if err != nil {
			return Error.Wrap(err)
		}

		var allSQL string

		for _, nodeID := range nodeIDs {
			dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
			if err != nil {
				return Error.Wrap(errs.Combine(err, tx.Rollback()))
			}

			// do not update reputation if node is disqualified
			if dbNode.Disqualified != nil {
				continue
			}

			updateNodeStats, err := reduceUpdateRequests(dbNode, updateRequestsByNodeID[nodeID])
			if err != nil {
				return err
			}

			allSQL += buildUpdateStatement(b.db, updateNodeStats)
		}

		if allSQL != "" {
			_, err := tx.Tx.Exec(allSQL)
			if err != nil {
				return errs.Combine(err, tx.Rollback())
			}
		}
		return Error.Wrap(tx.Commit())
	}

	var errlist errs.Group
	length := len(nodeIDs)
	for i := 0; i < length; i += b.batchSize {
		end := i + b.batchSize
		if end > length {
			end = length
		}

		// TODO(isaac): Log an error, or determine if we need a separate error
		// for each node in the failed batch
		err := doUpdate(nodeIDs[i:end])
		if err != nil {
			return err
		}
	}
	return errlist.Err()
}
