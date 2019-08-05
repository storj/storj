// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package overlay_test

import (
	"strconv"
	"testing"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"
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
	numNodes := 100000
	nodeIDs := make([]storj.NodeID, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeIDs[i] = teststorj.NodeIDFromString(strconv.Itoa(i))
	}
	b.Run("lock", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx := i % numNodes
			unlockFunc := ml.Lock(nodeIDs[idx])
			unlockFunc()
		}
	})
	b.Run("rlock", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx := i % numNodes
			unlockFunc := ml.RLock(nodeIDs[idx])
			unlockFunc()
		}
	})
}
