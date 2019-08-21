// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/testdata"
)

// loadSnapshots loads the coded snapshots defined in testdata/snapshot_vX.go
// These snapshots were converted from SQL scripts into a Snapshot struct
// because we needed a way to distinguish what SQL executes against which database connection.
func loadSnapshots() (*dbschema.Snapshots, error) {
	snapshots := &dbschema.Snapshots{}

	// TODO: Do we need this? Or maybe I should integrate the Snapshot struct I created with this?
	// snapshot represents clean DB state
	snapshots.Add(&dbschema.Snapshot{
		Version: -1,
		Schema:  &dbschema.Schema{},
		Script:  "",
	})

	// TODO: Like above, I don't like that there's 2 concepts of a Snapshot but maybe it's needed, maybe not?
	for _, snapshot := range testdata.GetSnapshots() {
		// TODO: Do we need to create a dbschema.Snapshot per Snapshot step?
		snap := &dbschema.Snapshot{
			Version: snapshot.Version,
			Schema:  nil,                          // TODO: Do we need to fill this?
			Script:  snapshot.Steps[0].Statements, // TODO: Is this correct? Like above, do we need a dbschema.Snapshot per step?
		}
		snap.Version = snapshot.Version

		snapshots.Add(snap)
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

func TestMigrate(t *testing.T) {
	ctx := testcontext.New(t)
	snapshots, err := loadSnapshots()
	require.NoError(t, err)

	log := zaptest.NewLogger(t)

	cfg := storagenodedb.Config{
		Pieces:   ctx.Dir("storage"),
		Info2:    ctx.Dir("storage") + "/info.db",
		Kademlia: ctx.Dir("storage") + "/kademlia",
	}

	// create a new satellitedb connection
	db, err := storagenodedb.New(log, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	// get migration for this database
	migrations := db.Migration()
	for i, step := range migrations.Steps {
		// the schema is different when migration step is before the step, cannot test the layout
		tag := fmt.Sprintf("#%d - v%d", i, step.Version)

		// run migration up to a specific version
		err := migrations.TargetVersion(step.Version).Run(log.Named("migrate"), db.VersionsMigration())
		require.NoError(t, err, tag)

		// find the matching expected version
		expected, ok := snapshots.FindVersion(step.Version)
		require.True(t, ok)

		// insert data for new tables
		if newdata := newData(expected); newdata != "" {
			_, err = db.Versions().Exec(newdata)
			require.NoError(t, err, tag)
		}

		// load schema from database
		currentSchema, err := sqliteutil.QuerySchema(db.Versions())
		require.NoError(t, err, tag)

		// we don't care changes in versions table
		currentSchema.DropTable("versions")

		// load data from database
		currentData, err := sqliteutil.QueryData(db.Versions(), currentSchema)
		require.NoError(t, err, tag)

		// verify schema and data
		require.Equal(t, expected.Schema, currentSchema, tag)
		require.Equal(t, expected.Data, currentData, tag)
	}
}
