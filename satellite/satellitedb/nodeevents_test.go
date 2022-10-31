// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestNodeEvents(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		testID := teststorj.NodeIDFromString("test")
		testEmail := "test@storj.test"
		eventType := nodeevents.Disqualified

		neFromInsert, err := db.NodeEvents().Insert(ctx, testEmail, testID, eventType)
		require.NoError(t, err)
		require.NotNil(t, neFromInsert.ID)
		require.Equal(t, testID, neFromInsert.NodeID)
		require.Equal(t, testEmail, neFromInsert.Email)
		require.Equal(t, eventType, neFromInsert.Event)
		require.NotNil(t, neFromInsert.CreatedAt)
		require.Nil(t, neFromInsert.EmailSent)

		neFromGet, err := db.NodeEvents().GetLatestByEmailAndEvent(ctx, neFromInsert.Email, neFromInsert.Event)
		require.NoError(t, err)
		require.Equal(t, neFromInsert, neFromGet)
	})
}
