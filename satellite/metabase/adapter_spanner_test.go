// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/private/mud"
	"storj.io/storj/private/mud/mudtest"
	"storj.io/storj/satellite/metabase"
)

func TestBeginObjectSpanner(t *testing.T) {
	spannerConnection := os.Getenv("STORJ_TEST_SPANNER")
	if spannerConnection == "" || spannerConnection == "omit" {
		t.Skip("STORJ_TEST_SPANNER is not defined, no available spanner instance to test")
		return
	}

	mudtest.Run[*metabase.SpannerAdapter](t, mudtest.WithTestLogger(t, func(ball *mud.Ball) {
		metabase.SpannerTestModule(ball, spannerConnection)
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
