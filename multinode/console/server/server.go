// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/multinode/console/controllers"
	"storj.io/storj/multinode/nodes"
)

var (
	// Error is an error class for internal Multinode Dashboard http server error.
	Error = errs.Class("multinode console server error")
)

// Config contains configuration for Multinode Dashboard http server.
type Config struct {
	Address   string `json:"address" help:"server address of the api gateway and frontend app" default:"127.0.0.1:15002"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents Multinode Dashboard http server.
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	config Config
	nodes  *nodes.Service

	listener net.Listener
	http     http.Server
}

// NewServer returns new instance of Multinode Dashboard http server.
func NewServer(log *zap.Logger, config Config, nodes *nodes.Service, listener net.Listener) (*Server, error) {
	server := Server{
		log:      log,
		config:   config,
		nodes:    nodes,
		listener: listener,
	}

	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v0").Subrouter()
	apiRouter.NotFoundHandler = controllers.NewNotFound(server.log)

	nodesController := controllers.NewNodes(server.log, server.nodes)
	nodesRouter := apiRouter.PathPrefix("/nodes").Subrouter()
	nodesRouter.HandleFunc("", nodesController.Add).Methods(http.MethodPost)
	nodesRouter.HandleFunc("", nodesController.List).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/{id}", nodesController.Get).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/{id}", nodesController.UpdateName).Methods(http.MethodPatch)
	nodesRouter.HandleFunc("/{id}", nodesController.Delete).Methods(http.MethodDelete)

	server.http = http.Server{
		Handler: router,
	}

	return &server, nil
}

// Run starts the server that host webapp and api endpoints.
func (server *Server) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)

	var group errgroup.Group

	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(server.http.Shutdown(context.Background()))
	})
	group.Go(func() error {
		defer cancel()
		return Error.Wrap(server.http.Serve(server.listener))
	})

	return Error.Wrap(group.Wait())
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return Error.Wrap(server.http.Close())
}
