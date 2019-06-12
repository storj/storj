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

	templateDir string
}

// commonPages returns templates that are required for everything.
func (s *Server) commonPages() []string {
	return []string{
		filepath.Join(s.templateDir, "base.html"),
		filepath.Join(s.templateDir, "index.html"),
		filepath.Join(s.templateDir, "banner.html"),
	}
}

// NewServer creates new instance of offersweb server
func NewServer(logger *zap.Logger, config Config, listener net.Listener) *Server {
	s := &Server{
		log:      logger,
		config:   config,
		listener: listener,
	}

	logger.Sugar().Debugf("Starting Marketing Admin UI on %s...", s.listener.Addr().String())

	fs := http.FileServer(http.Dir(s.config.StaticDir))
	mux := http.NewServeMux()
	if s.config.StaticDir != "" {
		mux.Handle("/static/", http.StripPrefix("/static", fs))
		mux.Handle("/", s)
	}
	s.server.Handler = mux

	s.templateDir = filepath.Join(s.config.StaticDir, "pages")

	return s
}

// ServeHTTP handles index request
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveNotFound(w, req)
		return
	}

	files := append(s.commonPages(),
		filepath.Join(s.templateDir, "home.html"),
		filepath.Join(s.templateDir, "refOffers.html"),
		filepath.Join(s.templateDir, "freeOffers.html"),
		filepath.Join(s.templateDir, "roModal.html"),
		filepath.Join(s.templateDir, "foModal.html"),
	)

	home, err := template.New("landingPage").ParseFiles(files...)
	if err != nil {
		s.serveInternalError(w, req)
		return
	}

	err = home.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

func (s *Server) serveNotFound(w http.ResponseWriter, req *http.Request) {
	files := append(s.commonPages(),
		filepath.Join(s.templateDir, "page-not-found.html"),
	)

	unavailable, err := template.New("page-not-found").ParseFiles(files...)
	if err != nil {
		s.serveInternalError(w, req)
		return
	}

	w.WriteHeader(http.StatusNotFound)

	err = unavailable.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

func (s *Server) serveInternalError(w http.ResponseWriter, req *http.Request) {
	files := append(s.commonPages(),
		filepath.Join(s.templateDir, "internal-server-error.html"),
	)

	unavailable, err := template.New("internal-server-error").ParseFiles(files...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.log.Error("failed to parse internal server error", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	err = unavailable.ExecuteTemplate(w, "base", nil)
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
