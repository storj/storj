// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestConsoleTx(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		dbConsole := db.Console()

		t.Run("WithTx with success", func(t *testing.T) {
			name := "Sleve McDichael"
			var projInfo *console.Project

			err := dbConsole.WithTx(ctx, func(ctx context.Context, tx console.DBTx) (err error) {
				projectDB := tx.Projects()
				projInfo, err = projectDB.Insert(ctx, &console.Project{Name: name})
				require.NoError(t, err)
				require.NotZero(t, projInfo.ID)
				require.Equal(t, name, projInfo.Name)
				return err
			})
			require.NoError(t, err)

			projectDB := dbConsole.Projects()
			gotProjectInfo, err := projectDB.Get(ctx, projInfo.ID)
			require.NoError(t, err)

			require.Equal(t, projInfo.ID, gotProjectInfo.ID)
			require.Equal(t, projInfo.Name, gotProjectInfo.Name)
			require.Equal(t, projInfo.CreatedAt, gotProjectInfo.CreatedAt)
		})

		t.Run("WithTx with failure", func(t *testing.T) {
			name := "Bobson Dugnutt"
			var projInfo *console.Project

			err := dbConsole.WithTx(ctx, func(ctx context.Context, tx console.DBTx) (err error) {
				projectDB := tx.Projects()
				projInfo, err = projectDB.Insert(ctx, &console.Project{Name: name})
				require.NoError(t, err)
				require.NotZero(t, projInfo.ID)

				// verify retrievability inside the transaction
				gotProjInfo, err := projectDB.Get(ctx, projInfo.ID)
				require.NoError(t, err)
				require.Equal(t, projInfo.ID, gotProjInfo.ID)
				require.Equal(t, projInfo.Name, gotProjInfo.Name)
				require.Equal(t, projInfo.CreatedAt, gotProjInfo.CreatedAt)

				// but return an error anyway to cause rollback
				return errs.New("some errors just want to see the world burn")
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "see the world burn")

			// insertion should have been rolled back
			projectDB := dbConsole.Projects()
			gotProjInfo, err := projectDB.Get(ctx, projInfo.ID)
			require.Error(t, err)
			require.Nil(t, gotProjInfo)
		})
	}, satellitedbtest.WithSpanner())
}
