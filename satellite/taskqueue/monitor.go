// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package taskqueue

import (
	"context"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// MonitorConfig configures the task queue monitor.
type MonitorConfig struct {
	Interval time.Duration `help:"how frequently to check task queue stream lengths" releaseDefault:"5m" devDefault:"1m" testDefault:"$TESTINTERVAL"`
}

// Monitor periodically checks all Redis streams and exposes their lengths as monkit metrics.
type Monitor struct {
	log    *zap.Logger
	client *Client
	Loop   *sync2.Cycle

	mu     sync.Mutex
	stats  map[string]int64
	update time.Time
}

var _ monkit.StatSource = &Monitor{}

// NewMonitor creates a new task queue monitor.
func NewMonitor(log *zap.Logger, client *Client, config MonitorConfig) *Monitor {
	m := &Monitor{
		log:    log,
		client: client,
		Loop:   sync2.NewCycle(config.Interval),
	}
	mon.Chain(m)
	return m
}

// Run starts the periodic monitoring loop.
func (m *Monitor) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return m.Loop.Run(ctx, func(ctx context.Context) error {
		m.RunOnce(ctx)
		return nil
	})
}

// RunOnce refreshes the stream length statistics.
func (m *Monitor) RunOnce(ctx context.Context) {
	streams, err := m.client.streamLengths(ctx)
	if err != nil {
		m.log.Error("couldn't get task queue stream lengths", zap.Error(err))
		return
	}

	m.mu.Lock()
	m.stats = streams
	m.update = time.Now()
	m.mu.Unlock()
}

// Stats implements monkit.StatSource.
func (m *Monitor) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.update.IsZero() || time.Since(m.update) > 24*time.Hour {
		return
	}

	for stream, length := range m.stats {
		key := monkit.NewSeriesKey("taskqueue").
			WithTag("stream", stream)
		cb(key, "length", float64(length))
	}
}
