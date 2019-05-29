// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"runtime"
	"strings"

	"github.com/gorilla/schema"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"storj.io/storj/satellite/marketing"
)

var (
	// Error is satellite marketing error type
	Error       = errs.Class("satellite marketing error")
	_, fp, _, _ = runtime.Caller(0)
	dir         = strings.Split(fp, "server.go")[0]
	decoder     = schema.NewDecoder()
)

// Config contains configuration for marketingweb server
type Config struct {
	Address string `help:"server address of the frontend app" default:"0.0.0.0:8090"`
}

// Server represents marketingweb server
type Server struct {
	log *zap.Logger

	config Config

	listener net.Listener
	server   http.Server
	service  *marketing.Service
}

// NewServer creates new instance of marketingweb server
func NewServer(logger *zap.Logger, config Config, service *marketing.Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		config:   config,
		listener: listener,
		service:  service,
	}

	logger.Sugar().Debugf("Starting marketingweb UI...", server.listener.Addr().String())

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
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

	switch req.Method {
	case http.MethodGet:
		// Serve the resource.
		s.getHandler(w, req)
	case http.MethodPost:
		// Create a new record.
		s.createHandler(w, req)
	case http.MethodPut:
		// Update an existing record.
		s.updateHandler(w, req)
	default:
		// Give an error message.
		s.serveError(w, req)
	}
}

func (s *Server) getHandler(w http.ResponseWriter, req *http.Request) {
	offers, err := s.service.ListAllOffers(context.Background())
	if err != nil {
		s.log.Error("app handler error", zap.Error(err))

		s.serveError(w, req)
		return
	}

	home := template.Must(template.New("landingPage").ParseFiles(dir+"pages/base.html", dir+"pages/index.html", dir+"pages/home.html"))
	home.ExecuteTemplate(w, "base", offers)
}

func (s *Server) createHandler(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	var o marketing.NewOffer
	err := decoder.Decode(&o, req.PostForm)
	if err != nil {
		s.log.Error("createdHandler error", zap.Error(err))

		s.serveError(w, req)
		return
	}

	n, err := s.service.CreateOffer(context.Background(), &o)
	if err != nil {
		s.log.Error("createdHandler error", zap.Error(err))

		s.serveError(w, req)
		return
	}

	// TODO: return the new offer and display it on the page
	fmt.Println(n)
}

func (s *Server) updateHandler(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	var o marketing.UpdateOffer
	err := decoder.Decode(&o, req.PostForm)
	if err != nil {
		s.log.Error("createdHandler error", zap.Error(err))

		s.serveError(w, req)
		return
	}

	err = s.service.UpdateOffer(context.Background(), &o)
	if err != nil {
		s.log.Error("createdHandler error", zap.Error(err))

		s.serveError(w, req)
		return
	}

	// TODO: update success response
}

func (s *Server) serveError(w http.ResponseWriter, req *http.Request) {
	unavailable := template.Must(template.New("404").ParseFiles(dir+"pages/base.html", dir+"pages/index.html", dir+"pages/404.html"))
	unavailable.ExecuteTemplate(w, "base", nil)
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
