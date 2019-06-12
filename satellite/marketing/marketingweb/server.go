// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Error is satellite marketing error type
var Error = errs.Class("satellite marketing error")

// Config contains configuration for marketing offersweb server
type Config struct {
	Address   string `help:"server address of the marketing Admin GUI" default:"127.0.0.1:8090"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents marketing offersweb server
type Server struct {
	log *zap.Logger

	config Config
	
	listener net.Listener
	server   http.Server

	templates *template.Template
}

// NewServer creates new instance of offersweb server
func NewServer(logger *zap.Logger, config Config, listener net.Listener) (*Server, error) {
	s := &Server{
		log:      logger,
		config:   config,
		listener: listener,
	}

	var err error
	s.templates, err = template.ParseGlob(filepath.Join(config.StaticDir, "pages", "*.html"))
	if err != nil {
		return nil, err
	}

	logger.Sugar().Debugf("Starting Marketing Admin UI on %s...", s.listener.Addr().String())

	fs := http.FileServer(http.Dir(s.config.StaticDir))
	mux := http.NewServeMux()
	if s.config.StaticDir != "" {
		mux.Handle("/static/", http.StripPrefix("/static", fs))
		mux.Handle("/", s)
	}
	s.server.Handler = mux

	return s, nil
}

// ServeHTTP is handles 
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveError(w, req)
		return
	}

	err := s.templates.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

func (s *Server) serveError(w http.ResponseWriter, req *http.Request) {
	err := s.templates.ExecuteTemplate(w, "404", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

// Run starts the server that host admin web app and api endpoint
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(s.server.Shutdown(nil))
	})
	group.Go(func() error {
		defer cancel()
		return Error.Wrap(s.server.Serve(s.listener))
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (s *Server) Close() error {
	return Error.Wrap(s.server.Close())
}
