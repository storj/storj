// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDomainsRepository(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		domains := db.Console().Domains()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "test",
			Email:        "test@example.test",
			PasswordHash: []byte("newPass"),
		})
		require.NoError(t, err)
		require.NotNil(t, user)

		project1, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "ProjectName1"})
		require.NoError(t, err)
		project2, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "ProjectName2"})
		require.NoError(t, err)

		domain := console.Domain{
			CreatedBy: user.ID,
			Subdomain: "testSubdomain.example.com",
			Prefix:    "testPrefix",
			AccessID:  "testAccessID",
		}

		t.Run("Create and delete", func(t *testing.T) {
			domain.ProjectID = project1.ID

			createdDomain, err := domains.Create(ctx, domain)
			require.NoError(t, err)
			require.NotNil(t, createdDomain)
			require.Equal(t, project1.ID, createdDomain.ProjectID)
			require.Equal(t, user.ID, createdDomain.CreatedBy)

			createdDomain, err = domains.Create(ctx, domain)
			require.Error(t, err)
			require.Nil(t, createdDomain)

			domain.ProjectID = project2.ID

			createdDomain, err = domains.Create(ctx, domain)
			require.NoError(t, err)
			require.NotNil(t, createdDomain)
			require.Equal(t, project2.ID, createdDomain.ProjectID)
			require.Equal(t, user.ID, createdDomain.CreatedBy)

			err = domains.Delete(ctx, project1.ID, domain.Subdomain)
			require.NoError(t, err)
			err = domains.Delete(ctx, project2.ID, domain.Subdomain)
			require.NoError(t, err)
		})
	})
}
