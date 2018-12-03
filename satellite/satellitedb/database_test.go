// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabase(t *testing.T) {
	database, err := NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	assert.NoError(t, err)

	err = database.CreateTables()
	assert.NoError(t, err)

	err = database.Close()
	assert.NoError(t, err)
}
