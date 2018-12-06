// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparabledb_test

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/datarepair/irreparabledb"
)

const (
	// postgres connstring that works with docker-compose
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
)

var (
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

func TestPostgres(t *testing.T) {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	irrdb, err := irreparabledb.New(*testPostgres)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(irrdb.Close)

	testDatabase(ctx, t, irrdb)
}

func TestSqlite3(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	irrdb, err := irreparabledb.New("sqlite3://file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(irrdb.Close)

	testDatabase(ctx, t, irrdb)
}

func testDatabase(ctx context.Context, t *testing.T, irrdb *irreparabledb.Database) {
	//testing variables
	segmentInfo := &irreparabledb.RemoteSegmentInfo{
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
