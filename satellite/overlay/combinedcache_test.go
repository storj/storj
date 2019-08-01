// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/overlay"
)

func TestCachedChecker(t *testing.T) {
	cachedChecker := overlay.NewCombinedCache(nil, 30*time.Minute)

	nodeID := teststorj.NodeIDFromString("testid")

	node := &pb.Node{
		Id: nodeID,
		Address: &pb.NodeAddress{
			Address:   "127.0.0.3",
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
		},
		LastIp: "127.0.0.1",
	}

	require.True(t, cachedChecker.SetAndCompareAddress(node))
	require.False(t, cachedChecker.SetAndCompareAddress(node))

	// Now change each item and ensure it triggers change
	node.LastIp = "127.0.0.2"
	require.True(t, cachedChecker.SetAndCompareAddress(node))
	require.False(t, cachedChecker.SetAndCompareAddress(node))

	node.Address.Address = "127.0.0.4"
	require.True(t, cachedChecker.SetAndCompareAddress(node))
	require.False(t, cachedChecker.SetAndCompareAddress(node))

	node.Address.Transport = 2
	require.True(t, cachedChecker.SetAndCompareAddress(node))
	require.False(t, cachedChecker.SetAndCompareAddress(node))

	stats := cachedChecker.GetNodeStats(nodeID, true)
	require.Nil(t, stats)

	expStats := &overlay.NodeStats{
		Latency90: 1,
	}
	cachedChecker.SetNodeStats(nodeID, true, expStats)

	recStats := cachedChecker.GetNodeStats(nodeID, true)
	require.EqualValues(t, expStats, recStats)

	recStats = cachedChecker.GetNodeStats(nodeID, false)
	require.Nil(t, recStats)
}
