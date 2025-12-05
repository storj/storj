// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

var anArrayOfStrings = []string{
	"bronco",
	"blacksmith",
	"tombstone",
	"shrederator",
	"tantrum",
	"witch doctor",
	"",
	"sawblaze",
	"minotaur",
	"biohazard",
	"bite force",
	"son of whyachi",
	"end game",
	"whiplash",
}

func TestByteaArray(t *testing.T) {
	withUniqueDB(t, "pgutil-types", func(ctx *testcontext.Context, t *testing.T, db *dbutil.TempDatabase) {
		array := make([][]byte, len(anArrayOfStrings))
		for i, s := range anArrayOfStrings {
			array[i] = []byte(s)
		}
		// set one element to nil to test how that works too
		array[0] = nil

		rows, err := db.QueryContext(ctx, `SELECT item FROM UNNEST($1::bytea[]) u(item)`, pgutil.ByteaArray(array))
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
		}()

		for _, expected := range array {
			require.True(t, rows.Next())
			var got []byte
			require.NoError(t, rows.Scan(&got))
			require.NotNil(t, got)
			if expected == nil {
				require.Len(t, got, 0)
			} else {
				require.Equal(t, expected, got)
			}
		}
		require.False(t, rows.Next())
	})
}

func TestNullByteaArray(t *testing.T) {
	withUniqueDB(t, "pgutil-types", func(ctx *testcontext.Context, t *testing.T, db *dbutil.TempDatabase) {
		array := make([][]byte, len(anArrayOfStrings))
		for i, s := range anArrayOfStrings {
			array[i] = []byte(s)
		}
		// set one element to nil to test how that works too
		array[0] = nil

		rows, err := db.QueryContext(ctx, `SELECT item FROM UNNEST($1::bytea[]) u(item)`, pgutil.NullByteaArray(array))
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
		}()

		for _, expected := range array {
			require.True(t, rows.Next())
			var got []byte
			require.NoError(t, rows.Scan(&got))
			if expected == nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, expected, got)
			}
		}
		require.False(t, rows.Next())
	})
}

func TestInt2Array(t *testing.T) {
	withUniqueDB(t, "pgutil-types", func(ctx *testcontext.Context, t *testing.T, db *dbutil.TempDatabase) {
		array := []int16{0, -1, 99, math.MinInt16, math.MaxInt16}

		rows, err := db.QueryContext(ctx, `SELECT item FROM UNNEST($1::int2[]) u(item)`, pgutil.Int2Array(array))
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
		}()

		for _, expected := range array {
			require.True(t, rows.Next())
			var got int16
			require.NoError(t, rows.Scan(&got))
			require.Equal(t, expected, got)
		}
		require.False(t, rows.Next())
	})
}

func TestPlacementConstraintArray(t *testing.T) {
	withUniqueDB(t, "pgutil-types", func(ctx *testcontext.Context, t *testing.T, db *dbutil.TempDatabase) {
		array := []storj.PlacementConstraint{storj.EveryCountry, storj.DE, storj.EU, storj.US, math.MaxUint16}

		// PostgreSQL (and SQL) don't have unsigned int types, but values above math.MaxInt16 should
		// translate ok in a round trip.
		rows, err := db.QueryContext(ctx, `SELECT item FROM UNNEST($1::int2[]) u(item)`, pgutil.PlacementConstraintArray(array))
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
		}()

		for _, expected := range array {
			require.True(t, rows.Next())
			var got storj.PlacementConstraint
			require.NoError(t, rows.Scan(&got))
			require.Equal(t, expected, got)
		}
		require.False(t, rows.Next())
	})
}

// withUniqueDB arranges for a unique database (or unique namespace in a
// database) and runs a test callback for each type of test database available.
func withUniqueDB(t *testing.T, namePrefix string, cb func(ctx *testcontext.Context, t *testing.T, db *dbutil.TempDatabase)) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connStr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connStr, namePrefix, nil)
		if err != nil {
			t.Fatalf("encountered error: %v", err)
		}
		defer ctx.Check(db.Close)

		cb(ctx, t, db)
	})
}
