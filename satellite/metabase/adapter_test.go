// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestAdapterBeginObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// spanner if available, default DB if not
		adapter := db.ChooseAdapter(testrand.UUID())

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
