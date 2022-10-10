// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"time"

	"storj.io/common/pb"
)

// UpdateAuditHistoryResponse contains information returned by UpdateAuditHistory.
type UpdateAuditHistoryResponse struct {
	NewScore           float64
	TrackingPeriodFull bool
	History            []byte
}

// DuplicateAuditHistory creates a duplicate (deep copy) of an AuditHistory object.
func DuplicateAuditHistory(auditHistory *pb.AuditHistory) *pb.AuditHistory {
	// argument is not a pointer type, so auditHistory is already a copy.
	// Just need to copy the slice.
	if auditHistory == nil {
		return nil
	}
	windows := make([]*pb.AuditWindow, len(auditHistory.Windows))
	for i := range windows {
		windows[i] = &pb.AuditWindow{}
		*windows[i] = *auditHistory.Windows[i]
	}
	auditHistory.Windows = windows
	return auditHistory
}

// AddAuditToHistory adds a single online/not-online event to an AuditHistory.
// If the AuditHistory contains windows that are now outside the tracking
// period, those windows will be trimmed.
func AddAuditToHistory(a *pb.AuditHistory, online bool, auditTime time.Time, config AuditHistoryConfig) error {
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

// MergeAuditHistories merges two audit histories into one, including all
// windows that are present in either input and summing counts for
// any windows that appear in _both_ inputs. Any windows that are now outside
// the tracking period will be trimmed.
//
// The history parameter will be mutated to include the windows passed as
// addHistory.
//
// Returns true if the number of windows in the new history is the maximum
// possible for the tracking config.
func MergeAuditHistories(history *pb.AuditHistory, addHistory []*pb.AuditWindow, config AuditHistoryConfig) (trackingPeriodFull bool) {
	windows := history.Windows

	for addIndex, windowIndex := 0, 0; addIndex < len(addHistory); {
		switch {
		case windowIndex == len(windows):
			windows = append(windows, &pb.AuditWindow{
				WindowStart: addHistory[addIndex].WindowStart,
			})
			fallthrough
		case windows[windowIndex].WindowStart.Equal(addHistory[addIndex].WindowStart):
			windows[windowIndex].TotalCount += addHistory[addIndex].TotalCount
			windows[windowIndex].OnlineCount += addHistory[addIndex].OnlineCount
			addIndex++
		case windows[windowIndex].WindowStart.Before(addHistory[addIndex].WindowStart):
			windowIndex++
		case windows[windowIndex].WindowStart.After(addHistory[addIndex].WindowStart):
			windows = append(windows[:windowIndex+1], windows[windowIndex:]...)
			windows[windowIndex] = &pb.AuditWindow{
				WindowStart: addHistory[addIndex].WindowStart,
				TotalCount:  addHistory[addIndex].TotalCount,
				OnlineCount: addHistory[addIndex].OnlineCount,
			}
			addIndex++
		}
	}

	// trim off windows that are too old
	if len(windows) > 0 {
		cutoffTime := windows[len(windows)-1].WindowStart.Add(-config.TrackingPeriod)
		for len(windows) > 0 && windows[0].WindowStart.Before(cutoffTime) {
			windows = windows[1:]
		}
	}

	history.Windows = windows
	RecalculateScore(history)

	windowsPerTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	trackingPeriodFull = len(history.Windows)-1 >= windowsPerTrackingPeriod

	return trackingPeriodFull
}

// RecalculateScore calculates and assigns the Score field in a pb.AuditHistory object.
// The score is calculated by averaging the online percentage in each window
// (not including the last).
func RecalculateScore(history *pb.AuditHistory) {
	if len(history.Windows) <= 1 {
		history.Score = 1
		return
	}
	totalWindowScores := float64(0)
	for i, window := range history.Windows {
		// do not include last window in score
		if i+1 == len(history.Windows) {
			break
		}
		totalWindowScores += float64(window.OnlineCount) / float64(window.TotalCount)
	}
	history.Score = totalWindowScores / float64(len(history.Windows)-1)
}
