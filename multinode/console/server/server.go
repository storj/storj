// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/multinode/console/controllers"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/multinode/payouts"
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

	config  Config
	nodes   *nodes.Service
	payouts *payouts.Service

	listener net.Listener
	http     http.Server

	index *template.Template
}

// NewServer returns new instance of Multinode Dashboard http server.
func NewServer(log *zap.Logger, config Config, nodes *nodes.Service, payouts *payouts.Service, listener net.Listener) (*Server, error) {
	server := Server{
		log:      log,
		config:   config,
		nodes:    nodes,
		listener: listener,
		payouts:  payouts,
	}

	router := mux.NewRouter()
	fs := http.FileServer(http.Dir(server.config.StaticDir))

	apiRouter := router.PathPrefix("/api/v0").Subrouter()
	apiRouter.NotFoundHandler = controllers.NewNotFound(server.log)

	nodesController := controllers.NewNodes(server.log, server.nodes)
	nodesRouter := apiRouter.PathPrefix("/nodes").Subrouter()
	nodesRouter.HandleFunc("", nodesController.Add).Methods(http.MethodPost)
	nodesRouter.HandleFunc("/infos", nodesController.ListInfos).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/infos/{satelliteID}", nodesController.ListInfosSatellite).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/trusted-satellites", nodesController.TrustedSatellites).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/{id}", nodesController.Get).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/{id}", nodesController.UpdateName).Methods(http.MethodPatch)
	nodesRouter.HandleFunc("/{id}", nodesController.Delete).Methods(http.MethodDelete)

	payoutsController := controllers.NewPayouts(server.log, server.payouts)
	payoutsRouter := apiRouter.PathPrefix("/payouts").Subrouter()
	payoutsRouter.HandleFunc("/total-earned", payoutsController.GetAllNodesTotalEarned).Methods(http.MethodGet)

	if server.config.StaticDir != "" {
		router.PathPrefix("/static/").Handler(http.StripPrefix("/static", fs))
		router.PathPrefix("/").HandlerFunc(server.appHandler)
	}

	server.http = http.Server{
		Handler: router,
	}

	return &server, nil
}

// appHandler is web app http handler function.
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	header.Set("Content-Type", "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")

	if server.index == nil {
		server.log.Error("index template is not set")
		return
	}

	if err := server.index.Execute(w, nil); err != nil {
		server.log.Error("index template could not be executed", zap.Error(Error.Wrap(err)))
		return
	}
}

// Run starts the server that host webapp and api endpoints.
func (server *Server) Run(ctx context.Context) (err error) {
	err = server.initializeTemplates()
	if err != nil {
		return Error.Wrap(err)
	}

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

// initializeTemplates is used to initialize all templates.
func (server *Server) initializeTemplates() (err error) {
	server.index, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "dist", "index.html"))
	if err != nil {
		server.log.Error("dist folder is not generated. use 'npm run build' command", zap.Error(err))
	}

	return err
}
