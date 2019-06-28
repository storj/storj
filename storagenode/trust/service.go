// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"fmt"
	"sync"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/storj"
)

// Error is the default error class
var Error = errs.Class("trust:")
var mon = monkit.Package()

// Pool implements different peer verifications.
type Pool struct {
	kademlia *kademlia.Kademlia

	mu sync.RWMutex

	trustAllSatellites bool
	trustedSatellites  map[storj.NodeID]*satelliteInfoCache
}

// satelliteInfoCache caches identity information about a satellite
type satelliteInfoCache struct {
	mu       sync.Mutex
	identity *identity.PeerIdentity
	address  string
}

// NewPool creates a new trust pool using kademlia to find certificates and with the specified list of trusted satellites.
func NewPool(kademlia *kademlia.Kademlia, trustAll bool, trustedSatelliteURLs string) (*Pool, error) {
	if trustAll {
		return &Pool{
			kademlia: kademlia,

			trustAllSatellites: true,
			trustedSatellites:  map[storj.NodeID]*satelliteInfoCache{},
		}, nil
	}

	// TODO: preload all satellite peer identities

	// parse the comma separated list of approved satellite IDs into an array of storj.NodeIDs
	trusted := make(map[storj.NodeID]*satelliteInfoCache)
	urls, err := storj.ParseNodeURLs(trustedSatelliteURLs)
	if err != nil {
		return nil, err
	}

	for _, node := range urls {
		trusted[node.ID] = &satelliteInfoCache{address: node.Address}
	}

	return &Pool{
		kademlia: kademlia,

		trustAllSatellites: false,
		trustedSatellites:  trusted,
	}, nil
}

// VerifySatelliteID checks whether id corresponds to a trusted satellite.
func (pool *Pool) VerifySatelliteID(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	if pool.trustAllSatellites {
		return nil
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	_, ok := pool.trustedSatellites[id]
	if !ok {
		return fmt.Errorf("satellite %q is untrusted", id)
	}
	return nil
}

// VerifyUplinkID verifides whether id corresponds to a trusted uplink.
func (pool *Pool) VerifyUplinkID(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	// trusting all the uplinks for now
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

	if pool.trustAllSatellites {
		// add a new entry
		if !ok {
			pool.mu.Lock()
			// did another goroutine manage to make it first?
			info, ok = pool.trustedSatellites[id]
			if !ok {
				info = &satelliteInfoCache{}
				pool.trustedSatellites[id] = info
			}
			pool.mu.Unlock()
		}
	} else {
		if !ok {
			return nil, fmt.Errorf("signee %q is untrusted", id)
		}
	}

	info.mu.Lock()
	defer info.mu.Unlock()

	if info.identity == nil {
		identity, err := pool.kademlia.FetchPeerIdentity(ctx, id)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		info.identity = identity
	}

	return signing.SigneeFromPeerIdentity(info.identity), nil
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
	info, ok := pool.trustedSatellites[id]
	if !ok {
		return "", Error.New("ID not found in trusted satellites list (%v)", id)
	}
	// TODO: return error if address == "" ?
	return info.address, nil
}
