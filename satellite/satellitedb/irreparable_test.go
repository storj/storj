// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/datarepair"
)

func TestPostgres(t *testing.T) {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	db, err := NewDB(*testPostgres)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	err = db.CreateTables()
	assert.NoError(t, err)

	irrdb := db.Irreparable()

	testDatabase(ctx, t, irrdb)
}

func TestSqlite3(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	db, err := NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	err = db.CreateTables()
	assert.NoError(t, err)

	irrdb := db.Irreparable()

	testDatabase(ctx, t, irrdb)
}

func testDatabase(ctx context.Context, t *testing.T, irrdb datarepair.IrreparableDB) {
	//testing variables
	segmentInfo := &datarepair.RemoteSegmentInfo{
		EncryptedSegmentPath:   []byte("IamSegmentkeyinfo"),
		EncryptedSegmentDetail: []byte("IamSegmentdetailinfo"),
		LostPiecesCount:        int64(10),
		RepairUnixSec:          time.Now().Unix(),
		RepairAttemptCount:     int64(10),
	}

	{ // New entry
		err := irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		assert.NoError(t, err)
	}

	{ //Increment the already existing entry
		err := irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		assert.NoError(t, err)
		segmentInfo.RepairAttemptCount++

		dbxInfo, err := irrdb.Get(ctx, segmentInfo.EncryptedSegmentPath)
		assert.NoError(t, err)
		assert.Equal(t, segmentInfo, dbxInfo)
	}

	{ //Delete existing entry
		err := irrdb.Delete(ctx, segmentInfo.EncryptedSegmentPath)
		assert.NoError(t, err)

		_, err = irrdb.Get(ctx, segmentInfo.EncryptedSegmentPath)
		assert.Error(t, err)
	}
}
