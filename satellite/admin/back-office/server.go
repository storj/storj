// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Package admin implements a server which serves a REST API and a web application to allow
// performing satellite administration tasks.
//
// NOTE this is work in progress and will eventually replace the current satellite administration
// server implemented in the parent package, hence this package name is the same than its parent
// because it will simplify the replace once it's ready.
package admin

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	ui "storj.io/storj/satellite/admin/back-office/ui"
)

// Error is the error class that wraps all the errors returned by this package.
var Error = errs.Class("satellite-admin")

// Config defines configuration for the satellite administration server.
type Config struct {
	StaticDir string `help:"an alternate directory path which contains the static assets for the satellite administration web app. When empty, it uses the embedded assets" releaseDefault:"" devDefault:""`
}

// Server serves the API endpoints and the web application to allow preforming satellite
// administration tasks.
type Server struct {
	log *zap.Logger

	listener net.Listener
	server   http.Server

	config Config
}

// ParentRouter is mux.Router with its full path prefix.
type ParentRouter struct {
	Router *mux.Router
	// PathPrefix is the full path prefix of Router.
	PathPrefix string
}

// NewServer creates a satellite administration server instance with the provided dependencies and
// configurations.
//
// When listener is nil, Server.Run is a noop.
//
// When parentRouter is nil it creates a new Router to attach the server endpoints, otherwise , it
// attaches them  to the provided one, allowing to expose its functionality through another server.
func NewServer(log *zap.Logger, listener net.Listener, parentRouter *ParentRouter, config Config) *Server {
	server := &Server{
		log:      log,
		listener: listener,
		config:   config,
	}

	if parentRouter == nil {
		parentRouter = &ParentRouter{}
	}

	root := parentRouter.Router
	if root == nil {
		root = mux.NewRouter()
	}

	// API endpoints.
	// api := root.PathPrefix("/api/").Subrouter()

	// Static assets for the web interface.
	// This handler must be the last one because it uses the root as prefix, otherwise, it will serve
	// all the paths defined by the handlers set after this one.
	var staticHandler http.Handler
	if config.StaticDir == "" {
		if parentRouter.PathPrefix != "" {
			staticHandler = http.StripPrefix(parentRouter.PathPrefix, http.FileServer(http.FS(ui.Assets)))
		} else {
			staticHandler = http.FileServer(http.FS(ui.Assets))
		}
	} else {
		if parentRouter.PathPrefix != "" {
			staticHandler = http.StripPrefix(parentRouter.PathPrefix, http.FileServer(http.Dir(config.StaticDir)))
		} else {
			staticHandler = http.FileServer(http.Dir(config.StaticDir))
		}
	}

	root.PathPrefix("/").Handler(staticHandler).Methods("GET")

	return server
}

// Run starts the administration HTTP server using the provided listener.
// If listener is nil, it does nothing and return nil.
func (server *Server) Run(ctx context.Context) error {
	if server.listener == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(server.server.Shutdown(context.Background()))
	})
	group.Go(func() error {
		defer cancel()
		err := server.server.Serve(server.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return Error.Wrap(err)
	})
	return group.Wait()
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return Error.Wrap(server.server.Close())
}
