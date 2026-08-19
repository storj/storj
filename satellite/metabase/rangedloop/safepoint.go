// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/tidbutil"
)

// Safepoint is a pinned TiKV GC safepoint on every cluster backing the
// metabase, so that a scan reads one consistent snapshot. Garbage collection
// depends on this for correctness: without it, a server-side copy interleaved
// with the scan can hide live pieces from the bloom filters.
//
// A Safepoint for a configuration that is not enabled holds nothing and every
// method is a no-op, so callers do not need to special case it.
type Safepoint struct {
	config SafepointConfig
	holds  tidbutil.Holds
	// acquireCtx is the context the holds were acquired with, kept so that
	// releasing them stays attributable to the run.
	acquireCtx context.Context
}

// HoldSafepoint pins a GC safepoint on every cluster backing db and starts
// heartbeating them. The caller runs the scan under Context and must Close the
// result; if the process dies, the safepoints expire after the configured TTL.
func HoldSafepoint(ctx context.Context, log *zap.Logger, db *metabase.DB, config SafepointConfig) (*Safepoint, error) {
	safepoint := &Safepoint{config: config, acquireCtx: ctx}
	if !config.Enabled() {
		return safepoint, nil
	}

	// Every cluster backing the metabase needs its own hold: an unpinned one is
	// scanned without the snapshot the run promises. The clusters are isolated,
	// each with its own PD, so the PD endpoints are labeled with the metabase
	// backend they serve and every backend must be there.
	impls := db.Implementations()
	for label, impl := range impls {
		if impl != dbutil.TiDB {
			return nil, errs.New("safepoint is not supported on %v (metabase backend %q)", impl, label)
		}
	}

	holds, err := tidbutil.Hold(ctx, log.Named("safepoint"), tidbutil.SafepointConfig{
		PDEndpoints: config.PDEndpoints,
		ServiceID:   config.ServiceID,
		TTL:         config.TTL,
		CAPath:      config.CACertPath,
		CertPath:    config.CertPath,
		KeyPath:     config.KeyPath,
	})
	if err != nil {
		return nil, errs.New("Error acquiring GC safepoint: %+v", err)
	}
	safepoint.holds = holds

	for label := range impls {
		if _, ok := holds[label]; !ok {
			return nil, errs.Combine(errs.New("no safepoint PD endpoints configured for metabase backend %q", label), safepoint.Close())
		}
	}
	for label := range holds {
		if _, ok := impls[label]; !ok {
			return nil, errs.Combine(errs.New("safepoint PD endpoints configured for %q, which is not a metabase backend", label), safepoint.Close())
		}
	}

	log.Info("holding TiKV GC safepoints for the scan",
		zap.Time("read_timestamp", holds.ReadTime()),
		zap.Strings("service_ids", holds.ServiceIDs()),
		zap.Duration("ttl", config.TTL))

	return safepoint, nil
}

// ReadTime returns the timestamp the pinned snapshot is read at, or the zero
// time when no safepoint is held.
func (safepoint *Safepoint) ReadTime() time.Time {
	if safepoint.holds == nil {
		return time.Time{}
	}
	return safepoint.holds.ReadTime()
}

// Context returns a context for the scan, which is cancelled when a hold is
// lost or the scan has been running longer than MaxDuration.
func (safepoint *Safepoint) Context(ctx context.Context) (context.Context, context.CancelFunc) {
	if safepoint.holds == nil {
		return ctx, func() {}
	}
	ctx = safepoint.holds.Context(ctx)
	if safepoint.config.MaxDuration > 0 {
		return context.WithTimeoutCause(ctx, safepoint.config.MaxDuration,
			errs.New("scan exceeded safepoint max duration %s", safepoint.config.MaxDuration))
	}
	return ctx, func() {}
}

// Close stops heartbeating and removes the safepoints from PD.
func (safepoint *Safepoint) Close() error {
	if safepoint.holds == nil {
		return nil
	}
	holds := safepoint.holds
	safepoint.holds = nil

	// release with a fresh deadline even when the scan context is already
	// cancelled, but keep the run's context values so a failure is still
	// attributable to it; the TTL remains the backstop if this fails
	ctx, cancel := context.WithTimeout(context.WithoutCancel(safepoint.acquireCtx), 30*time.Second)
	defer cancel()
	return holds.Release(ctx)
}
