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
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type auditHistory pb.AuditHistory

// auditHistoryFromBytes deserializes a byte array to get an auditHistory struct.
func auditHistoryFromBytes(b []byte) (*auditHistory, error) {
	a := &pb.AuditHistory{}
	err := pb.Unmarshal(b, a)
	return (*auditHistory)(a), err
}

// bytes serializes an auditHistory struct into a byte slice.
func (a *auditHistory) bytes() ([]byte, error) {
	return pb.Marshal((*pb.AuditHistory)(a))
}

func (a *auditHistory) addAudit(auditTime time.Time, online bool, config overlay.AuditHistoryConfig) error {
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
		a.Windows = append(a.Windows, &pb.AuditWindow{WindowStart: newAuditWindowStartTime})
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

	// if we do not have enough completed windows to fill a tracking period (exclude latest window), score should be 1
	windowsPerTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	if len(a.Windows)-1 < windowsPerTrackingPeriod {
		a.Score = 1
		return nil
	}

	// if no windows were added or removed, score does not change
	if !windowsModified {
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

	if len(a.Windows) <= 1 {
		return Error.New("not enough windows to calculate score; this should not happen")
	}
	// divide by number of windows-1 because last window is not included
	a.Score = totalWindowScores / float64(len(a.Windows)-1)
	return nil
}

// UpdateAuditHistory updates a node's audit history with an online or offline audit and returns the online score for the tracking period.
func (cache *overlaycache) UpdateAuditHistory(ctx context.Context, nodeID storj.NodeID, auditTime time.Time, online bool, config overlay.AuditHistoryConfig) (onlineScore float64, err error) {
	err = cache.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		_, err = tx.Tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
		if err != nil {
			return err
		}

		onlineScore, err = cache.updateAuditHistoryWithTx(ctx, tx, nodeID, auditTime, online, config)
		if err != nil {
			return err
		}
		return nil
	})
	return onlineScore, err
}

func (cache *overlaycache) updateAuditHistoryWithTx(ctx context.Context, tx *dbx.Tx, nodeID storj.NodeID, auditTime time.Time, online bool, config overlay.AuditHistoryConfig) (onlineScore float64, err error) {
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
		return 0, Error.Wrap(err)
	} else {
		historyBytes = dbAuditHistory.History
	}

	history, err := auditHistoryFromBytes(historyBytes)
	if err != nil {
		return 0, err
	}

	err = history.addAudit(auditTime, online, config)
	if err != nil {
		return 0, err
	}

	historyBytes, err = history.bytes()
	if err != nil {
		return 0, err
	}

	// if the entry did not exist at the beginning, create a new one. Otherwise update
	if newEntry {
		_, err = tx.Create_AuditHistory(
			ctx,
			dbx.AuditHistory_NodeId(nodeID.Bytes()),
			dbx.AuditHistory_History(historyBytes),
		)
		return history.Score, Error.Wrap(err)
	}

	_, err = tx.Update_AuditHistory_By_NodeId(
		ctx,
		dbx.AuditHistory_NodeId(nodeID.Bytes()),
		dbx.AuditHistory_Update_Fields{
			History: dbx.AuditHistory_History(historyBytes),
		},
	)

	return history.Score, Error.Wrap(err)
}
