// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/console"
)

const (
	contentType = "Content-Type"

	applicationJSON = "application/json"
)

// Error is storagenode console web error type
var (
	mon   = monkit.Package()
	Error = errs.Class("storagenode console web error")
)

// Config contains configuration for storagenode console web server
type Config struct {
	Address   string `help:"server address of the api gateway and frontend app" default:"127.0.0.1:14002"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents storagenode console web server
type Server struct {
	log *zap.Logger

	config   Config
	service  *console.Service
	listener net.Listener

	server http.Server
}

// NewServer creates new instance of storagenode console web server
func NewServer(logger *zap.Logger, config Config, service *console.Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		service:  service,
		config:   config,
		listener: listener,
	}

	var fs http.Handler
	mux := http.NewServeMux()

	// handle static pages
	if config.StaticDir != "" {
		fs = http.FileServer(http.Dir(server.config.StaticDir))

		mux.Handle("/static/", http.StripPrefix("/static", fs))
		mux.Handle("/", http.HandlerFunc(server.appHandler))
	}

	// handle api endpoints
	mux.Handle("/api/dashboard", http.HandlerFunc(server.dashboardHandler))
	mux.Handle("/api/satellites", http.HandlerFunc(server.satellitesHandler))
	mux.Handle("/api/satellite/", http.HandlerFunc(server.satelliteHandler))

	server.server = http.Server{
		Handler: mux,
	}

	return &server
}

// Run starts the server that host webapp and api endpoints
func (server *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return server.server.Shutdown(nil)
	})
	group.Go(func() error {
		defer cancel()
		return server.server.Serve(server.listener)
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (server *Server) Close() error {
	return server.server.Close()
}

// appHandler is web app http handler function
func (server *Server) appHandler(wr http.ResponseWriter, req *http.Request) {
	http.ServeFile(wr, req, filepath.Join(server.config.StaticDir, "dist", "index.html"))
}

// dashboardHandler handles dashboard api requests
func (server *Server) dashboardHandler(wr http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	defer mon.Task()(&ctx)(nil)

	if req.Method != http.MethodGet {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := server.service.GetDashboardData(ctx)
	if err != nil {
		server.writeError(wr, http.StatusInternalServerError, err)
		return
	}

	server.writeData(wr, data)
}

// satelliteHandler handles satellite api request
func (server *Server) satellitesHandler(wr http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	defer mon.Task()(&ctx)(nil)

	if req.Method != http.MethodGet {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := server.service.GetAllSatellitesData(ctx)
	if err != nil {
		server.writeError(wr, http.StatusInternalServerError, err)
		return
	}

	server.writeData(wr, data)
}

// satelliteHandler handles satellite api requests
func (server *Server) satelliteHandler(wr http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	defer mon.Task()(&ctx)(nil)

	if req.Method != http.MethodGet {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	satelliteID, err := storj.NodeIDFromString(strings.TrimLeft(req.URL.Path, "/api/satellite/"))
	if err != nil {
		server.writeError(wr, http.StatusBadRequest, err)
		return
	}

	if err = server.service.VerifySatelliteID(ctx, satelliteID); err != nil {
		server.writeError(wr, http.StatusNotFound, err)
		return
	}

	data, err := server.service.GetSatelliteData(ctx, satelliteID)
	if err != nil {
		server.writeError(wr, http.StatusInternalServerError, err)
		return
	}

	server.writeData(wr, data)
}

// jsonOutput defines json structure of api response data
type jsonOutput struct {
	Data  interface{} `json:"data"`
	Error string      `json:"err"`
}

// writeData is helper method to write JSON to http.ResponseWriter and log encoding error
func (server Server) writeData(wr http.ResponseWriter, data interface{}) {
	wr.Header().Set(contentType, applicationJSON)
	wr.WriteHeader(http.StatusOK)

	output := jsonOutput{Data: data}

	if err := json.NewEncoder(wr).Encode(output); err != nil {
		server.log.Error(err.Error())
	}
}

// helper method to write JSON error to http.ResponseWriter and log encoding error
func (server Server) writeError(wr http.ResponseWriter, status int, err error) {
	wr.Header().Set(contentType, applicationJSON)
	wr.WriteHeader(status)

	output := jsonOutput{Error: err.Error()}

	if err := json.NewEncoder(wr).Encode(output); err != nil {
		server.log.Error(err.Error())
	}
}
