// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"fmt"
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

		t.Run("Create, get and delete", func(t *testing.T) {
			domain.ProjectID = project1.ID

			createdDomain, err := domains.Create(ctx, domain)
			require.NoError(t, err)
			require.NotNil(t, createdDomain)
			require.Equal(t, project1.ID, createdDomain.ProjectID)
			require.Equal(t, user.ID, createdDomain.CreatedBy)

			retrievedDomain, err := domains.GetByProjectIDAndSubdomain(ctx, project1.ID, domain.Subdomain)
			require.NoError(t, err)
			require.NotNil(t, retrievedDomain)
			require.Equal(t, createdDomain.Subdomain, retrievedDomain.Subdomain)
			require.Equal(t, createdDomain.Prefix, retrievedDomain.Prefix)
			require.Equal(t, createdDomain.AccessID, retrievedDomain.AccessID)
			require.Equal(t, createdDomain.ProjectID, retrievedDomain.ProjectID)
			require.Equal(t, createdDomain.CreatedBy, retrievedDomain.CreatedBy)

			retrievedDomain, err = domains.GetByProjectIDAndSubdomain(ctx, project1.ID, "random")
			require.True(t, console.ErrNoSubdomain.Has(err))
			require.Nil(t, retrievedDomain)

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

		t.Run("GetPagedByProjectID + GetAllDomainNamesByProjectID", func(t *testing.T) {
			const totalDomains = 10
			for i := 0; i < totalDomains; i++ {
				d := domain
				d.ProjectID = project1.ID
				d.Subdomain = fmt.Sprintf("testsubdomain%d.example.com", i)

				createdDomain, err := domains.Create(ctx, d)
				require.NoError(t, err)
				require.NotNil(t, createdDomain)
			}

			cursor := console.DomainCursor{
				Limit:          5,
				Page:           1,
				Order:          console.SubdomainOrder,
				OrderDirection: console.Ascending,
			}

			page, err := domains.GetPagedByProjectID(ctx, project1.ID, cursor)
			require.NoError(t, err)
			require.NotNil(t, page)
			require.Equal(t, uint64(totalDomains), page.TotalCount)
			require.Len(t, page.Domains, int(cursor.Limit))

			for i := 1; i < len(page.Domains); i++ {
				prev := page.Domains[i-1].Subdomain
				current := page.Domains[i].Subdomain
				require.LessOrEqual(t, prev, current)
			}

			cursor.Page = 2
			page2, err := domains.GetPagedByProjectID(ctx, project1.ID, cursor)
			require.NoError(t, err)
			require.NotNil(t, page2)
			require.Equal(t, uint64(totalDomains), page2.TotalCount)
			require.Len(t, page2.Domains, totalDomains-int(cursor.Limit))

			names, err := domains.GetAllDomainNamesByProjectID(ctx, project1.ID)
			require.NoError(t, err)
			require.Len(t, names, totalDomains)
		})

		t.Run("DeleteAllByProjectID", func(t *testing.T) {
			err := domains.DeleteAllByProjectID(ctx, project1.ID)
			require.NoError(t, err)

			cursor := console.DomainCursor{
				Search:         "",
				Limit:          5,
				Page:           1,
				Order:          console.SubdomainOrder,
				OrderDirection: console.Ascending,
			}
			page, err := domains.GetPagedByProjectID(ctx, project1.ID, cursor)
			require.NoError(t, err)
			require.NotNil(t, page)
			require.Equal(t, uint64(0), page.TotalCount)
		})
	})
}
