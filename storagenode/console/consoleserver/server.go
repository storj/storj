// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consolenotifications"
	"storj.io/storj/storagenode/notifications"
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
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	service       *console.Service
	notifications *notifications.Service
	listener      net.Listener

	server http.Server
}

// NewServer creates new instance of storagenode console web server.
func NewServer(logger *zap.Logger, assets http.FileSystem, notifications *notifications.Service, service *console.Service, listener net.Listener) *Server {
	server := Server{
		log:           logger,
		service:       service,
		listener:      listener,
		notifications: notifications,
	}

	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api").Subrouter()
	notificationRouter := router.PathPrefix("/api/notifications").Subrouter()
	notificationController := consolenotifications.NewNotifications(server.log, server.notifications)

	if assets != nil {
		fs := http.FileServer(assets)
		router.PathPrefix("/static/").Handler(server.cacheMiddleware(http.StripPrefix("/static", fs)))
		router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := r.Clone(r.Context())
			req.URL.Path = "/dist/"
			fs.ServeHTTP(w, req)
		}))
	}

	// handle api endpoints
	apiRouter.Handle("/dashboard", http.HandlerFunc(server.dashboardHandler)).Methods(http.MethodGet)
	apiRouter.Handle("/satellites", http.HandlerFunc(server.satellitesHandler)).Methods(http.MethodGet)
	apiRouter.Handle("/satellite/{id}", http.HandlerFunc(server.satelliteHandler)).Methods(http.MethodGet)
	notificationRouter.Handle("/list", http.HandlerFunc(notificationController.ListNotifications)).Methods(http.MethodGet)
	notificationRouter.Handle("/{id}/read", http.HandlerFunc(notificationController.ReadNotification)).Methods(http.MethodPost)
	notificationRouter.Handle("/readall", http.HandlerFunc(notificationController.ReadAllNotifications)).Methods(http.MethodPost)

	server.server = http.Server{
		Handler: router,
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
		return server.server.Shutdown(context.Background())
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
	var err error

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		server.writeError(w, http.StatusBadRequest, Error.Wrap(err))
		return
	}

	satelliteID, err := storj.NodeIDFromString(id)
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

// cacheMiddleware is a middleware for caching static files.
func (server *Server) cacheMiddleware(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		fn.ServeHTTP(w, r)
	})
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
