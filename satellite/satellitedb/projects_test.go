// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/satellitedb/dbx"
)

func TestProjectFromDbx(t *testing.T) {
	ctx := context.Background()

	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		project, err := projectFromDBX(ctx, nil)

		assert.Nil(t, project)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid ID", func(t *testing.T) {
		dbxProject := dbx.Project{
			Id: []byte("qweqwe"),
		}

		project, err := projectFromDBX(ctx, &dbxProject)

		assert.Nil(t, project)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}
