// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// Error is the default error class
var Error = errs.Class("trust:")

var mon = monkit.Package()

// Pool implements different peer verifications.
type Pool struct {
	mu        sync.RWMutex
	transport transport.Client

	trustedSatellites map[storj.NodeID]*satelliteInfoCache
}

// satelliteInfoCache caches identity information about a satellite
type satelliteInfoCache struct {
	mu       sync.Mutex
	url      storj.NodeURL
	identity *identity.PeerIdentity
}

// NewPool creates a new trust pool of the specified list of trusted satellites.
func NewPool(transport transport.Client, trustedSatellites storj.NodeURLs) (*Pool, error) {
	// TODO: preload all satellite peer identities

	// parse the comma separated list of approved satellite IDs into an array of storj.NodeIDs
	trusted := make(map[storj.NodeID]*satelliteInfoCache)

	for _, node := range trustedSatellites {
		trusted[node.ID] = &satelliteInfoCache{url: node}
	}

	return &Pool{
		transport:         transport,
		trustedSatellites: trusted,
	}, nil
}

// VerifySatelliteID checks whether id corresponds to a trusted satellite.
func (pool *Pool) VerifySatelliteID(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	_, ok := pool.trustedSatellites[id]
	if !ok {
		return Error.New("satellite %q is untrusted", id)
	}
	return nil
}

// GetSignee gets the corresponding signee for verifying signatures.
// It ignores passed in ctx cancellation to avoid miscaching between concurrent requests.
func (pool *Pool) GetSignee(ctx context.Context, id storj.NodeID) (_ signing.Signee, err error) {
	defer mon.Task()(&ctx)(&err)

	// lookup peer identity with id
	pool.mu.RLock()
	info, ok := pool.trustedSatellites[id]
	pool.mu.RUnlock()

	if !ok {
		return nil, Error.New("signee %q is untrusted", id)
	}

	info.mu.Lock()
	defer info.mu.Unlock()

	if info.identity == nil {
		identity, err := pool.FetchPeerIdentity(ctx, info.url)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		info.identity = identity
	}

	return signing.SigneeFromPeerIdentity(info.identity), nil
}

// FetchPeerIdentity dials the url and fetches the identity.
func (pool *Pool) FetchPeerIdentity(ctx context.Context, url storj.NodeURL) (_ *identity.PeerIdentity, err error) {
	identity, err := pool.transport.FetchPeerIdentity(ctx, &pb.Node{
		Id: url.ID,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   url.Address,
		},
	})
	return identity, Error.Wrap(err)
}

// GetSatellites returns a slice containing all trusted satellites
func (pool *Pool) GetSatellites(ctx context.Context) (satellites []storj.NodeID) {
	defer mon.Task()(&ctx)(nil)
	for sat := range pool.trustedSatellites {
		satellites = append(satellites, sat)
	}
	return satellites
}

// GetAddress returns the address of a satellite in the trusted list
func (pool *Pool) GetAddress(ctx context.Context, id storj.NodeID) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	info, ok := pool.trustedSatellites[id]
	if !ok {
		return "", Error.New("ID %v not found in trusted list", id)
	}
	return info.url.Address, nil
}
