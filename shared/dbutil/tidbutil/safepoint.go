// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package tidbutil provides utilities for working with TiDB clusters.
package tidbutil

import (
	"context"
	"strings"
	"sync"
	"time"

	pd "github.com/tikv/pd/client"
	"github.com/tikv/pd/client/clients/gc"
	"github.com/tikv/pd/client/constants"
	"github.com/tikv/pd/client/pkg/caller"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/common/uuid"
)

// Error is the error class for this package.
var Error = errs.Class("tidbutil")

// tsoPhysicalShiftBits is the number of bits reserved for the logical part of
// a TiDB TSO. A TSO is composed as physical(ms)<<18 | logical.
const tsoPhysicalShiftBits = 18

// acquireTimeout bounds taking the hold. Long enough to ride out a brief PD
// blip, short enough that an unreachable PD fails the run instead of leaving it
// retrying with nothing to show for it.
const acquireTimeout = time.Minute

// SafepointConfig configures pinning a TiKV GC safepoint for the duration of a
// consistent scan.
type SafepointConfig struct {
	// PDEndpoints is the comma-separated list of PD endpoints.
	PDEndpoints string
	// ServiceID is the identifier prefix for the GC barrier/safepoint
	// registered with PD; a unique per-run suffix is appended.
	ServiceID string
	// TTL is how long after the last successful heartbeat the safepoint
	// auto-expires on the PD side.
	TTL time.Duration
}

// pdClient is the subset of the PD client used by Holder.
type pdClient interface {
	GetTS(ctx context.Context) (int64, int64, error)
	GetGCStatesClient(keyspaceID uint32) gc.GCStatesClient
	UpdateServiceGCSafePoint(ctx context.Context, serviceID string, ttl int64, safePoint uint64) (uint64, error)
	Close()
}

// Holder holds a TiKV GC safepoint at a fixed timestamp so that reads
// AS OF that timestamp stay valid for the duration of a scan.
//
// It prefers the GC barrier API (PD >= v9) and falls back to the legacy
// service GC safepoint API. The hold is kept alive by periodic heartbeats;
// if heartbeats fail for long enough that expiry becomes imminent, the
// context returned by Context is cancelled so that the scan aborts before
// the snapshot can become invalid.
type Holder struct {
	log    *zap.Logger
	client pdClient

	serviceID string
	tso       uint64
	readTime  time.Time
	ttl       time.Duration

	// useLegacy selects the legacy service GC safepoint API over the GC
	// barrier API. It is set once SetGCBarrier reports Unimplemented, which
	// is the case on PD < v9 (e.g. the TiDB v8.5.x);
	// on those the legacy path is used for the entire run.
	useLegacy bool

	stop     context.CancelFunc
	stopped  sync.WaitGroup
	lostOnce sync.Once
	lost     chan struct{}
	lostErr  error

	// cancelClient releases the context the PD client was built on; nil when
	// the client is owned by the caller (tests).
	cancelClient context.CancelFunc
}

// Hold connects to PD, registers a GC safepoint at the current cluster
// timestamp and starts heartbeating it. Callers must call Release when the
// scan is done; if the process dies, the safepoint expires after TTL.
func Hold(ctx context.Context, log *zap.Logger, config SafepointConfig) (_ *Holder, err error) {
	if config.PDEndpoints == "" {
		return nil, Error.New("PD endpoints are not configured")
	}
	if config.TTL < time.Second {
		// The legacy service safepoint API takes the TTL in whole seconds, and
		// a sub-second value truncates to 0, which that API defines as
		// "delete": every heartbeat would silently remove the hold and the
		// scan would keep running unprotected. Refuse instead.
		return nil, Error.New("safepoint TTL must be at least 1s, got %s", config.TTL)
	}

	// The PD client's background loops -- including the safepoint heartbeats --
	// run off the context it is built with, so it has to outlive acquiring the
	// hold and is torn down by Release instead. Only acquisition is bounded.
	clientCtx, cancelClient := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancelClient()
		}
	}()

	acquireCtx, cancelAcquire := context.WithTimeout(ctx, acquireTimeout)
	defer cancelAcquire()

	client, err := connectPD(clientCtx, acquireCtx, config.PDEndpoints)
	if err != nil {
		return nil, err
	}

	// hold's heartbeat detaches from this context, so the deadline does not
	// follow the holder past acquisition.
	holder, err := hold(acquireCtx, log, client, config)
	if err != nil {
		client.Close()
		return nil, err
	}
	holder.cancelClient = cancelClient
	return holder, nil
}

// connectPD dials PD, giving up once acquireCtx expires. The client itself is
// built from clientCtx so that it outlives this call.
//
// The timeout is the whole point: the PD client retries discovery until its
// context ends and never returns an error of its own, so an unreachable PD
// would otherwise leave the caller blocked forever rather than failing.
func connectPD(clientCtx, acquireCtx context.Context, endpoints string) (pd.Client, error) {
	type connected struct {
		client pd.Client
		err    error
	}
	// buffered: on timeout nobody is left to receive, and the goroutine must
	// not leak once cancelClient unblocks it.
	done := make(chan connected, 1)
	go func() {
		client, err := pd.NewClientWithContext(clientCtx, caller.Component("storj/gc-safepoint"),
			strings.Split(endpoints, ","), pd.SecurityOption{})
		done <- connected{client, err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return nil, Error.New("connecting to PD: %w", result.err)
		}
		return result.client, nil
	case <-acquireCtx.Done():
		// The dial may still succeed after this timeout: cancelling clientCtx
		// stops the client's background loops, but its gRPC connections are
		// only released by Close. Reap it so a late success does not leak them.
		go func() {
			if result := <-done; result.err == nil {
				result.client.Close()
			}
		}()
		return nil, Error.New("connecting to PD at %q: %w", endpoints, acquireCtx.Err())
	}
}

// hold implements Hold on an already-connected client. Separated for testing.
func hold(ctx context.Context, log *zap.Logger, client pdClient, config SafepointConfig) (_ *Holder, err error) {
	// Use the cluster's TSO rather than the local clock: AS OF TIMESTAMP is
	// evaluated against cluster time and local skew could otherwise pick a
	// timestamp that is not protected by the safepoint. The logical part is
	// deliberately discarded, see below.
	physical, _, err := client.GetTS(ctx)
	if err != nil {
		return nil, Error.New("getting cluster timestamp: %w", err)
	}

	suffix, err := uuid.New()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	holder := &Holder{
		log:    log,
		client: client,
		// unique per run so that concurrent runs cannot clobber each other's
		// hold; abandoned entries are cleaned up by their TTL
		serviceID: config.ServiceID + "-" + suffix.String()[:8],
		// The barrier sits at the start of the millisecond, without the logical
		// part, because that is precisely where callers read: they scan with
		// AS OF TIMESTAMP ReadTime(), and TiDB resolves a datetime to
		// (unix_ms << 18) with the logical bits zeroed. Keeping the logical bits
		// here would leave the barrier a few ticks *above* the timestamp
		// actually read, and once the barrier becomes the binding GC minimum --
		// which is what happens as soon as a scan outlives gc_life_time, the
		// case this exists for -- the scan's own read is rejected for being
		// older than the GC safe point. Rounding down protects a fraction of a
		// millisecond more history, which is the harmless direction.
		tso:      uint64(physical) << tsoPhysicalShiftBits,
		readTime: time.UnixMilli(physical).UTC(),
		ttl:      config.TTL,
		lost:     make(chan struct{}),
	}

	if err := holder.set(ctx); err != nil {
		return nil, err
	}

	heartbeatCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	holder.stop = stop
	holder.stopped.Add(1)
	go holder.heartbeat(heartbeatCtx)

	return holder, nil
}

// ReadTime returns the timestamp protected by the safepoint. Reads
// AS OF this timestamp are guaranteed to stay valid while the hold is alive.
func (holder *Holder) ReadTime() time.Time { return holder.readTime }

// ServiceID returns the unique identifier registered with PD.
func (holder *Holder) ServiceID() string { return holder.serviceID }

// Context returns a context that is cancelled when the hold is lost or the
// parent is done. Run the scan under this context.
func (holder *Holder) Context(parent context.Context) context.Context {
	ctx, cancel := context.WithCancelCause(parent)
	go func() {
		select {
		case <-holder.lost:
			cancel(Error.New("GC safepoint hold lost: %w", holder.lostErr))
		case <-ctx.Done():
		}
	}()
	return ctx
}

// Release stops heartbeating and removes the safepoint from PD. The TTL
// remains the backstop if removal fails.
func (holder *Holder) Release(ctx context.Context) (err error) {
	holder.stop()
	holder.stopped.Wait()

	defer func() {
		holder.client.Close()
		if holder.cancelClient != nil {
			holder.cancelClient()
		}
	}()

	if holder.useLegacy {
		// a non-positive TTL removes the service safepoint
		_, err = holder.client.UpdateServiceGCSafePoint(ctx, holder.serviceID, 0, holder.tso)
	} else {
		_, err = holder.client.GetGCStatesClient(constants.NullKeyspaceID).DeleteGCBarrier(ctx, holder.serviceID)
	}
	if err != nil {
		holder.log.Warn("failed to remove GC safepoint; it will expire via TTL",
			zap.String("service_id", holder.serviceID),
			zap.Duration("ttl", holder.ttl),
			zap.Error(err))
		return Error.New("removing GC safepoint: %w", err)
	}
	return nil
}

// set registers or refreshes the safepoint, preferring the GC barrier API and
// falling back to the legacy service GC safepoint API when unavailable.
//
// Only the legacy path has been exercised against a real PD: v8.5.x does not
// implement SetGCBarrier at all, so every deployment we currently run falls back
// here. The barrier path is covered by unit tests against a fake PD only. Note
// the two are not equivalent even in principle: a barrier bounds PD's txn safe
// point, whereas the legacy call bounds the GC safe point directly, so a PD new
// enough to serve barriers deserves its own integration test rather than an
// assumption that this keeps working.
func (holder *Holder) set(ctx context.Context) error {
	if !holder.useLegacy {
		_, err := holder.client.GetGCStatesClient(constants.NullKeyspaceID).SetGCBarrier(ctx, holder.serviceID, holder.tso, holder.ttl)
		if err == nil {
			return nil
		}
		if status.Code(err) != codes.Unimplemented {
			return Error.New("setting GC barrier: %w", err)
		}
		holder.log.Info("PD GC barrier API unavailable; falling back to service GC safepoint", zap.Error(err))
		holder.useLegacy = true
	}

	minSafepoint, err := holder.client.UpdateServiceGCSafePoint(ctx, holder.serviceID, int64(holder.ttl.Seconds()), holder.tso)
	if err != nil {
		return Error.New("updating service GC safepoint: %w", err)
	}
	if minSafepoint > holder.tso {
		// the cluster GC safepoint is already past our timestamp; a scan at
		// this timestamp could read a partially garbage-collected snapshot
		return Error.New("cluster GC safepoint %d is already ahead of requested safepoint %d", minSafepoint, holder.tso)
	}
	return nil
}

// heartbeat keeps the safepoint alive and marks the hold lost when expiry
// becomes imminent.
func (holder *Holder) heartbeat(ctx context.Context) {
	defer holder.stopped.Done()

	refresh := holder.ttl / 3
	// Abort the scan at the second consecutive failed heartbeat, while there
	// is still around a refresh interval left before PD may expire the hold,
	// leaving room for the scan to unwind. The threshold sits halfway between
	// the first and second failed ticks (~refresh and ~2*refresh after the
	// last success) because comparing against 2*refresh itself is jitter-prone:
	// the check runs after set() returns, so it can land marginally on either
	// side, and missing it would defer the abort to ~3*refresh -- exactly when
	// PD expires the hold, giving the scan no time to stop reading.
	margin := holder.ttl / 2

	lastSuccess := time.Now()
	ticker := time.NewTicker(refresh)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		err := func() error {
			ctx, cancel := context.WithTimeout(ctx, refresh/2)
			defer cancel()
			return holder.set(ctx)
		}()
		if err == nil {
			lastSuccess = time.Now()
			continue
		}
		if ctx.Err() != nil {
			return
		}

		holder.log.Warn("GC safepoint heartbeat failed",
			zap.String("service_id", holder.serviceID),
			zap.Duration("since_last_success", time.Since(lastSuccess)),
			zap.Error(err))

		if time.Since(lastSuccess) > margin {
			holder.markLost(Error.New("heartbeats failing for %s of %s TTL: %w", time.Since(lastSuccess), holder.ttl, err))
			return
		}
	}
}

func (holder *Holder) markLost(err error) {
	holder.lostOnce.Do(func() {
		holder.lostErr = err
		close(holder.lost)
	})
}
