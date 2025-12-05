// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/testdata"
)

// insertOldData will insert any OldData from the MultiDBState into the
// appropriate rawDB. This prepares the rawDB for the test comparing schema and
// data and any changes to rows.
func insertOldData(ctx context.Context, mdbs *testdata.MultiDBState, rawDBs map[string]storagenodedb.DBContainer) error {
	for dbName, dbState := range mdbs.DBStates {
		if dbState.OldData == "" {
			continue
		}

		rawDB, ok := rawDBs[dbName]
		if !ok {
			return errs.New("Failed to find DB %s", dbName)
		}
		_, err := rawDB.GetDB().ExecContext(ctx, dbState.OldData)
		if err != nil {
			return err
		}
	}
	return nil
}

// insertNewData will insert any NewData from the MultiDBState into the
// appropriate rawDB. This prepares the rawDB for the test comparing schema and
// data. It will not insert NewData if OldData is set: the migration is expected
// to convert OldData into what NewData would insert.
func insertNewData(ctx context.Context, mdbs *testdata.MultiDBState, rawDBs map[string]storagenodedb.DBContainer) error {
	for dbName, dbState := range mdbs.DBStates {
		if dbState.NewData == "" || dbState.OldData != "" {
			continue
		}

		rawDB, ok := rawDBs[dbName]
		if !ok {
			return errs.New("Failed to find DB %s", dbName)
		}
		_, err := rawDB.GetDB().ExecContext(ctx, dbState.NewData)
		if err != nil {
			return err
		}
	}
	return nil
}

// getSchemas queries the schema of each rawDB and returns a map of each rawDB's
// schema keyed by dbName.
func getSchemas(ctx context.Context, rawDBs map[string]storagenodedb.DBContainer) (map[string]*dbschema.Schema, error) {
	schemas := make(map[string]*dbschema.Schema)
	for dbName, rawDB := range rawDBs {
		db := rawDB.GetDB()
		if db == nil {
			continue
		}

		schema, err := sqliteutil.QuerySchema(ctx, rawDB.GetDB())
		if err != nil {
			return nil, err
		}

		// we don't care changes in versions table
		schema.DropTable("versions")

		schemas[dbName] = schema
	}
	return schemas, nil
}

// getSchemas queries the data of each rawDB and returns a map of each rawDB's
// data keyed by dbName.
func getData(ctx context.Context, rawDBs map[string]storagenodedb.DBContainer, schemas map[string]*dbschema.Schema) (map[string]*dbschema.Data, error) {
	data := make(map[string]*dbschema.Data)
	for dbName, rawDB := range rawDBs {
		db := rawDB.GetDB()
		if db == nil {
			continue
		}

		datum, err := sqliteutil.QueryData(ctx, rawDB.GetDB(), schemas[dbName])
		if err != nil {
			return nil, err
		}

		data[dbName] = datum
	}
	return data, nil
}

func TestMigrate(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	storageDir := ctx.Dir("storage")
	cfg := storagenodedb.Config{
		Pieces:    storageDir,
		Storage:   storageDir,
		Info:      filepath.Join(storageDir, "piecestore.db"),
		Info2:     filepath.Join(storageDir, "info.db"),
		Filestore: filestore.DefaultConfig,
	}

	// create a new satellitedb connection
	db, err := storagenodedb.OpenNew(ctx, log, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	rawDBs := db.RawDatabases()

	// get migration for this database
	migrations := db.Migration(ctx)
	for i, step := range migrations.Steps {
		// the schema is different when migration step is before the step, cannot test the layout
		tag := fmt.Sprintf("#%d - v%d", i, step.Version)

		// find the matching expected version
		expected, ok := testdata.States.FindVersion(step.Version)
		require.True(t, ok)

		// insert old data for any tables
		err = insertOldData(ctx, expected, rawDBs)
		require.NoError(t, err, tag)

		// run migration up to a specific version
		err := migrations.TargetVersion(step.Version).Run(ctx, log.Named("migrate"))
		require.NoError(t, err, tag)

		// insert data for new tables
		err = insertNewData(ctx, expected, rawDBs)
		require.NoError(t, err, tag)

		// load schema from database
		schemas, err := getSchemas(ctx, rawDBs)
		require.NoError(t, err, tag)

		// load data from database
		data, err := getData(ctx, rawDBs, schemas)
		require.NoError(t, err, tag)

		multiDBSnapshot, err := testdata.LoadMultiDBSnapshot(ctx, expected)
		require.NoError(t, err, tag)

		// verify schema and data for each db in the expected snapshot
		for dbName, dbSnapshot := range multiDBSnapshot.DBSnapshots {
			// If the tables and indexes of the schema are empty, that's
			// semantically the same as nil. Set to nil explicitly to help with
			// comparison to snapshot.
			schema, ok := schemas[dbName]
			if ok && len(schema.Tables) == 0 {
				schema.Tables = nil
			}
			if ok && len(schema.Indexes) == 0 {
				schema.Indexes = nil
			}

			require.Equal(t, dbSnapshot.Schema, schemas[dbName], tag)
			require.Equal(t, dbSnapshot.Data, data[dbName], tag)

			// verify schema for last migration step matches expected production schema
			if i == len(migrations.Steps)-1 {
				prodSchema := storagenodedb.Schema()[dbName]
				require.Equal(t, dbSnapshot.Schema, prodSchema, tag)
			}
		}
	}
}
