// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Error is satellite marketing error type
var (
	Error       = errs.Class("satellite marketing error")
	_, fp, _, _ = runtime.Caller(0)
	dir         = strings.Split(fp, "server.go")[0]
)

// Config contains configuration for marketing offersweb server
type Config struct {
	Address string `help:"server address of the marketing Admin GUI" default:"0.0.0.0:8090"`
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
func addPages(assets []string) []string {
	rp := dir + "pages/"
	pages := []string{rp + "base.html", rp + "index.html", rp + "banner.html"}
	for _, page := range assets {
		pages = append(pages, page)
	}
	return pages
}

// NewServer creates new instance of offersweb server
func NewServer(logger *zap.Logger, config Config, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		config:   config,
		listener: listener,
	}

	logger.Sugar().Debugf("Starting Marketing Admin UI on %s...", server.listener.Addr().String())
	mux := mux.NewRouter()
	s := http.StripPrefix("/static/", http.FileServer(http.Dir(dir+"/static/")))
	mux.PathPrefix("/static/").Handler(s)
	mux.Handle("/", http.HandlerFunc(server.appHandler))

	server.server = http.Server{
		Handler: mux,
	}

	return &server
}

// appHandler is web app http handler function
func (s *Server) appHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveError(w, req)
		return
	}

	rp := dir + "pages/"
	pages := []string{rp + "home.html", rp + "refOffers.html", rp + "freeOffers.html", rp + "roModal.html", rp + "foModal.html"}
	files := addPages(pages)
	home := template.Must(template.New("landingPage").ParseFiles(files...))
	err := home.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.serveError(w, req)
	}
}

func (s *Server) serveError(w http.ResponseWriter, req *http.Request) {
	rp := dir + "pages/"
	unavailable := template.Must(template.New("404").ParseFiles(rp + "404.html"))
	err := unavailable.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.serveError(w, req)
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
