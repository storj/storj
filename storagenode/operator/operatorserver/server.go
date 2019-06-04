// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operatorserver

import (
	"context"
	"net"
	"net/http"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/storagenode/operator"
)

// Error is storagenode operator web error type
var Error = errs.Class("storagenode operator web error")

// Config contains configuration for storagenode operator web server
type Config struct {
	Address   string `help:"server address of the graphql api gateway and frontend app" default:"127.0.0.1:14002"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents storagenode operator web server
type Server struct {
	log *zap.Logger

	config   Config
	service  *operator.Service
	listener net.Listener

	server http.Server
}

// NewServer creates new instance of storagenode operator web server
func NewServer(logger *zap.Logger, config Config, service *operator.Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		service:  service,
		config:   config,
		listener: listener,
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(server.config.StaticDir))

	if server.config.StaticDir != "" {
		mux.Handle("/", http.HandlerFunc(server.appHandler))
		mux.Handle("/static/", http.StripPrefix("/static", fs))
	}

	server.server = http.Server{
		Handler: mux,
	}

	return &server
}

// appHandler is web app http handler function
func (s *Server) appHandler(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, filepath.Join(s.config.StaticDir, "dist", "public", "index.html"))
}

// Run starts the server that host webapp and api endpoints
func (s *Server) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return s.server.Shutdown(nil)
	})
	group.Go(func() error {
		defer cancel()
		return s.server.Serve(s.listener)
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (s *Server) Close() error {
	return s.server.Close()
}
