// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/dbx"
)

func addAudit(a *internalpb.AuditHistory, auditTime time.Time, online bool, config overlay.AuditHistoryConfig) error {
	newAuditWindowStartTime := auditTime.Truncate(config.WindowSize)
	earliestWindow := newAuditWindowStartTime.Add(-config.TrackingPeriod)
	// windowsModified is used to determine whether we will need to recalculate the score because windows have been added or removed.
	windowsModified := false

	// delete windows outside of tracking period scope
	updatedWindows := a.Windows
	for i, window := range a.Windows {
		if window.WindowStart.Before(earliestWindow) {
			updatedWindows = a.Windows[i+1:]
			windowsModified = true
		} else {
			// windows are in order, so if this window is in the tracking period, we are done deleting windows
			break
		}
	}
	a.Windows = updatedWindows

	// if there are no windows or the latest window has passed, add another window
	if len(a.Windows) == 0 || a.Windows[len(a.Windows)-1].WindowStart.Before(newAuditWindowStartTime) {
		windowsModified = true
		a.Windows = append(a.Windows, &internalpb.AuditWindow{WindowStart: newAuditWindowStartTime})
	}

	latestIndex := len(a.Windows) - 1
	if a.Windows[latestIndex].WindowStart.After(newAuditWindowStartTime) {
		return Error.New("cannot add audit to audit history; window already passed")
	}

	// add new audit to latest window
	if online {
		a.Windows[latestIndex].OnlineCount++
	}
	a.Windows[latestIndex].TotalCount++

	// if no windows were added or removed, score does not change
	if !windowsModified {
		return nil
	}

	if len(a.Windows) <= 1 {
		a.Score = 1
		return nil
	}

	totalWindowScores := 0.0
	for i, window := range a.Windows {
		// do not include last window in score
		if i+1 == len(a.Windows) {
			break
		}
		totalWindowScores += float64(window.OnlineCount) / float64(window.TotalCount)
	}

	// divide by number of windows-1 because last window is not included
	a.Score = totalWindowScores / float64(len(a.Windows)-1)
	return nil
}

// UpdateAuditHistory updates a node's audit history with an online or offline audit.
func (cache *overlaycache) UpdateAuditHistory(ctx context.Context, nodeID storj.NodeID, auditTime time.Time, online bool, config overlay.AuditHistoryConfig) (res *overlay.UpdateAuditHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = cache.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		_, err = tx.Tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
		if err != nil {
			return err
		}

		res, err = cache.updateAuditHistoryWithTx(ctx, tx, nodeID, auditTime, online, config)
		if err != nil {
			return err
		}
		return nil
	})
	return res, err
}

func (cache *overlaycache) updateAuditHistoryWithTx(ctx context.Context, tx *dbx.Tx, nodeID storj.NodeID, auditTime time.Time, online bool, config overlay.AuditHistoryConfig) (res *overlay.UpdateAuditHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	res = &overlay.UpdateAuditHistoryResponse{
		NewScore:           1,
		TrackingPeriodFull: false,
	}

	// get and deserialize node audit history
	historyBytes := []byte{}
	newEntry := false
	dbAuditHistory, err := tx.Get_AuditHistory_By_NodeId(
		ctx,
		dbx.AuditHistory_NodeId(nodeID.Bytes()),
	)
	if errs.Is(err, sql.ErrNoRows) {
		// set flag to true so we know to create rather than update later
		newEntry = true
	} else if err != nil {
		return res, Error.Wrap(err)
	} else {
		historyBytes = dbAuditHistory.History
	}

	history := &internalpb.AuditHistory{}
	err = pb.Unmarshal(historyBytes, history)
	if err != nil {
		return res, err
	}

	err = addAudit(history, auditTime, online, config)
	if err != nil {
		return res, err
	}

	historyBytes, err = pb.Marshal(history)
	if err != nil {
		return res, err
	}

	// if the entry did not exist at the beginning, create a new one. Otherwise update
	if newEntry {
		_, err = tx.Create_AuditHistory(
			ctx,
			dbx.AuditHistory_NodeId(nodeID.Bytes()),
			dbx.AuditHistory_History(historyBytes),
		)
		return res, Error.Wrap(err)
	}

	_, err = tx.Update_AuditHistory_By_NodeId(
		ctx,
		dbx.AuditHistory_NodeId(nodeID.Bytes()),
		dbx.AuditHistory_Update_Fields{
			History: dbx.AuditHistory_History(historyBytes),
		},
	)

	windowsPerTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	res.TrackingPeriodFull = len(history.Windows)-1 >= windowsPerTrackingPeriod
	res.NewScore = history.Score
	return res, Error.Wrap(err)
}

// GetAuditHistory gets a node's audit history.
func (cache *overlaycache) GetAuditHistory(ctx context.Context, nodeID storj.NodeID) (auditHistory *overlay.AuditHistory, err error) {
	defer mon.Task()(&ctx)(&err)

	dbAuditHistory, err := cache.db.Get_AuditHistory_By_NodeId(
		ctx,
		dbx.AuditHistory_NodeId(nodeID.Bytes()),
	)
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return nil, overlay.ErrNodeNotFound.New("no audit history for node")
		}
		return nil, err
	}
	history, err := auditHistoryFromPB(dbAuditHistory.History)
	if err != nil {
		return nil, err
	}
	return history, nil
}

func auditHistoryFromPB(historyBytes []byte) (auditHistory *overlay.AuditHistory, err error) {
	historyPB := &internalpb.AuditHistory{}
	err = pb.Unmarshal(historyBytes, historyPB)
	if err != nil {
		return nil, err
	}
	history := &overlay.AuditHistory{
		Score:   historyPB.Score,
		Windows: make([]*overlay.AuditWindow, len(historyPB.Windows)),
	}
	for i, window := range historyPB.Windows {
		history.Windows[i] = &overlay.AuditWindow{
			TotalCount:  window.TotalCount,
			OnlineCount: window.OnlineCount,
			WindowStart: window.WindowStart,
		}
	}
	return history, nil
}
