// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// VersionSchema defines a versioned schema
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

// loadVersions loads all the dbschemas from testdata/postgres.*
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

			schema.DropTable("versions")

			versions.list = append(versions.list, &VersionSchema{
				Version: version,
				Script:  string(data),
				Schema:  schema,
			})
		}
	})

	return versions.list, versions.err
}

var (
	dbxschema struct {
		sync.Once
		*dbschema.Schema
		err error
	}
)

func loadDBXSchema(connstr, dbxscript string) (*dbschema.Schema, error) {
	dbxschema.Do(func() {
		dbxschema.Schema, dbxschema.err = pgutil.LoadSchemaFromSQL(connstr, dbxscript)
	})
	return dbxschema.Schema, dbxschema.err
}

func findVersion(versions []*VersionSchema, targetVersion int) *VersionSchema {
	for _, version := range versions {
		if version.Version == targetVersion {
			return version
		}
	}
	return nil
}

func TestMigrate(t *testing.T) {
	if *satellitedbtest.TestPostgres == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + satellitedbtest.DefaultPostgresConn)
	}

	versions, err := loadVersions(*satellitedbtest.TestPostgres)
	require.NoError(t, err)

	for _, base := range versions {
		base := base
		// versions 0 to 4 can be a starting point
		if base.Version < 0 || 4 < base.Version {
			continue
		}

		t.Run(strconv.Itoa(base.Version), func(t *testing.T) {
			log := zaptest.NewLogger(t)
			schemaName := "migrate/satellite/" + strconv.Itoa(base.Version) + pgutil.RandomString(8)
			connstr := pgutil.ConnstrWithSchema(*satellitedbtest.TestPostgres, schemaName)

			// create a new satellitedb connection
			sdb, err := satellitedb.New(log, connstr)
			require.NoError(t, err)
			defer func() { require.NoError(t, sdb.Close()) }()

			db := sdb.(*satellitedb.DB)

			// setup our own schema to avoid collisions
			require.NoError(t, db.CreateSchema(schemaName))
			defer func() { require.NoError(t, db.DropSchema(schemaName)) }()

			rawdb := db.TestDBAccess()

			// insert the base data into postgres
			_, err = rawdb.Exec(base.Script)
			require.NoError(t, err)

			var finalSchema *dbschema.Schema

			migrations := db.PostgresMigration()
			for i, step := range migrations.Steps {
				// the schema is different when migration step is before the step, cannot test the layout
				if step.Version < base.Version {
					continue
				}

				tag := fmt.Sprintf("#%d - v%d", i, step.Version)
				require.NoError(t, migrations.TargetVersion(step.Version).Run(log.Named("migrate"), rawdb), tag)

				currentSchema, err := pgutil.QuerySchema(rawdb)
				require.NoError(t, err, tag)

				currentSchema.DropTable("versions")

				expected := findVersion(versions, step.Version)
				require.Equal(t, expected.Schema, currentSchema, tag)

				finalSchema = currentSchema
			}

			dbxschema, err := loadDBXSchema(*satellitedbtest.TestPostgres, rawdb.Schema())
			require.NoError(t, err)

			require.Equal(t, dbxschema, finalSchema, "dbx")
		})
	}
}
