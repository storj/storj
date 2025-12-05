// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mud/mudtest"
)

func TestBeginObjectSpanner(t *testing.T) {
	spannerConnStr := dbtest.PickOrStartSpanner(t)

	mudtest.Run[*metabase.SpannerAdapter](t, mudtest.WithTestLogger(t, func(ball *mud.Ball) {
		metabase.SpannerTestModule(ball, spannerConnStr)
	}),
		func(ctx context.Context, t *testing.T, adapter *metabase.SpannerAdapter) {
			uuid := testrand.UUID()
			o := &metabase.Object{}
			err := adapter.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: metabase.ObjectStream{
					ProjectID: uuid,
				},
			}, o)
			require.NoError(t, err)
			require.Equal(t, metabase.Version(1), o.Version)

			err = adapter.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: metabase.ObjectStream{
					ProjectID: uuid,
				},
			}, o)
			require.NoError(t, err)
			require.Equal(t, metabase.Version(2), o.Version)

		})
}

func TestSpannerClientCompression(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "test",
		MaxNumberOfParts: 100,
		Compression:      "gzip",
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 4)
	})
}
