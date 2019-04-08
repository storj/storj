// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/storj"
)

// Pool implements different peer verifications.
type Pool struct {
	kademlia *kademlia.Kademlia

	mu sync.RWMutex

	trustAllSatellites bool
	trustedSatellites  map[storj.NodeID]*satelliteInfoCache
}

// satelliteInfoCache caches identity information about a satellite
type satelliteInfoCache struct {
	once     sync.Once
	identity *identity.PeerIdentity
	err      error
}

// NewPool creates a new trust pool using kademlia to find certificates and with the specified list of trusted satellites.
func NewPool(kademlia *kademlia.Kademlia, trustAll bool, trustedSatelliteIDs string) (*Pool, error) {
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

	for _, s := range strings.Split(trustedSatelliteIDs, ",") {
		if s == "" {
			continue
		}

		satelliteID, err := storj.NodeIDFromString(s)
		if err != nil {
			return nil, err
		}
		trusted[satelliteID] = &satelliteInfoCache{} // we will set these later
	}

	return &Pool{
		kademlia: kademlia,

		trustAllSatellites: false,
		trustedSatellites:  trusted,
	}, nil
}

// VerifySatelliteID checks whether id corresponds to a trusted satellite.
func (pool *Pool) VerifySatelliteID(ctx context.Context, id storj.NodeID) error {
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
func (pool *Pool) VerifyUplinkID(ctx context.Context, id storj.NodeID) error {
	// trusting all the uplinks for now
	return nil
}

// GetSignee gets the corresponding signee for verifying signatures.
func (pool *Pool) GetSignee(ctx context.Context, id storj.NodeID) (signing.Signee, error) {
	// creating a new context here to avoid request context cancelling fetching peer identity
	nestedContext := context.Background()

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

	info.once.Do(func() {
		info.identity, info.err = pool.kademlia.FetchPeerIdentity(nestedContext, id)
	})

	if info.err != nil {
		return nil, info.err
	}
	return signing.SigneeFromPeerIdentity(info.identity), nil
}
