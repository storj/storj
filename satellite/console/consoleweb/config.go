// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/satellite/console"

	"storj.io/storj/pkg/provider"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
)

// Config contains info needed for satellite account related services
type Config struct {
	GatewayConfig
	SatelliteAddr string `help:"satellite main endpoint" default:""`
	DatabaseURL   string `help:"" default:"sqlite3://$CONFDIR/satellitedb.db"`
}

// Run implements Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	log := zap.NewExample()

	db, ok := ctx.Value("masterdb").(interface{ ConsoleDB() console.DB })
	if !ok {
		return errs.New("unable to get satellite master db instance")
	}

	service, err := console.NewService(
		log,
		&consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")},
		db.ConsoleDB(),
	)

	if err != nil {
		return err
	}

	creator := consoleql.TypeCreator{}
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
		log:     log,
		schema:  schema,
		service: service,
		config:  c.GatewayConfig,
	}).run()

	return server.Run(ctx)
}
