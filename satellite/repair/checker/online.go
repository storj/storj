// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

// ReliabilityCache caches known nodes for the specified staleness duration
// and updates automatically from overlay.
//
// architecture: Service
type ReliabilityCache struct {
	overlay      Overlay
	staleness    time.Duration
	onlineWindow time.Duration
	mu           sync.Mutex
	state        atomic.Value // contains immutable *reliabilityState
}

// reliabilityState.
type reliabilityState struct {
	nodeByID map[storj.NodeID]nodeselection.SelectedNode
	created  time.Time
}

// NewReliabilityCache creates a new reliability checking cache.
// onlineWindow is used to determine if storage nodes are considered online based on their last
// successful contact.
func NewReliabilityCache(overlay Overlay, staleness time.Duration, onlineWindow time.Duration) *ReliabilityCache {
	return &ReliabilityCache{
		overlay:      overlay,
		staleness:    staleness,
		onlineWindow: onlineWindow,
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

	return len(state.nodeByID), nil
}

// GetNodes gets the cached SelectedNode records (valid as of the given time) for each of
// the requested node IDs, and returns them in order. If a node is not in the reliability
// cache (that is, it is unknown or disqualified), an empty SelectedNode will be returned
// for the index corresponding to that node ID.
// Slice selectedNodes will be filled with results nodes and returned. It's length must be
// equal to nodeIDs slice.
func (cache *ReliabilityCache) GetNodes(ctx context.Context, validUpTo time.Time, nodeIDs []storj.NodeID, selectedNodes []nodeselection.SelectedNode) ([]nodeselection.SelectedNode, error) {
	state, err := cache.loadFast(ctx, validUpTo)
	if err != nil {
		return nil, err
	}

	if len(nodeIDs) != len(selectedNodes) {
		return nil, errs.New("nodeIDs length must be equal to selectedNodes: want %d have %d", len(nodeIDs), len(selectedNodes))
	}

	for i, nodeID := range nodeIDs {
		selectedNodes[i] = state.nodeByID[nodeID]
	}
	return selectedNodes, nil
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

	selectedNodes, err := cache.overlay.GetAllParticipatingNodesForRepair(ctx, cache.onlineWindow)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	state := &reliabilityState{
		created:  time.Now(),
		nodeByID: make(map[storj.NodeID]nodeselection.SelectedNode, len(selectedNodes)),
	}

	var online int64
	for _, node := range selectedNodes {
		state.nodeByID[node.ID] = node

		if node.Online {
			online++
		}
	}

	mon.IntVal("checker_online_nodes").Observe(online)
	mon.IntVal("checker_offline_nodes").Observe(int64(len(selectedNodes)) - online)

	cache.state.Store(state)
	return state, nil
}
