// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/prometheus/common/log"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/storagenode/console"
)

// TODO: improve embedded resources generation
//go-bindata -pkg operatorserver -o storagenode/operator/operatorserver/static.go web/operator/dist web/operator/dist/public/

// Error is storagenode operator web error type
var Error = errs.Class("storagenode operator web error")

// Config contains configuration for storagenode operator web server
type Config struct {
	Address string `help:"server address of the api gateway and frontend app" default:"127.0.0.1:14002"`
}

// Server represents storagenode operator web server
type Server struct {
	log *zap.Logger

	config   Config
	service  *console.Service
	listener net.Listener

	server http.Server

	staticDir string
}

// NewServer creates new instance of storagenode operator web server
func NewServer(logger *zap.Logger, config Config, service *console.Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		service:  service,
		config:   config,
		listener: listener,
	}

	mux := http.NewServeMux()

	server.staticDir = "web/operator/"

	mux.Handle("/", http.HandlerFunc(server.appHandler))
	mux.Handle("/static/", http.HandlerFunc(server.appStaticHandler))

	server.server = http.Server{
		Handler: mux,
	}

	return &server
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

// appHandler is an entry point for storagenode operator web interface
func (s *Server) appHandler(w http.ResponseWriter, req *http.Request) {
	data, err := Asset(s.staticDir + "dist/public/index.html")
	if err != nil {
		log.Error(err)
		return
	}

	_, err = w.Write(data)
	if err != nil {
		log.Error(err)
		return
	}
}

// appStaticHandler is needed to return static resources
func (s *Server) appStaticHandler(w http.ResponseWriter, req *http.Request) {
	resourceName := strings.TrimPrefix(req.RequestURI, "/static/")

	data, err := Asset(s.staticDir + resourceName)
	if err != nil {
		log.Error(err)
		return
	}

	_, err = w.Write(data)
	if err != nil {
		log.Error(err)
		return
	}
}
