// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package tidbutil provides utilities for working with TiDB clusters.
package tidbutil

import (
	"context"
	"slices"
	"sort"
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

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
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

// releaseTimeout bounds removing the holds again when acquisition fails partway
// through. Whatever is not removed expires with its TTL.
const releaseTimeout = 30 * time.Second

// SafepointConfig configures pinning a TiKV GC safepoint for the duration of a
// consistent scan.
type SafepointConfig struct {
	// PDEndpoints lists the PD endpoints, comma-separated within a cluster and
	// semicolon-separated between clusters. A metabase spread over several
	// isolated TiDB clusters has one PD per cluster, and every one of them has
	// to be pinned.
	//
	// Each entry is labeled with the metabase backend it belongs to, in the
	// same "label=" form the metabase connection string uses:
	//
	//	west=pd-w1:2379,pd-w2:2379;east=pd-e1:2379
	//
	// Without a label an entry takes its position as one, which is also the
	// default label of a backend, so a single unlabeled list keeps working.
	PDEndpoints string
	// ServiceID is the identifier prefix for the GC barrier/safepoint
	// registered with PD; a unique per-run suffix is appended.
	ServiceID string
	// TTL is how long after the last successful heartbeat the safepoint
	// auto-expires on the PD side.
	TTL time.Duration

	// CAPath, CertPath and KeyPath enable mTLS to PD. All three are required
	// on a cluster deployed with TLS between components ("tiup cluster tls
	// enable"): PD then advertises https client URLs and rejects plaintext
	// connections, so a client built without them cannot connect at all.
	//
	// Isolated clusters may each have their own certificate authority, so each
	// of the three may also be a labeled, semicolon-separated list in the same
	// form as PDEndpoints, naming exactly the same backends:
	//
	//	west=/certs/west/ca.crt;east=/certs/east/ca.crt
	//
	// A plain path is shared by every cluster, which is the usual case of one
	// deployment-wide CA. The three are resolved independently, so a shared CA
	// combines with per-cluster client certificates.
	CAPath   string
	CertPath string
	KeyPath  string
}

// securityOptions resolves the TLS paths into one PD security option per
// cluster, keyed by backend label.
func (config SafepointConfig) securityOptions(labels []string) (map[string]pd.SecurityOption, error) {
	ca, err := resolvePaths("ca-cert-path", config.CAPath, labels)
	if err != nil {
		return nil, err
	}
	cert, err := resolvePaths("cert-path", config.CertPath, labels)
	if err != nil {
		return nil, err
	}
	key, err := resolvePaths("key-path", config.KeyPath, labels)
	if err != nil {
		return nil, err
	}

	options := make(map[string]pd.SecurityOption, len(labels))
	for _, label := range labels {
		option := pd.SecurityOption{
			CAPath:   ca[label],
			CertPath: cert[label],
			KeyPath:  key[label],
		}
		if err := validateTLS(option); err != nil {
			return nil, Error.New("backend %q: %w", label, err)
		}
		options[label] = option
	}
	return options, nil
}

// resolvePaths spreads one TLS path setting over the clusters.
//
// A plain path -- no label, no separator -- belongs to all of them. Anything
// else is a labeled list that has to name the backends exactly: a cluster
// missing from it would be dialed without the material the setting was meant
// to give it, in plaintext, which is the failure validateTLS exists to keep
// from happening silently.
func resolvePaths(name, value string, labels []string) (map[string]string, error) {
	paths := make(map[string]string, len(labels))

	value = strings.TrimSpace(value)
	if value == "" {
		return paths, nil
	}
	if !strings.ContainsAny(value, "=;") {
		for _, label := range labels {
			paths[label] = value
		}
		return paths, nil
	}

	entries, err := dbutil.SplitLabeled(value, ";")
	if err != nil {
		return nil, Error.New("parsing safepoint %s: %w", name, err)
	}
	for _, entry := range entries {
		if !slices.Contains(labels, entry.Label) {
			return nil, Error.New("safepoint %s names %q, which is not a configured backend", name, entry.Label)
		}
		paths[entry.Label] = entry.Value
	}
	for _, label := range labels {
		if _, ok := paths[label]; !ok {
			return nil, Error.New("safepoint %s has no entry for backend %q", name, label)
		}
	}
	return paths, nil
}

// validateTLS rejects a partially configured TLS setup.
//
// The PD client decides whether to use TLS from the certificate and key alone:
// with both empty its TLS config is nil, it rewrites every endpoint to http and
// ignores CAPath entirely. A CA-only configuration would therefore look
// configured while connecting in plaintext, which against a TLS-enabled PD
// surfaces only as an opaque connect timeout. Refuse it up front instead.
func validateTLS(security pd.SecurityOption) error {
	switch {
	case security.CertPath != "" && security.KeyPath != "":
		if security.CAPath == "" {
			return Error.New("safepoint TLS requires a CA path alongside the client certificate and key")
		}
	case security.CertPath != "" || security.KeyPath != "":
		return Error.New("safepoint TLS requires both a client certificate and key (cert: %q, key: %q)",
			security.CertPath, security.KeyPath)
	case security.CAPath != "":
		return Error.New("safepoint CA path %q is set without a client certificate and key; "+
			"the PD client ignores the CA in that case and connects in plaintext", security.CAPath)
	}
	return nil
}

// pdClient is the subset of the PD client used by Holder.
type pdClient interface {
	GetTS(ctx context.Context) (int64, int64, error)
	GetClusterID(ctx context.Context) uint64
	GetGCStatesClient(keyspaceID uint32) gc.GCStatesClient
	UpdateServiceGCSafePoint(ctx context.Context, serviceID string, ttl int64, safePoint uint64) (uint64, error)
	Close()
}

// Holder holds a TiKV GC safepoint on one cluster at a fixed timestamp so that
// reads AS OF that timestamp stay valid for the duration of a scan. A metabase
// spanning several clusters needs one per cluster, see Holds.
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

// Holds is a hold on every configured cluster, keyed by the label of the
// metabase backend it serves. A metabase may be spread over several isolated
// TiDB clusters, each with its own PD, and a scan is only consistent if all of
// them are pinned.
type Holds map[string]*Holder

// Hold connects to every configured PD, registers a GC safepoint at each
// cluster's current timestamp and starts heartbeating them. Callers must call
// Release when the scan is done; if the process dies, the safepoints expire
// after TTL.
func Hold(ctx context.Context, log *zap.Logger, config SafepointConfig) (_ Holds, err error) {
	clusters, err := dbutil.SplitLabeled(config.PDEndpoints, ";")
	if err != nil {
		return nil, Error.New("parsing PD endpoints: %w", err)
	}
	if len(clusters) == 0 {
		return nil, Error.New("PD endpoints are not configured")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	labels := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		labels = append(labels, cluster.Label)
	}
	security, err := config.securityOptions(labels)
	if err != nil {
		return nil, err
	}

	holds := Holds{}
	for _, cluster := range clusters {
		log := log.With(zap.String("backend", cluster.Label), zap.String("pd_endpoints", cluster.Value))
		holder, err := holdCluster(ctx, log, config, cluster.Value, security[cluster.Label])
		if err != nil {
			return nil, errs.Combine(err, holds.release(ctx))
		}
		holds[cluster.Label] = holder
	}

	if err := holds.verifyDistinctClusters(ctx); err != nil {
		return nil, errs.Combine(err, holds.release(ctx))
	}
	if err := holds.alignReadTime(ctx); err != nil {
		return nil, errs.Combine(err, holds.release(ctx))
	}
	return holds, nil
}

// verifyDistinctClusters rejects two labels pointing at the same cluster.
//
// Every other check passes for "west=pd-w1:2379;east=pd-w1:2379" -- the labels
// differ, and they match the metabase backends exactly -- while it takes two
// barriers on west and leaves east unpinned, so the scan reads east live. That
// is the data loss the safepoint exists to prevent, and without this it is
// silent.
func (holds Holds) verifyDistinctClusters(ctx context.Context) error {
	byCluster := make(map[uint64]string, len(holds))
	for label, holder := range holds {
		clusterID := holder.client.GetClusterID(ctx)
		if other, ok := byCluster[clusterID]; ok {
			return Error.New("backends %q and %q point at the same TiDB cluster %d, so one of them is left unpinned",
				min(label, other), max(label, other), clusterID)
		}
		byCluster[clusterID] = label
	}
	return nil
}

// ReadTime returns the timestamp to scan at: the newest of the per-cluster
// timestamps. Every barrier sits at or below it, so a read at this timestamp
// is protected on all of the clusters.
func (holds Holds) ReadTime() time.Time {
	var newest time.Time
	for _, holder := range holds {
		if holder.readTime.After(newest) {
			newest = holder.readTime
		}
	}
	return newest
}

// ServiceIDs returns the identifier registered with each PD as
// "backend=service-id", sorted by backend.
func (holds Holds) ServiceIDs() []string {
	ids := make([]string, 0, len(holds))
	for label, holder := range holds {
		ids = append(ids, label+"="+holder.serviceID)
	}
	sort.Strings(ids)
	return ids
}

// Context returns a context that is cancelled when any of the holds is lost or
// the parent is done. Run the scan under this context.
func (holds Holds) Context(parent context.Context) context.Context {
	for _, holder := range holds {
		parent = holder.Context(parent)
	}
	return parent
}

// Release stops heartbeating and removes the safepoints from PD. The TTL
// remains the backstop for the ones that fail.
func (holds Holds) Release(ctx context.Context) error {
	var group errs.Group
	for _, holder := range holds {
		group.Add(holder.Release(ctx))
	}
	return group.Err()
}

// release tears down the holds taken so far after acquisition failed, where
// ctx may itself be the reason it failed.
func (holds Holds) release(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()
	return holds.Release(ctx)
}

// alignReadTime waits until every cluster's clock has reached ReadTime.
//
// The scan reads at the newest of the per-cluster timestamps so that it is at
// or above every barrier, and therefore protected everywhere. On a cluster
// whose PD clock lags that timestamp is momentarily in the future, and TiDB
// rejects a stale read of the future outright, so let the laggards catch up
// here instead of failing the first query of the scan.
func (holds Holds) alignReadTime(ctx context.Context) error {
	if len(holds) < 2 {
		// the only timestamp there is came from this cluster's own clock
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, acquireTimeout)
	defer cancel()

	readTime := holds.ReadTime()
	for label, holder := range holds {
		for {
			physical, _, err := holder.client.GetTS(ctx)
			if err != nil {
				return Error.New("getting cluster timestamp: %w", err)
			}
			behind := readTime.Sub(time.UnixMilli(physical))
			if behind <= 0 {
				break
			}
			holder.log.Info("waiting for the PD clock to reach the read timestamp",
				zap.Time("read_timestamp", readTime), zap.Duration("behind", behind))

			if !sync2.Sleep(ctx, min(behind, time.Second)) {
				return Error.New("waiting for the PD clock of backend %q to reach %s: %w",
					label, readTime, ctx.Err())
			}
		}
	}
	return nil
}

// validate checks the settings that are independent of the cluster.
func (config SafepointConfig) validate() error {
	if config.TTL < time.Second {
		// The legacy service safepoint API takes the TTL in whole seconds, and
		// a sub-second value truncates to 0, which that API defines as
		// "delete": every heartbeat would silently remove the hold and the
		// scan would keep running unprotected. Refuse instead.
		return Error.New("safepoint TTL must be at least 1s, got %s", config.TTL)
	}
	return nil
}

// holdCluster takes the hold on a single cluster.
func holdCluster(ctx context.Context, log *zap.Logger, config SafepointConfig, endpoints string, security pd.SecurityOption) (_ *Holder, err error) {
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

	client, err := connectPD(clientCtx, acquireCtx, security, endpoints)
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
func connectPD(clientCtx, acquireCtx context.Context, security pd.SecurityOption, endpoints string) (pd.Client, error) {
	type connected struct {
		client pd.Client
		err    error
	}
	// buffered: on timeout nobody is left to receive, and the goroutine must
	// not leak once cancelClient unblocks it.
	done := make(chan connected, 1)
	go func() {
		client, err := pd.NewClientWithContext(clientCtx, caller.Component("storj/gc-safepoint"),
			splitEndpoints(endpoints, ","), security)
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

// splitEndpoints splits an endpoint list on sep, tolerating the spaces that a
// list written by hand in a config file tends to pick up. The PD client would
// otherwise treat " 10.0.0.2:2389" as a hostname and never reach the second
// member.
func splitEndpoints(endpoints, sep string) []string {
	parts := strings.Split(endpoints, sep)
	trimmed := parts[:0]
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			trimmed = append(trimmed, part)
		}
	}
	return trimmed
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
