// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapweb

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"

	"github.com/graphql-go/graphql"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	contentType = "Content-Type"

	applicationJSON    = "application/json"
	applicationGraphql = "application/graphql"
)

// Error is satellite console error type
var Error = errs.Class("bootstrap console error")

// Config contains configuration for bootstrap web server
type Config struct {
	Address   string `help:"server address of the graphql api gateway and frontend app" default:"127.0.0.1:8082"`
	StaticDir string `help:"path to static resources" default:"./web/bootstrap"`
}

// Server represents bootstrap web server
type Server struct {
	log *zap.Logger

	config   Config
	service  *Service
	listener net.Listener

	schema graphql.Schema
	server http.Server
}

// NewServer creates new instance of console server
func NewServer(logger *zap.Logger, config Config, service *Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		service:  service,
		config:   config,
		listener: listener,
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(server.config.StaticDir))

	mux.Handle("/api/graphql/v0", http.HandlerFunc(server.grapqlHandler))

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

// grapqlHandler is graphql endpoint http handler function
func (s *Server) grapqlHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(contentType, applicationJSON)

	query, err := getQuery(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := graphql.Do(graphql.Params{
		Schema:         s.schema,
		Context:        context.Background(),
		RequestString:  query.Query,
		VariableValues: query.Variables,
		OperationName:  query.OperationName,
		RootObject:     make(map[string]interface{}),
	})

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		s.log.Error(err.Error())
		return
	}

	sugar := s.log.Sugar()
	sugar.Debug(result)
}

// Run starts the server that host webapp and api endpoint
func (s *Server) Run(ctx context.Context) error {
	var err error
	s.schema, err = CreateSchema(s.service)
	if err != nil {
		return Error.Wrap(err)
	}

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
