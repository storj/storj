// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package attribution_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		attributionDB := db.Attribution()

		newUUID := func() uuid.UUID {
			v, err := uuid.New()
			require.NoError(t, err)
			return *v
		}

		project1, project2 := newUUID(), newUUID()
		partner1, partner2 := newUUID(), newUUID()

		infos := []*attribution.Info{
			{project1, []byte("alpha"), partner1, time.Time{}},
			{project1, []byte("beta"), partner2, time.Time{}},
			{project2, []byte("alpha"), partner2, time.Time{}},
			{project2, []byte("beta"), partner1, time.Time{}},
		}

		for _, info := range infos {
			got, err := attributionDB.Insert(ctx, info)
			require.NoError(t, err)

			got.CreatedAt = time.Time{}
			assert.Equal(t, info, got)
		}

		for _, info := range infos {
			got, err := attributionDB.Get(ctx, info.ProjectID, info.BucketName)
			require.NoError(t, err)
			assert.Equal(t, info.PartnerID, got.PartnerID)
		}
	})
}
