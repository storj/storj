// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/satellites"
)

// Error is the default error class.
var (
	Error        = errs.Class("trust")
	ErrUntrusted = Error.New("satellite is untrusted")

	mon = monkit.Package()
)

// IdentityResolver resolves peer identities from a node URL.
type IdentityResolver interface {
	// ResolveIdentity returns the peer identity of the peer located at the Node URL
	ResolveIdentity(ctx context.Context, url storj.NodeURL) (*identity.PeerIdentity, error)
}

// IdentityResolverFunc is a convenience type for implementing IdentityResolver using a
// function literal.
type IdentityResolverFunc func(ctx context.Context, url storj.NodeURL) (*identity.PeerIdentity, error)

// ResolveIdentity returns the peer identity of the peer located at the Node URL.
func (fn IdentityResolverFunc) ResolveIdentity(ctx context.Context, url storj.NodeURL) (*identity.PeerIdentity, error) {
	return fn(ctx, url)
}

// Dialer implements an IdentityResolver using an RPC dialer.
func Dialer(dialer rpc.Dialer) IdentityResolver {
	return IdentityResolverFunc(func(ctx context.Context, url storj.NodeURL) (_ *identity.PeerIdentity, err error) {
		defer mon.Task()(&ctx)(&err)

		conn, err := dialer.DialNodeURL(rpcpool.WithForceDial(ctx), url)
		if err != nil {
			return nil, err
		}
		defer func() { err = errs.Combine(err, conn.Close()) }()
		return conn.PeerIdentity()
	})
}

// Pool implements different peer verifications.
//
// architecture: Service
type Pool struct {
	log             *zap.Logger
	resolver        IdentityResolver
	refreshInterval time.Duration

	listMu sync.Mutex
	list   *List

	satellitesDB satellites.DB

	satellitesMu sync.RWMutex
	satellites   map[storj.NodeID]*satelliteInfoCache

	// set it to true, to refresh trust pool at the very beginning of the run loop.
	StartWithRefresh bool
}

// satelliteInfoCache caches identity information about a satellite.
type satelliteInfoCache struct {
	mu       sync.Mutex
	url      storj.NodeURL
	identity *identity.PeerIdentity
}

// NewPool creates a new trust pool of the specified list of trusted satellites.
func NewPool(log *zap.Logger, resolver IdentityResolver, config Config, satellitesDB satellites.DB) (*Pool, error) {
	// TODO: preload all satellite peer identities

	cache, err := LoadCache(config.CachePath)
	if err != nil {
		return nil, err
	}

	list, err := NewList(log, config.Sources, config.Exclusions.Rules, cache)
	if err != nil {
		return nil, err
	}

	return &Pool{
		log:             log,
		resolver:        resolver,
		refreshInterval: config.RefreshInterval,
		list:            list,
		satellitesDB:    satellitesDB,
		satellites:      make(map[storj.NodeID]*satelliteInfoCache),
	}, nil
}

// Run periodically refreshes the pool. The initial refresh is intended to
// happen before run is call. Therefore Run does not refresh right away.
func (pool *Pool) Run(ctx context.Context) error {
	if pool.StartWithRefresh {
		if err := pool.Refresh(ctx); err != nil {
			pool.log.Error("Failed to refresh", zap.Error(err))
			return err
		}
	}

	for {
		refreshAfter := jitter(pool.refreshInterval)
		pool.log.Info("Scheduling next refresh", zap.Duration("after", refreshAfter))
		if !sync2.Sleep(ctx, refreshAfter) {
			return ctx.Err()
		}
		if err := pool.Refresh(ctx); err != nil {
			pool.log.Error("Failed to refresh", zap.Error(err))
			return err
		}
	}
}

var monVerifySatelliteID = mon.Task()

// VerifySatelliteID checks whether id corresponds to a trusted satellite.
func (pool *Pool) VerifySatelliteID(ctx context.Context, id storj.NodeID) (err error) {
	defer monVerifySatelliteID(&ctx)(&err)

	_, err = pool.getInfo(id)
	return err
}

var monGetSignee = mon.Task()

// GetSignee gets the corresponding signee for verifying signatures.
// It ignores passed in ctx cancellation to avoid miscaching between concurrent requests.
func (pool *Pool) GetSignee(ctx context.Context, id storj.NodeID) (_ signing.Signee, err error) {
	defer monGetSignee(&ctx)(&err)

	info, err := pool.getInfo(id)
	if err != nil {
		return nil, err
	}

	info.mu.Lock()
	defer info.mu.Unlock()

	if info.identity == nil {
		identity, err := pool.resolver.ResolveIdentity(ctx, info.url)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		info.identity = identity
	}

	return signing.SigneeFromPeerIdentity(info.identity), nil
}

// GetSatellites returns a slice containing all trusted satellites.
func (pool *Pool) GetSatellites(ctx context.Context) (satellites []storj.NodeID) {
	defer mon.Task()(&ctx)(nil)

	pool.satellitesMu.RLock()
	for sat := range pool.satellites {
		satellites = append(satellites, sat)
	}
	pool.satellitesMu.RUnlock()

	sort.Sort(storj.NodeIDList(satellites))
	return satellites
}

// GetNodeURL returns the node url of a satellite in the trusted list.
func (pool *Pool) GetNodeURL(ctx context.Context, id storj.NodeID) (_ storj.NodeURL, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := pool.getInfo(id)
	if err != nil {
		return storj.NodeURL{}, err
	}
	return info.url, nil
}

// IsTrusted returns true if the satellite is trusted.
func (pool *Pool) IsTrusted(ctx context.Context, id storj.NodeID) bool {
	defer mon.Task()(&ctx)(nil)

	_, err := pool.getInfo(id)
	return err == nil
}

// Refresh refreshes the set of trusted satellites in the pool. Concurrent
// callers will be synchronized so only one proceeds at a time.
func (pool *Pool) Refresh(ctx context.Context) error {
	urls, err := pool.fetchURLs(ctx)
	if err != nil {
		return err
	}

	pool.satellitesMu.Lock()
	defer pool.satellitesMu.Unlock()

	// add/update trusted IDs
	trustedIDs := make(map[storj.NodeID]struct{})
	for _, url := range urls {
		trustedIDs[url.ID] = struct{}{}

		info, ok := pool.satellites[url.ID]
		if !ok {
			info = &satelliteInfoCache{
				url: url,
			}
			pool.log.Debug("Satellite is trusted", zap.String("id", url.ID.String()))
			pool.satellites[url.ID] = info
		}

		// update the URL address and reset the identity if it changed
		if info.url.Address != url.Address {
			pool.log.Debug("Satellite address updated; identity cache purged",
				zap.String("id", url.ID.String()),
				zap.String("old", info.url.Address),
				zap.String("new", url.Address),
			)
			info.url.Address = url.Address
			info.identity = nil
		}
	}

	// remove trusted IDs that are no longer in the URL list
	for id, info := range pool.satellites {
		if _, ok := trustedIDs[id]; !ok {
			pool.log.Debug("Satellite is no longer trusted", zap.String("id", id.String()))
			delete(pool.satellites, id)
			err := pool.satellitesDB.UpdateSatelliteStatus(ctx, id, satellites.Untrusted)
			if err != nil {
				return err
			}

			continue
		}

		// for cases where a satellite was previously marked as untrusted, but is now trusted
		// we reset the status back to normal
		status := satellites.Normal
		dbSatellite, err := pool.satellitesDB.GetSatellite(ctx, info.url.ID)
		if err == nil && !dbSatellite.SatelliteID.IsZero() {
			if dbSatellite.Status != satellites.Untrusted {
				status = dbSatellite.Status
			}
		}
		if err := pool.satellitesDB.SetAddressAndStatus(ctx, info.url.ID, info.url.Address, status); err != nil {
			return err
		}
	}

	return nil
}

// DeleteSatellite deletes a satellite from the pool and marks it as untrusted in the database.
func (pool *Pool) DeleteSatellite(ctx context.Context, id storj.NodeID) error {
	pool.satellitesMu.Lock()
	defer pool.satellitesMu.Unlock()

	if _, ok := pool.satellites[id]; !ok {
		// satellite is already removed from the trust cache
		return nil
	}

	// remove the satellite from the pool cache
	delete(pool.satellites, id)

	// remove the satellite from the trust cache
	err := pool.deleteSatelliteFromCache(ctx, id)
	if err != nil {
		return err
	}

	return pool.satellitesDB.UpdateSatelliteStatus(ctx, id, satellites.Untrusted)
}

// deleteSatelliteFromCache removes a satellite from the trust cache and saves the cache.
func (pool *Pool) deleteSatelliteFromCache(ctx context.Context, id storj.NodeID) error {
	pool.listMu.Lock()
	defer pool.listMu.Unlock()

	if !pool.list.cache.DeleteSatelliteEntry(id) {
		// satellite is already not in the cache
		return nil
	}

	return pool.list.saveCache(ctx)
}

func (pool *Pool) getInfo(id storj.NodeID) (*satelliteInfoCache, error) {
	pool.satellitesMu.RLock()
	defer pool.satellitesMu.RUnlock()

	info, ok := pool.satellites[id]
	if !ok {
		return nil, ErrUntrusted
	}
	return info, nil
}

func (pool *Pool) fetchURLs(ctx context.Context) ([]storj.NodeURL, error) {
	// Typically there will only be one caller of refresh (i.e. Run()) but
	// if at some point we might want  on-demand refresh, and *List is designed
	// to be used by a single goroutine (don't want multiple callers racing
	// on the cache, etc).
	pool.listMu.Lock()
	defer pool.listMu.Unlock()
	return pool.list.FetchURLs(ctx)
}

func jitter(t time.Duration) time.Duration {
	nanos := rand.NormFloat64()*float64(t/4) + float64(t)
	if nanos <= 0 {
		nanos = 1
	}
	return time.Duration(nanos)
}
