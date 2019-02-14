// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// loadSnapshots loads all the dbschemas from testdata/postgres.* caching the result
func loadSnapshots(connstr string) (*dbschema.Snapshots, error) {
	snapshots := &dbschema.Snapshots{}

	// find all postgres sql files
	matches, err := filepath.Glob("testdata/postgres.*")
	if err != nil {
		return nil, err
	}

	for _, match := range matches {
		versionStr := match[19 : len(match)-4] // hack to avoid trim issues with path differences in windows/linux
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return nil, err
		}

		scriptData, err := ioutil.ReadFile(match)
		if err != nil {
			return nil, err
		}

		snapshot, err := pgutil.LoadSnapshotFromSQL(connstr, string(scriptData))
		if err != nil {
			return nil, err
		}
		snapshot.Version = version

		snapshots.Add(snapshot)
	}

	return snapshots, nil
}

const newDataSeparator = `-- NEW DATA --`

func newData(snap *dbschema.Snapshot) string {
	tokens := strings.SplitN(snap.Script, newDataSeparator, 2)
	if len(tokens) != 2 {
		return ""
	}
	return tokens[1]
}

var (
	dbxschema struct {
		sync.Once
		*dbschema.Schema
		err error
	}
)

// loadDBXSChema loads dbxscript schema only once and caches it,
// it shouldn't change during the test
func loadDBXSchema(connstr, dbxscript string) (*dbschema.Schema, error) {
	dbxschema.Do(func() {
		dbxschema.Schema, dbxschema.err = pgutil.LoadSchemaFromSQL(connstr, dbxscript)
	})
	return dbxschema.Schema, dbxschema.err
}

func TestMigratePostgres(t *testing.T) {
	if *satellitedbtest.TestPostgres == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + satellitedbtest.DefaultPostgresConn)
	}

	snapshots, err := loadSnapshots(*satellitedbtest.TestPostgres)
	require.NoError(t, err)

	for _, base := range snapshots.List {
		// versions 0 to 4 can be a starting point
		if base.Version < 0 || 4 < base.Version {
			continue
		}

		t.Run(strconv.Itoa(base.Version), func(t *testing.T) {
			log := zaptest.NewLogger(t)
			schemaName := "migrate/satellite/" + strconv.Itoa(base.Version) + pgutil.RandomString(8)
			connstr := pgutil.ConnstrWithSchema(*satellitedbtest.TestPostgres, schemaName)

			// create a new satellitedb connection
			db, err := satellitedb.New(log, connstr)
			require.NoError(t, err)
			defer func() { require.NoError(t, db.Close()) }()

			// setup our own schema to avoid collisions
			require.NoError(t, db.CreateSchema(schemaName))
			defer func() { require.NoError(t, db.DropSchema(schemaName)) }()

			// we need raw database access unfortunately
			rawdb := db.(*satellitedb.DB).TestDBAccess()

			// insert the base data into postgres
			_, err = rawdb.Exec(base.Script)
			require.NoError(t, err)

			var finalSchema *dbschema.Schema

			// get migration for this database
			migrations := db.(*satellitedb.DB).PostgresMigration()
			for i, step := range migrations.Steps {
				// the schema is different when migration step is before the step, cannot test the layout
				if step.Version < base.Version {
					continue
				}

				tag := fmt.Sprintf("#%d - v%d", i, step.Version)

				// run migration up to a specific version
				err := migrations.TargetVersion(step.Version).Run(log.Named("migrate"), rawdb)
				require.NoError(t, err, tag)

				// find the matching expected version
				expected, ok := snapshots.FindVersion(step.Version)
				require.True(t, ok)

				// insert data for new tables
				if newdata := newData(expected); newdata != "" && step.Version > base.Version {
					_, err = rawdb.Exec(newdata)
					require.NoError(t, err, tag)
				}

				// load schema from database
				currentSchema, err := pgutil.QuerySchema(rawdb)
				require.NoError(t, err, tag)

				// we don't care changes in versions table
				currentSchema.DropTable("versions")

				// load data from database
				currentData, err := pgutil.QueryData(rawdb, currentSchema)
				require.NoError(t, err, tag)

				// verify schema and data
				require.Equal(t, expected.Schema, currentSchema, tag)
				require.Equal(t, expected.Data, currentData, tag)

				// keep the last version around
				finalSchema = currentSchema
			}

			// verify that we also match the dbx version
			dbxschema, err := loadDBXSchema(*satellitedbtest.TestPostgres, rawdb.Schema())
			require.NoError(t, err)

			require.Equal(t, dbxschema, finalSchema, "dbx")
		})
	}
}
