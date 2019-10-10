// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"strconv"
	"strings"
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
	schemaSuffix := pgutil.CreateRandomTestingSchemaName(6)
	t.Log("schema-suffix ", schemaSuffix)

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.MasterDB.Name, func(t *testing.T) {
			t.Parallel()

			// postgres has a maximum schema length of 64
			// we need additional 6 bytes for the random suffix
			//    and 4 bytes for the satellite index "/S0/""
			const MaxTestNameLength = 64 - 6 - 4

			testname := t.Name()
			if len(testname) > MaxTestNameLength {
				testname = testname[:MaxTestNameLength]
			}

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}

			planetConfig := config
			planetConfig.Reconfigure.NewSatelliteDB = func(log *zap.Logger, index int) (satellite.DB, error) {
				schema := strings.ToLower(testname + "/S" + strconv.Itoa(index) + "/" + schemaSuffix)
				db, err := satellitedb.New(log, pgutil.ConnstrWithSchema(satelliteDB.MasterDB.URL, schema))
				if err != nil {
					t.Fatal(err)
				}

				err = db.CreateSchema(schema)
				if err != nil {
					t.Fatal(err)
				}

				return &satelliteSchema{
					DB:     db,
					schema: schema,
				}, nil
			}

			if satelliteDB.PointerDB.URL != "" {
				planetConfig.Reconfigure.NewSatellitePointerDB = func(log *zap.Logger, index int) (metainfo.PointerDB, error) {
					schema := strings.ToLower(testname + "/P" + strconv.Itoa(index) + "/" + schemaSuffix)

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

			// make sure nodes are refreshed in db
			planet.Satellites[0].Discovery.Service.Refresh.TriggerWait()

			test(t, ctx, planet)
		})
	}
}

// satelliteSchema closes database and drops the associated schema
type satelliteSchema struct {
	satellite.DB
	schema string
}

// Close closes the database and drops the schema.
func (db *satelliteSchema) Close() error {
	return errs.Combine(
		db.DB.DropSchema(db.schema),
		db.DB.Close(),
	)
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
