// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package overlay_test

import (
	"testing"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite/overlay"
)

func TestKeyLock(t *testing.T) {
	ml := overlay.NewKeyLock()
	key := teststorj.NodeIDFromString("hi")
	unlockFunc := ml.Lock(key)
	unlockFunc()
	unlockFunc = ml.RLock(key)
	unlockFunc()
}

func BenchmarkKeyLock(b *testing.B) {
	b.ReportAllocs()
	ml := overlay.NewKeyLock()
	numNodes := 100
	nodeIDs := make([]storj.NodeID, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeIDs[i] = testrand.NodeID()
	}
	b.Run("lock all new nodes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			unlockFunc := ml.Lock(testrand.NodeID())
			unlockFunc()
		}
	})
	b.Run("lock existing nodes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx := i % numNodes
			unlockFunc := ml.Lock(nodeIDs[idx])
			unlockFunc()
		}
	})
	b.Run("rlock existing nodes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx := i % numNodes
			unlockFunc := ml.RLock(nodeIDs[idx])
			unlockFunc()
		}
	})
}
