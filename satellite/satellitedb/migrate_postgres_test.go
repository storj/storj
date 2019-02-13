// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// VersionSchema
type VersionSchema struct {
	Version int
	Script  string
	*dbschema.Schema
}

var (
	versions struct {
		sync.Once
		list []*VersionSchema
		err  error
	}
)

func loadVersions(connstr string) ([]*VersionSchema, error) {
	versions.Do(func() {
		matches, err := filepath.Glob("testdata/postgres.*")
		if err != nil {
			versions.err = err
			return
		}

		for _, match := range matches {
			versionStr := match[19 : len(match)-4]
			version, err := strconv.Atoi(versionStr)
			if err != nil {
				versions.err = err
				return
			}

			data, err := ioutil.ReadFile(match)
			if err != nil {
				versions.err = err
				return
			}

			schema, err := pgutil.LoadSchemaFromSQL(connstr, string(data))
			if err != nil {
				versions.err = err
				return
			}

			versions.list = append(versions.list, &VersionSchema{
				Version: version,
				Script:  string(data),
				Schema:  schema,
			})
		}
	})
	return versions.list, versions.err
}

func TestMigrateSchemas(t *testing.T) {
	if *satellitedbtest.TestPostgres == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + satellitedbtest.DefaultPostgresConn)
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	versions, err := loadVersions(*satellitedbtest.TestPostgres)
	require.NoError(t, err)

	t.Log(versions)
	/*
		schemaName := "pgutil-query-" + pgutil.RandomString(8)
		connstr := pgutil.ConnstrWithSchema(*satellitedbtest.TestPostgres, schemaName)

		db, err := sql.Open("postgres", connstr)
		require.NoError(t, err)

		defer ctx.Check(db.Close)

		require.NoError(t, pgutil.CreateSchema(db, schemaName))
		defer func() {
			require.NoError(t, pgutil.DropSchema(db, schemaName))
		}()

		emptySchema, err := pgutil.QuerySchema(db)
		assert.NoError(t, err)
		assert.Equal(t, &dbschema.Schema{}, emptySchema)
	*/
}

func TestMigrateStarts(t *testing.T) {
	if *satellitedbtest.TestPostgres == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + satellitedbtest.DefaultPostgresConn)
	}

	versions, err := loadVersions(*satellitedbtest.TestPostgres)
	require.NoError(t, err)

	for _, version := range versions {
		version := version
		// 0 to 4 are versions that may be as starting point
		if version.Version < 0 || 4 < version.Version {
			continue
		}
		t.Run(strconv.Itoa(version.Version), func(t *testing.T) {
			log := zaptest.NewLogger(t)
			schemaName := "satellite/start-" + strconv.Itoa(version.Version) + pgutil.RandomString(8)
			connstr := pgutil.ConnstrWithSchema(*satellitedbtest.TestPostgres, schemaName)

			// create a new satellitedb connection
			sdb, err := satellitedb.New(log, connstr)
			require.NoError(t, err)
			defer func() { require.NoError(t, sdb.Close()) }()

			db := sdb.(*satellitedb.DB)

			// setup our own schema to avoid collisions
			require.NoError(t, db.CreateSchema(schemaName))
			defer func() { require.NoError(t, db.DropSchema(schemaName)) }()

			// insert the base data into postgres
			err = db.RawExec(version.Script)
			require.NoError(t, err)

			// run migrations
			require.NoError(t, db.CreateTables())

			// TODO: verify state
		})
	}
}
