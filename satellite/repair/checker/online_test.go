// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/overlay"
)

func TestReliabilityCache_Concurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ocache := overlay.NewService(zap.NewNop(), fakeOverlayDB{}, overlay.Config{})
	rcache := NewReliabilityCache(ocache, time.Millisecond)

	for i := 0; i < 10; i++ {
		ctx.Go(func() error {
			for i := 0; i < 10000; i++ {
				pieces := []*pb.RemotePiece{{NodeId: testrand.NodeID()}}
				_, err := rcache.MissingPieces(ctx, time.Now(), pieces)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	ctx.Wait()
}

type fakeOverlayDB struct{ overlay.DB }

func (fakeOverlayDB) Reliable(context.Context, *overlay.NodeCriteria) (storj.NodeIDList, error) {
	return storj.NodeIDList{
		testrand.NodeID(),
		testrand.NodeID(),
		testrand.NodeID(),
		testrand.NodeID(),
	}, nil
}
