// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteweb

import (
	"context"

	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satelliteauth"

	"go.uber.org/zap"

	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/satellite/satellitedb"
	"storj.io/storj/pkg/satellite/satelliteweb/satelliteql"
	"storj.io/storj/pkg/utils"
)

// Config contains info needed for satellite account related services
type Config struct {
	GatewayConfig
	SatelliteAddr string `help:"satellite main endpoint" default:""`
	DatabaseURL   string `help:"" default:"sqlite3://$CONFDIR/satellitedb.db"`
}

// Run implements Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	sugar := zap.NewExample().Sugar()

	// Create satellite DB
	dbURL, err := utils.ParseURL(c.DatabaseURL)
	if err != nil {
		return err
	}

	db, err := satellitedb.New(dbURL.Scheme, dbURL.Path)
	if err != nil {
		return err
	}

	err = db.CreateTables()
	sugar.Error(err)

	service, err := satellite.NewService(
		&satelliteauth.Hmac{Secret: []byte("my-suppa-secret-key")},
		db,
	)

	if err != nil {
		return err
	}

	creator := satelliteql.TypeCreator{}
	err = creator.Create(service)
	if err != nil {
		return err
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    creator.RootQuery(),
		Mutation: creator.RootMutation(),
	})

	if err != nil {
		return err
	}

	go (&gateway{
		schema: schema,
		config: c.GatewayConfig,
		logger: sugar,
	}).run()

	return server.Run(ctx)
}
