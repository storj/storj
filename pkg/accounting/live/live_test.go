// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"encoding/binary"
	"math/rand"
	"sync"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	valuesListSize = 1000
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
	service, err := New(zap.L().Named("live-accounting"), config)
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
		var u uuid.UUID
		binary.BigEndian.PutUint64(u[len(u)-8:], uint64(i))
		projectIDs[i] = u
	}

	// send lots of space used updates for all of these projects to the live
	// accounting store.
	ctx := context.Background()
	var wg sync.WaitGroup
	for _, projID := range projectIDs {
		wg.Add(1)
		go func(projID uuid.UUID) {
			defer wg.Done()

			// have each project sending the values in a different order
			myValues := make([]int64, valuesListSize)
			copy(myValues, someValues)
			rand.Shuffle(valuesListSize, func(v1, v2 int) {
				myValues[v1], myValues[v2] = myValues[v2], myValues[v1]
			})

			for _, val := range myValues {
				service.AddProjectStorageUsage(ctx, projID, val, val)
			}
		}(projID)
	}
	wg.Wait()

	// make sure all of them got all updates and got right totals
	for _, projID := range projectIDs {
		inlineUsed, remoteUsed, err := service.GetProjectStorageUsage(ctx, projID)
		require.NoError(t, err)
		assert.Equal(t, inlineUsed, sum)
		assert.Equal(t, remoteUsed, sum)
	}
}
