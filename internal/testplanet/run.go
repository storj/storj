// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"strconv"
	"testing"

	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"

	"storj.io/storj/internal/testcontext"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	for _, satelliteDB := range satellitedbtest.Databases() {
		t.Run(satelliteDB.Name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if satelliteDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.Name, satelliteDB.Message)
			}

			planet, err := NewCustom(zaptest.NewLogger(t), Config{
				SatelliteCount:   config.SatelliteCount,
				StorageNodeCount: config.StorageNodeCount,
				UplinkCount:      config.UplinkCount,

				Reconfigure: Reconfigure{
					NewSatelliteDB: func(index int) (satellite.DB, error) {
						db, err := satellitedb.New(satelliteDB.URL)
						if err != nil {
							t.Fatal(err)
						}

						schema := satelliteDB.Name + "-" + strconv.Itoa(index)

						err = db.SetSchema(schema)
						if err != nil {
							t.Fatal(err)
						}

						return &satelliteSchema{
							DB:     db,
							schema: schema,
						}, err
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Check(planet.Shutdown)

			planet.Start(ctx)
			test(t, ctx, planet)
		})
	}
}

// satelliteSchema closes database and drops the associated schema
type satelliteSchema struct {
	satellite.DB
	schema string
}

func (db *satelliteSchema) Close() error {
	return errs.Combine(
		db.DB.DropSchema(db.schema),
		db.DB.Close(),
	)
}
