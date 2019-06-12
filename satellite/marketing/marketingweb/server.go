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
}

// The three pages contained in addPages are pages all templates require
// This exists in order to limit handler verbosity
func (s *Server) addPages(assets []string) []string {
	rp := s.config.StaticDir + "/pages/"
	pages := []string{rp + "base.html", rp + "index.html", rp + "banner.html"}
	for _, page := range assets {
		pages = append(pages, page)
	}
	return pages
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

	return s
}

// ServeHTTP handles index request
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveInternalError(w, req)
		return
	}

	rp := s.config.StaticDir + "/pages/"
	pages := []string{rp + "home.html", rp + "refOffers.html", rp + "freeOffers.html", rp + "roModal.html", rp + "foModal.html"}
	files := s.addPages(pages)

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

func (s *Server) serveInternalError(w http.ResponseWriter, req *http.Request) {
	rp := s.config.StaticDir + "/pages/"
	files := s.addPages([]string{rp + "internal-server-error.html"})

	unavailable, err := template.New("internal-server-error").ParseFiles(files...)
	if err != nil {
		s.serveInternalError(w, req)
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
