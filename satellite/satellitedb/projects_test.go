// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectsGetByPublicID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()

		prj, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
		})
		require.NoError(t, err)
		require.NotNil(t, prj)

		pubID := prj.PublicID
		require.NotNil(t, pubID)
		require.False(t, pubID.IsZero())

		prj, err = projects.GetByPublicID(ctx, pubID)
		require.NoError(t, err)
		require.Equal(t, pubID, prj.PublicID)
	})
}

func TestProjectsGetSalt(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()

		prj, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
		})
		require.NoError(t, err)
		require.NotNil(t, prj)

		salt, err := projects.GetSalt(ctx, prj.ID)
		require.NoError(t, err)

		hash := sha256.Sum256(prj.ID[:])
		require.Equal(t, hash[:], salt)
	})
}
