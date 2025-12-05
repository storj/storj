// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
)

var (
	// Segments in the EU placement.
	inline1 = []rangedloop.Segment{
		{StreamID: uuid.UUID{1}, EncryptedSize: 10, Placement: storj.EU},
	}
	remote2 = []rangedloop.Segment{
		{StreamID: uuid.UUID{2}, EncryptedSize: 16, RootPieceID: testrand.PieceID(), Pieces: metabase.Pieces{{}}, Placement: storj.EU},
		{StreamID: uuid.UUID{2}, EncryptedSize: 10, RootPieceID: testrand.PieceID(), Placement: storj.EU},
	}
	remote3 = []rangedloop.Segment{
		{StreamID: uuid.UUID{3}, EncryptedSize: 16, RootPieceID: testrand.PieceID(), Pieces: metabase.Pieces{{}}, Placement: storj.EU},
		{StreamID: uuid.UUID{3}, EncryptedSize: 16, RootPieceID: testrand.PieceID(), Pieces: metabase.Pieces{{}}, Placement: storj.EU},
		{StreamID: uuid.UUID{3}, EncryptedSize: 16, RootPieceID: testrand.PieceID(), Pieces: metabase.Pieces{{}}, Placement: storj.EU},
		{StreamID: uuid.UUID{3}, EncryptedSize: 10, RootPieceID: testrand.PieceID(), ExpiresAt: &time.Time{}, Placement: storj.EU},
	}

	// Segments in the US placement.
	inline4 = []rangedloop.Segment{
		{StreamID: uuid.UUID{4}, EncryptedSize: 9, Placement: storj.US},
	}
	remote5 = []rangedloop.Segment{
		{StreamID: uuid.UUID{5}, EncryptedSize: 20, RootPieceID: testrand.PieceID(), Pieces: metabase.Pieces{{}}, Placement: storj.US},
		{StreamID: uuid.UUID{5}, EncryptedSize: 40, RootPieceID: testrand.PieceID(), Pieces: metabase.Pieces{{}}, Placement: storj.US},
		{StreamID: uuid.UUID{5}, EncryptedSize: 5, RootPieceID: testrand.PieceID(), Placement: storj.US},
	}
)

func TestObserver(t *testing.T) {
	ctx := testcontext.New(t)

	loop := func(tb testing.TB, obs *Observer, streams ...[]rangedloop.Segment) PlacementsMetrics {
		service := rangedloop.NewService(
			zap.NewNop(),
			rangedloop.Config{BatchSize: 2, Parallelism: 2},
			&rangedlooptest.RangeSplitter{Segments: combineSegments(streams...)},
			[]rangedloop.Observer{obs})
		_, err := service.RunOnce(ctx)
		require.NoError(tb, err)
		return obs.TestingMetrics()
	}

	t.Run("stats aggregation", func(t *testing.T) {
		obs := NewObserver()

		metrics := loop(t, obs, inline1, remote2, remote3, inline4, remote5)

		metricsLen := storj.EU + 1
		if storj.EU < storj.US {
			metricsLen = storj.US + 1
		}

		expectedMetrics := PlacementsMetrics(make([]Metrics, metricsLen))
		expectedMetrics[storj.EU] = Metrics{
			InlineObjects:              1,
			RemoteObjects:              2,
			TotalInlineSegments:        3,
			TotalRemoteSegments:        4,
			TotalInlineBytes:           30,
			TotalRemoteBytes:           64,
			TotalSegmentsWithExpiresAt: 1,
		}
		expectedMetrics[storj.US] = Metrics{
			InlineObjects:              1,
			RemoteObjects:              1,
			TotalInlineSegments:        2,
			TotalRemoteSegments:        2,
			TotalInlineBytes:           14,
			TotalRemoteBytes:           60,
			TotalSegmentsWithExpiresAt: 0,
		}

		require.Len(t, metrics, len(expectedMetrics))
		require.Equal(t, expectedMetrics, metrics)
	})

	t.Run("stats reset by start", func(t *testing.T) {
		obs := NewObserver()

		_ = loop(t, obs, inline1)

		// Any metrics gathered during the first loop should be dropped.
		metrics := loop(t, obs, remote3, remote5)

		metricsLen := storj.EU + 1
		if storj.EU < storj.US {
			metricsLen = storj.US + 1
		}

		expectedMetrics := PlacementsMetrics(make([]Metrics, metricsLen))
		expectedMetrics[storj.EU] = Metrics{
			InlineObjects:              0,
			RemoteObjects:              1,
			TotalInlineSegments:        1,
			TotalRemoteSegments:        3,
			TotalInlineBytes:           10,
			TotalRemoteBytes:           48,
			TotalSegmentsWithExpiresAt: 1,
		}
		expectedMetrics[storj.US] = Metrics{
			InlineObjects:              0,
			RemoteObjects:              1,
			TotalInlineSegments:        1,
			TotalRemoteSegments:        2,
			TotalInlineBytes:           5,
			TotalRemoteBytes:           60,
			TotalSegmentsWithExpiresAt: 0,
		}

		require.Len(t, metrics, len(expectedMetrics))
		require.Equal(t, expectedMetrics, metrics)
	})

	t.Run("join fails gracefully on bad partial type", func(t *testing.T) {
		type wrongPartial struct{ rangedloop.Partial }
		obs := NewObserver()
		err := obs.Start(ctx, time.Time{})
		require.NoError(t, err)
		err = obs.Join(ctx, wrongPartial{})
		require.EqualError(t, err, "metrics: expected *metrics.observerFork but got metrics.wrongPartial")
	})
}

func combineSegments(ss ...[]rangedloop.Segment) []rangedloop.Segment {
	var combined []rangedloop.Segment
	for _, s := range ss {
		combined = append(combined, s...)
	}
	return combined
}
