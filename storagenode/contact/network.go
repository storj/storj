// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"sync"
	"time"
)

const (
	// NetworkStatusOk represents node successfully pinged.
	NetworkStatusOk = "OK"
	// NetworkStatusMisconfigured means satellite could not ping
	//  back node due to misconfiguration on the node host.
	NetworkStatusMisconfigured = "Misconfigured"
	// NetworkStatusDisabled means QUIC is disabled by config.
	NetworkStatusDisabled = "Disabled"
	// NetworkStatusRefreshing means QUIC check is in progress.
	NetworkStatusRefreshing = "Refreshing"
)

// QUICStats contains information regarding QUIC status of the node.
type QUICStats struct {
	status  string
	enabled bool

	mu         sync.Mutex
	lastPinged time.Time
}

// NewQUICStats returns a new QUICStats.
func NewQUICStats(enabled bool) *QUICStats {
	stats := &QUICStats{
		enabled: enabled,
		status:  NetworkStatusRefreshing,
	}

	if !enabled {
		stats.status = NetworkStatusDisabled
	}
	return stats
}

// SetStatus sets the QUIC status during PingMe request.
func (q *QUICStats) SetStatus(pingSuccess bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.lastPinged = time.Now()
	if pingSuccess {
		q.status = NetworkStatusOk
		return
	}

	q.status = NetworkStatusMisconfigured
}

// Status returns the quic status gathered in a PingMe request.
func (q *QUICStats) Status() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.enabled {
		return NetworkStatusDisabled
	}
	return q.status
}

// WhenLastPinged returns last time someone pinged this node via QUIC.
func (q *QUICStats) WhenLastPinged() (when time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lastPinged
}
