// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/rewards"
)

func TestStaticDB(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	world := rewards.PartnerInfo{
		Name: "World",
		ID:   "WORLD0",
	}

	hello := rewards.PartnerInfo{
		Name: "Hello",
		ID:   "11111111-1111-1111-1111-111111111111",
	}

	db, err := rewards.NewPartnersStaticDB(&rewards.PartnerList{
		Partners: []rewards.PartnerInfo{world, hello},
	})
	require.NotNil(t, db)
	require.NoError(t, err)

	byID, err := db.ByID(ctx, "WORLD0")
	require.NoError(t, err)
	require.Equal(t, world, byID)

	byName, err := db.ByName(ctx, "World")
	require.NoError(t, err)
	require.Equal(t, world, byName)

	byUserAgent, err := db.ByUserAgent(ctx, "wOrLd")
	require.NoError(t, err)
	require.Equal(t, world, byUserAgent)

	all, err := db.All(ctx)
	require.NoError(t, err)
	require.EqualValues(t, []rewards.PartnerInfo{hello, world}, all)
}
