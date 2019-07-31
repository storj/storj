// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"math/rand"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/testrand"
)

func TestPlainMemoryLiveAccounting(t *testing.T) {
	const (
		valuesListSize  = 1000
		valueMultiplier = 4096
		numProjects     = 200
	)
	config := Config{
		StorageBackend: "plainmemory:",
	}
	service, err := New(zaptest.NewLogger(t).Named("live-accounting"), config)
	require.NoError(t, err)

	// ensure we are using the expected underlying type
	_, ok := service.(*plainMemoryLiveAccounting)
	require.True(t, ok)

	// make a largish list of varying values
	someValues := make([]int64, valuesListSize)
	sum := int64(0)
	for i := range someValues {
		someValues[i] = int64((i + 1) * valueMultiplier)
		sum += someValues[i]
	}

	// make up some project IDs
	projectIDs := make([]uuid.UUID, numProjects)
	for i := range projectIDs {
		projectIDs[i] = testrand.UUID()
	}

	// send lots of space used updates for all of these projects to the live
	// accounting store.
	errg, ctx := errgroup.WithContext(context.Background())
	for _, projID := range projectIDs {
		projID := projID
		errg.Go(func() error {
			// have each project sending the values in a different order
			myValues := make([]int64, valuesListSize)
			copy(myValues, someValues)
			rand.Shuffle(valuesListSize, func(v1, v2 int) {
				myValues[v1], myValues[v2] = myValues[v2], myValues[v1]
			})

			for _, val := range myValues {
				if err := service.AddProjectStorageUsage(ctx, projID, val, val); err != nil {
					return err
				}
			}
			return nil
		})
	}
	require.NoError(t, errg.Wait())

	// make sure all of the "projects" got all space updates and got right totals
	for _, projID := range projectIDs {
		inlineUsed, remoteUsed, err := service.GetProjectStorageUsage(ctx, projID)
		require.NoError(t, err)
		assert.Equalf(t, sum, inlineUsed, "projectID %v", projID)
		assert.Equalf(t, sum, remoteUsed, "projectID %v", projID)
	}
}

func TestResetTotals(t *testing.T) {
	config := Config{
		StorageBackend: "plainmemory:",
	}
	service, err := New(zaptest.NewLogger(t).Named("live-accounting"), config)
	require.NoError(t, err)

	// ensure we are using the expected underlying type
	_, ok := service.(*plainMemoryLiveAccounting)
	require.True(t, ok)

	ctx := context.Background()

	projectID := testrand.UUID()
	err = service.AddProjectStorageUsage(ctx, projectID, 0, -20)
	require.NoError(t, err)
}
