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

type customTemplate struct {
	home          *template.Template
	pageNotFound  *template.Template
	internalError *template.Template
}

// Server represents marketing offersweb server
type Server struct {
	log *zap.Logger

	config Config

	listener net.Listener
	server   http.Server

	templateDir string
	templates   customTemplate
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
func NewServer(logger *zap.Logger, config Config, listener net.Listener) (*Server, error) {
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

	if err := s.parseTemplates(); err != nil {
		return nil, Error.Wrap(err)
	}

	return s, nil
}

// ServeHTTP handles index request
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveNotFound(w, req)
		return
	}

	if s.templates.home == nil {
		s.serveInternalError(w, req)
		return
	}

	err := s.templates.home.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

func (s *Server) parseTemplates() (err error) {

	homeFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "home.html"),
		filepath.Join(s.templateDir, "refOffers.html"),
		filepath.Join(s.templateDir, "freeOffers.html"),
		filepath.Join(s.templateDir, "roModal.html"),
		filepath.Join(s.templateDir, "foModal.html"),
	)

	s.templates.home, err = template.New("landingPage").ParseFiles(homeFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	pageNotFoundFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "page-not-found.html"),
	)

	s.templates.pageNotFound, err = template.New("page-not-found").ParseFiles(pageNotFoundFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	internalErrorFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "internal-server-error.html"),
	)

	s.templates.internalError, err = template.New("internal-server-error").ParseFiles(internalErrorFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (s *Server) serveNotFound(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotFound)

	err := s.templates.pageNotFound.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

func (s *Server) serveInternalError(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)

	err := s.templates.internalError.ExecuteTemplate(w, "base", nil)
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
