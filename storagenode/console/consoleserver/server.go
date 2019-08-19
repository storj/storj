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

// Error is storagenode console web error type.
var (
	mon   = monkit.Package()
	Error = errs.Class("storagenode console web error")
)

// Config contains configuration for storagenode console web server.
type Config struct {
	Address   string `help:"server address of the api gateway and frontend app" default:"127.0.0.1:14002"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents storagenode console web server.
type Server struct {
	log *zap.Logger

	config   Config
	service  *console.Service
	listener net.Listener

	server http.Server
}

// NewServer creates new instance of storagenode console web server.
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

// Run starts the server that host webapp and api endpoints.
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

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return server.server.Close()
}

// appHandler is web app http handler function.
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(server.config.StaticDir, "dist", "index.html"))
}

// dashboardHandler handles dashboard API requests.
func (server *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := server.service.GetDashboardData(ctx)
	if err != nil {
		server.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	server.writeData(w, data)
}

// satelliteHandler handles satellites API request.
func (server *Server) satellitesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := server.service.GetAllSatellitesData(ctx)
	if err != nil {
		server.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	server.writeData(w, data)
}

// satelliteHandler handles satellite API requests.
func (server *Server) satelliteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	satelliteID, err := storj.NodeIDFromString(strings.TrimLeft(r.URL.Path, "/api/satellite/"))
	if err != nil {
		server.writeError(w, http.StatusBadRequest, Error.Wrap(err))
		return
	}

	if err = server.service.VerifySatelliteID(ctx, satelliteID); err != nil {
		server.writeError(w, http.StatusNotFound, Error.Wrap(err))
		return
	}

	data, err := server.service.GetSatelliteData(ctx, satelliteID)
	if err != nil {
		server.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	server.writeData(w, data)
}

// jsonOutput defines json structure of api response data.
type jsonOutput struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// writeData is helper method to write JSON to http.ResponseWriter and log encoding error.
func (server *Server) writeData(w http.ResponseWriter, data interface{}) {
	w.Header().Set(contentType, applicationJSON)
	w.WriteHeader(http.StatusOK)

	output := jsonOutput{Data: data}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		server.log.Error("json encoder error", zap.Error(err))
	}
}

// writeError writes a JSON error payload to http.ResponseWriter log encoding error.
func (server *Server) writeError(w http.ResponseWriter, status int, err error) {
	if status >= http.StatusInternalServerError {
		server.log.Error("api handler server error", zap.Int("status code", status), zap.Error(err))
	}

	w.Header().Set(contentType, applicationJSON)
	w.WriteHeader(status)

	output := jsonOutput{Error: err.Error()}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		server.log.Error("json encoder error", zap.Error(err))
	}
}
