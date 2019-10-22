// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"testing"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage/postgreskv"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.MasterDB.Name, func(t *testing.T) {
			t.Parallel()

			schemaSuffix := satellitedbtest.SchemaSuffix()
			t.Log("schema-suffix ", schemaSuffix)

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}

			planetConfig := config
			planetConfig.Reconfigure.NewSatelliteDB = func(log *zap.Logger, index int) (satellite.DB, error) {
				schema := satellitedbtest.SchemaName(t.Name(), "S", index, schemaSuffix)

				db, err := satellitedb.New(log, pgutil.ConnstrWithSchema(satelliteDB.MasterDB.URL, schema))
				if err != nil {
					t.Fatal(err)
				}

				return &satellitedbtest.SchemaDB{
					DB:       db,
					Schema:   schema,
					AutoDrop: true,
				}, nil
			}

			if satelliteDB.PointerDB.URL != "" {
				planetConfig.Reconfigure.NewSatellitePointerDB = func(log *zap.Logger, index int) (metainfo.PointerDB, error) {
					schema := satellitedbtest.SchemaName(t.Name(), "P", index, schemaSuffix)

					db, err := postgreskv.New(pgutil.ConnstrWithSchema(satelliteDB.PointerDB.URL, schema))
					if err != nil {
						t.Fatal(err)
					}

					return &satellitePointerSchema{
						Client: db,
						schema: schema,
					}, nil
				}
			}

			planet, err := NewCustom(zaptest.NewLogger(t), planetConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Check(planet.Shutdown)

			planet.Start(ctx)

			test(t, ctx, planet)
		})
	}
}

// satellitePointerSchema closes database and drops the associated schema
type satellitePointerSchema struct {
	*postgreskv.Client
	schema string
}

// Close closes the database and drops the schema.
func (db *satellitePointerSchema) Close() error {
	return errs.Combine(
		db.Client.DropSchema(db.schema),
		db.Client.Close(),
	)
}
