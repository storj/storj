// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
)

// Error is satellite console error type
var Error = errs.Class("satellite console error")

// Server represents console web server
type Server struct {
	config Config
}

// NewServer creates new instance of console server
func NewServer(config Config) *Server {
	return &Server{config: config}
}

// Run implements Responsibility interface
func (s *Server) Run(ctx context.Context) error {
	log := zap.NewExample()

	db, ok := ctx.Value("masterdb").(interface {
		Console() console.DB
	})

	if !ok {
		return Error.Wrap(errs.New("unable to get master db instance"))
	}

	service, err := console.NewService(
		log,
		&consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")},
		db.Console(),
	)

	if err != nil {
		return Error.Wrap(err)
	}

	creator := consoleql.TypeCreator{}
	err = creator.Create(service)
	if err != nil {
		return Error.Wrap(err)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    creator.RootQuery(),
		Mutation: creator.RootMutation(),
	})

	if err != nil {
		return Error.Wrap(err)
	}

	return (&gateway{
		log:     log,
		schema:  schema,
		service: service,
		config:  s.config,
	}).run()
}
