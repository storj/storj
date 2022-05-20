// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/common/pb"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/reputation"
)

func addAudit(a *internalpb.AuditHistory, auditTime time.Time, online bool, config reputation.AuditHistoryConfig) error {
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

func (reputations *reputations) UpdateAuditHistory(ctx context.Context, oldHistory []byte, updateReq reputation.UpdateRequest, auditTime time.Time) (res *reputation.UpdateAuditHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	config := updateReq.AuditHistory

	online := updateReq.AuditOutcome != reputation.AuditOffline

	res = &reputation.UpdateAuditHistoryResponse{
		NewScore:           1,
		TrackingPeriodFull: false,
	}

	// deserialize node audit history
	history := &internalpb.AuditHistory{}
	err = pb.Unmarshal(oldHistory, history)
	if err != nil {
		return res, err
	}

	err = addAudit(history, auditTime, online, config)
	if err != nil {
		return res, err
	}

	res.History, err = pb.Marshal(history)
	if err != nil {
		return res, err
	}

	windowsPerTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	res.TrackingPeriodFull = len(history.Windows)-1 >= windowsPerTrackingPeriod
	res.NewScore = history.Score
	return res, nil
}
