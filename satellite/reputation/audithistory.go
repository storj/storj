// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"time"

	"storj.io/common/pb"
	"storj.io/storj/satellite/internalpb"
)

// AuditHistory represents a node's audit history for the most recent tracking period.
type AuditHistory struct {
	Score   float64
	Windows []*AuditWindow
}

// UpdateAuditHistoryResponse contains information returned by UpdateAuditHistory.
type UpdateAuditHistoryResponse struct {
	NewScore           float64
	TrackingPeriodFull bool
	History            []byte
}

// AuditWindow represents the number of online and total audits a node received for a specific time period.
type AuditWindow struct {
	WindowStart time.Time
	TotalCount  int32
	OnlineCount int32
}

// AuditHistoryToPB converts an overlay.AuditHistory to a pb.AuditHistory.
func AuditHistoryToPB(auditHistory AuditHistory) (historyPB *pb.AuditHistory) {
	historyPB = &pb.AuditHistory{
		Score:   auditHistory.Score,
		Windows: make([]*pb.AuditWindow, len(auditHistory.Windows)),
	}
	for i, window := range auditHistory.Windows {
		historyPB.Windows[i] = &pb.AuditWindow{
			TotalCount:  window.TotalCount,
			OnlineCount: window.OnlineCount,
			WindowStart: window.WindowStart,
		}
	}
	return historyPB
}

// AuditHistoryFromBytes returns the corresponding AuditHistory for the given
// encoded internalpb.AuditHistory.
func AuditHistoryFromBytes(encodedHistory []byte) (history AuditHistory, err error) {
	var pbHistory internalpb.AuditHistory
	if err := pb.Unmarshal(encodedHistory, &pbHistory); err != nil {
		return AuditHistory{}, err
	}
	history.Score = pbHistory.Score
	history.Windows = make([]*AuditWindow, len(pbHistory.Windows))
	for i, window := range pbHistory.Windows {
		history.Windows[i] = &AuditWindow{
			TotalCount:  window.TotalCount,
			OnlineCount: window.OnlineCount,
			WindowStart: window.WindowStart,
		}
	}
	return history, nil
}

// AddAuditToHistory adds a single online/not-online event to an AuditHistory.
// If the AuditHistory contains windows that are now outside the tracking
// period, those windows will be trimmed.
func AddAuditToHistory(a *internalpb.AuditHistory, online bool, auditTime time.Time, config AuditHistoryConfig) error {
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
