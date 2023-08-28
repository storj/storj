// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// ReliabilityCache caches the reliable nodes for the specified staleness duration
// and updates automatically from overlay.
//
// architecture: Service
type ReliabilityCache struct {
	overlay   *overlay.Service
	staleness time.Duration
	// define from which countries nodes should be marked as offline
	excludedCountryCodes map[location.CountryCode]struct{}
	mu                   sync.Mutex
	state                atomic.Value // contains immutable *reliabilityState
	placementRules       overlay.PlacementRules
}

// reliabilityState.
type reliabilityState struct {
	reliableOnline map[storj.NodeID]nodeselection.SelectedNode
	reliableAll    map[storj.NodeID]nodeselection.SelectedNode
	created        time.Time
}

// NewReliabilityCache creates a new reliability checking cache.
func NewReliabilityCache(overlay *overlay.Service, staleness time.Duration, placementRules overlay.PlacementRules, excludedCountries []string) *ReliabilityCache {
	excludedCountryCodes := make(map[location.CountryCode]struct{})
	for _, countryCode := range excludedCountries {
		if cc := location.ToCountryCode(countryCode); cc != location.None {
			excludedCountryCodes[cc] = struct{}{}
		}
	}

	return &ReliabilityCache{
		overlay:              overlay,
		staleness:            staleness,
		placementRules:       placementRules,
		excludedCountryCodes: excludedCountryCodes,
	}
}

// LastUpdate returns when the cache was last updated, or the zero value (time.Time{}) if it
// has never yet been updated. LastUpdate() does not trigger an update itself.
func (cache *ReliabilityCache) LastUpdate() time.Time {
	if state, ok := cache.state.Load().(*reliabilityState); ok {
		return state.created
	}
	return time.Time{}
}

// NumNodes returns the number of online active nodes (as determined by the reliability cache).
// This number is not guaranteed to be consistent with either the nodes database or the
// reliability cache after returning; it is just a best-effort count and should be treated as an
// estimate.
func (cache *ReliabilityCache) NumNodes(ctx context.Context) (numNodes int, err error) {
	state, err := cache.loadFast(ctx, time.Time{})
	if err != nil {
		return 0, err
	}

	return len(state.reliableOnline), nil
}

// MissingPieces returns piece indices that are unreliable with the given staleness period.
func (cache *ReliabilityCache) MissingPieces(ctx context.Context, created time.Time, pieces metabase.Pieces) (_ metabase.Pieces, err error) {
	state, err := cache.loadFast(ctx, created)
	if err != nil {
		return nil, err
	}
	var unreliable metabase.Pieces
	for _, p := range pieces {
		node, ok := state.reliableOnline[p.StorageNode]
		if !ok {
			unreliable = append(unreliable, p)
		} else if _, excluded := cache.excludedCountryCodes[node.CountryCode]; excluded {
			unreliable = append(unreliable, p)
		}
	}
	return unreliable, nil
}

// OutOfPlacementPieces checks which pieces are out of segment placement. Piece placement is defined by node location which is storing it.
func (cache *ReliabilityCache) OutOfPlacementPieces(ctx context.Context, created time.Time, pieces metabase.Pieces, placement storj.PlacementConstraint) (_ metabase.Pieces, err error) {
	defer mon.Task()(&ctx)(nil)

	if len(pieces) == 0 {
		return metabase.Pieces{}, nil
	}

	state, err := cache.loadFast(ctx, created)
	if err != nil {
		return nil, err
	}
	var outOfPlacementPieces metabase.Pieces
	nodeFilters := cache.placementRules(placement)
	for _, p := range pieces {
		if node, ok := state.reliableAll[p.StorageNode]; ok && !nodeFilters.Match(&node) {
			outOfPlacementPieces = append(outOfPlacementPieces, p)
		}
	}

	return outOfPlacementPieces, nil
}

// PiecesNodesLastNetsInOrder returns the /24 subnet for each piece storage node, in order. If a
// requested node is not in the database or it's unreliable, an empty string will be returned corresponding
// to that node's last_net.
func (cache *ReliabilityCache) PiecesNodesLastNetsInOrder(ctx context.Context, created time.Time, pieces metabase.Pieces) (lastNets []string, err error) {
	defer mon.Task()(&ctx)(nil)

	if len(pieces) == 0 {
		return []string{}, nil
	}

	state, err := cache.loadFast(ctx, created)
	if err != nil {
		return nil, err
	}

	lastNets = make([]string, len(pieces))
	for i, piece := range pieces {
		if node, ok := state.reliableAll[piece.StorageNode]; ok {
			lastNets[i] = node.LastNet
		}
	}
	return lastNets, nil
}

func (cache *ReliabilityCache) loadFast(ctx context.Context, validUpTo time.Time) (_ *reliabilityState, err error) {
	// This code is designed to be very fast in the case where a refresh is not needed: just an
	// atomic load from rarely written to bit of shared memory. The general strategy is to first
	// read if the state suffices to answer the query. If not (due to it not existing, being
	// too stale, etc.), then we acquire the mutex to block other requests that may be stale
	// and ensure we only issue one refresh at a time. After acquiring the mutex, we have to
	// double check that the state is still stale because some other call may have beat us to
	// the acquisition. Only then do we refresh and can then proceed answering the query.

	state, ok := cache.state.Load().(*reliabilityState)
	if !ok || validUpTo.After(state.created) || time.Since(state.created) > cache.staleness {
		cache.mu.Lock()
		state, ok = cache.state.Load().(*reliabilityState)
		if !ok || validUpTo.After(state.created) || time.Since(state.created) > cache.staleness {
			state, err = cache.refreshLocked(ctx)
		}
		cache.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}

// Refresh refreshes the cache.
func (cache *ReliabilityCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	_, err = cache.refreshLocked(ctx)
	return err
}

// refreshLocked does the refreshes assuming the write mutex is held.
func (cache *ReliabilityCache) refreshLocked(ctx context.Context) (_ *reliabilityState, err error) {
	defer mon.Task()(&ctx)(&err)

	online, offline, err := cache.overlay.Reliable(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	state := &reliabilityState{
		created:        time.Now(),
		reliableOnline: make(map[storj.NodeID]nodeselection.SelectedNode, len(online)),
		reliableAll:    make(map[storj.NodeID]nodeselection.SelectedNode, len(online)+len(offline)),
	}
	for _, node := range online {
		state.reliableOnline[node.ID] = node
		state.reliableAll[node.ID] = node
	}
	for _, node := range offline {
		state.reliableAll[node.ID] = node
	}

	cache.state.Store(state)
	return state, nil
}
