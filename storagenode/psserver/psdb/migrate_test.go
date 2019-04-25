// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/storagenode/psserver/psdb"
)

// loadSnapshots loads all the dbschemas from testdata/db.* caching the result
func loadSnapshots() (*dbschema.Snapshots, error) {
	snapshots := &dbschema.Snapshots{}

	// snapshot represents clean DB state
	snapshots.Add(&dbschema.Snapshot{
		Version: -1,
		Schema:  &dbschema.Schema{},
		Script:  "",
	})

	// find all sql files
	matches, err := filepath.Glob("testdata/sqlite.*")
	if err != nil {
		return nil, err
	}

	for _, match := range matches {
		versionStr := match[17 : len(match)-4] // hack to avoid trim issues with path differences in windows/linux
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return nil, err
		}

		scriptData, err := ioutil.ReadFile(match)
		if err != nil {
			return nil, err
		}

		snapshot, err := sqliteutil.LoadSnapshotFromSQL(string(scriptData))
		if err != nil {
			return nil, err
		}
		snapshot.Version = version

		snapshots.Add(snapshot)
	}

	snapshots.Sort()

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

const (
	minBaseVersion = -1 // clean DB
	maxBaseVersion = 0
)

func TestMigrate(t *testing.T) {
	snapshots, err := loadSnapshots()
	require.NoError(t, err)

	for _, base := range snapshots.List {
		if base.Version < minBaseVersion || maxBaseVersion < base.Version {
			continue
		}

		t.Run(strconv.Itoa(base.Version), func(t *testing.T) {
			log := zaptest.NewLogger(t)

			// create a new satellitedb connection
			db, err := psdb.OpenInMemory()
			require.NoError(t, err)
			defer func() { require.NoError(t, db.Close()) }()

			// insert the base data into sqlite
			_, err = db.RawDB().Exec(base.Script)
			require.NoError(t, err)

			// get migration for this database
			migrations := db.Migration()
			for i, step := range migrations.Steps {
				// the schema is different when migration step is before the step, cannot test the layout
				if step.Version < base.Version {
					continue
				}

				tag := fmt.Sprintf("#%d - v%d", i, step.Version)

				// run migration up to a specific version
				err := migrations.TargetVersion(step.Version).Run(log.Named("migrate"), db)
				require.NoError(t, err, tag)

				// find the matching expected version
				expected, ok := snapshots.FindVersion(step.Version)
				require.True(t, ok)

				// insert data for new tables
				if newdata := newData(expected); newdata != "" && step.Version > base.Version {
					_, err = db.RawDB().Exec(newdata)
					require.NoError(t, err, tag)
				}

				// load schema from database
				currentSchema, err := sqliteutil.QuerySchema(db.RawDB())
				require.NoError(t, err, tag)

				// we don't care changes in versions table
				currentSchema.DropTable("versions")

				// load data from database
				currentData, err := sqliteutil.QueryData(db.RawDB(), currentSchema)
				require.NoError(t, err, tag)

				// verify schema and data
				require.Equal(t, expected.Schema, currentSchema, tag)
				require.Equal(t, expected.Data, currentData, tag)
			}
		})
	}
}
