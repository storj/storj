// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/storj/multinode/console"
)

var (
	mon = monkit.Package()

	// Error is an error class for internal Multinode Dashboard http server error.
	Error = errs.Class("multinode console server error")
	// ErrNodesAPI - console nodes api error type.
	ErrNodesAPI = errs.Class("multinode console nodes api error")
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
	service *console.Service

	listener net.Listener
	http     http.Server
}

// NewServer returns new instance of Multinode Dashboard http server.
func NewServer(log *zap.Logger, config Config, service *console.Service, listener net.Listener) (*Server, error) {
	server := Server{
		log:      log,
		config:   config,
		service:  service,
		listener: listener,
	}

	router := mux.NewRouter()
	router.StrictSlash(true)

	apiRouter := router.PathPrefix("/api/v0").Subrouter()
	apiRouter.HandleFunc("/nodes", server.addNodeHandler).Methods(http.MethodPost)
	apiRouter.HandleFunc("/nodes/{nodeID}", server.removeNodeHandler).Methods(http.MethodDelete)

	server.http = http.Server{
		Handler: router,
	}

	return &server, nil
}

// addNodeHandler handles node addition.
func (server *Server) addNodeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error

	defer mon.Task()(&ctx)(&err)

	var data struct {
		ID            string
		APISecret     string
		PublicAddress string
	}

	if err = json.NewDecoder(r.Body).Decode(&data); err != nil {
		server.serveJSONError(w, http.StatusBadRequest, ErrNodesAPI.Wrap(err))
		return
	}

	id, err := storj.NodeIDFromString(data.ID)
	if err != nil {
		server.serveJSONError(w, http.StatusBadRequest, ErrNodesAPI.Wrap(err))
		return
	}

	apiSecret, err := console.APISecretFromBase64(data.APISecret)
	if err != nil {
		server.serveJSONError(w, http.StatusBadRequest, ErrNodesAPI.Wrap(err))
		return
	}

	if err = server.service.AddNode(ctx, id, apiSecret, data.PublicAddress); err != nil {
		server.serveJSONError(w, http.StatusInternalServerError, ErrNodesAPI.Wrap(err))
		return
	}
}

// removeNodeHandler handles node removal.
func (server *Server) removeNodeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error

	defer mon.Task()(&ctx)(&err)

	vars := mux.Vars(r)

	nodeID, err := storj.NodeIDFromString(vars["nodeID"])
	if err != nil {
		server.serveJSONError(w, http.StatusBadRequest, ErrNodesAPI.Wrap(err))
		return
	}

	if err = server.service.RemoveNode(ctx, nodeID); err != nil {
		server.serveJSONError(w, http.StatusNotFound, ErrNodesAPI.Wrap(err))
		return
	}
}

// serveJSONError writes error to response in json format.
func (server *Server) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		server.log.Error("failed to write json error response", zap.Error(err))
	}
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
