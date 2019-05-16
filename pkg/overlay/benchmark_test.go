// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func BenchmarkOffline(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		const (
			TotalNodeCount = 200
			OnlineCount    = 90
			OfflineCount   = 10
		)

		overlaydb := db.OverlayCache()
		ctx := context.Background()

		var check []storj.NodeID
		for i := 0; i < TotalNodeCount; i++ {
			var id storj.NodeID
			_, _ = rand.Read(id[:]) // math/rand never returns error

			err := overlaydb.UpdateAddress(ctx, &pb.Node{
				Id: id,
			})
			require.NoError(b, err)

			if i < OnlineCount {
				check = append(check, id)
			}
		}

		// create random offline node ids to check
		for i := 0; i < OfflineCount; i++ {
			var id storj.NodeID
			_, _ = rand.Read(id[:]) // math/rand never returns error
			check = append(check, id)
		}

		criteria := &overlay.NodeCriteria{
			AuditCount:         0,
			AuditSuccessRatio:  0.5,
			OnlineWindow:       1000 * time.Hour,
			UptimeCount:        0,
			UptimeSuccessRatio: 0.5,
		}

		b.ResetTimer()
		defer b.StopTimer()
		for i := 0; i < b.N; i++ {
			badNodes, err := overlaydb.KnownUnreliableOrOffline(ctx, criteria, check)
			require.NoError(b, err)
			require.Len(b, badNodes, OfflineCount)
		}
	})
}
