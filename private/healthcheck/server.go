// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
)

var mon = monkit.Package()

var (
	// Error class for this package.
	Error = errs.Class("healthcheck")
	// ErrCheckExists is returned when a check with the same name already exists.
	ErrCheckExists = Error.New("check with name already exists")
)

// HealthCheck is an interface that defines the methods for a health check.
type HealthCheck interface {
	// Healthy returns true if the service is healthy.
	Healthy(ctx context.Context) bool
	// Name returns the name of the service being checked.
	Name() string
}

// Config is the configuration for healthcheck server.
type Config struct {
	Enabled bool   `help:"Whether the health check server is enabled" default:"false"`
	Address string `help:"The address to listen on for health check server" default:"localhost:10500" testDefault:"$HOST:0"`
}

// Server handles HTTP request for health Server.
type Server struct {
	log *zap.Logger

	checks map[string]HealthCheck

	listener net.Listener
	server   http.Server
}

// NewServer creates a new HTTP Server.
func NewServer(log *zap.Logger, listener net.Listener, checks ...HealthCheck) *Server {
	checkMap := make(map[string]HealthCheck, len(checks))
	for _, check := range checks {
		checkMap[check.Name()] = check
	}
	srv := &Server{
		log:      log,
		listener: listener,
		checks:   checkMap,
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", srv.handleAllHTTP)
	router.HandleFunc("/health/{name}", srv.handleSingleHTTP)

	srv.server = http.Server{
		Handler: router,
	}

	return srv
}

// AddCheck adds a health check to the server.
func (s *Server) AddCheck(check HealthCheck) error {
	if _, ok := s.checks[check.Name()]; ok {
		return ErrCheckExists
	}
	s.checks[check.Name()] = check

	return nil
}

func (s *Server) handleAllHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	checkMap := make(map[string]bool, len(s.checks))
	allHealthy := true
	for name, check := range s.checks {
		healthy := check.Healthy(ctx)
		allHealthy = allHealthy && healthy
		checkMap[name] = healthy
	}
	if allHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	err = json.NewEncoder(w).Encode(checkMap)
	if err != nil {
		s.log.Error("Failed to encode health check response", zap.Error(err))
	}
}

func (s *Server) handleSingleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var name string
	var ok bool
	if name, ok = mux.Vars(r)["name"]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(map[string]string{"error": "missing name parameter"})
		if err != nil {
			s.log.Error("Failed to encode health check response", zap.Error(err))
		}
		return
	}

	var check HealthCheck
	if check, ok = s.checks[name]; !ok {
		w.WriteHeader(http.StatusNotFound)
		err = json.NewEncoder(w).Encode(map[string]string{"error": "unknown check name"})
		if err != nil {
			s.log.Error("Failed to encode health check response", zap.Error(err))
		}
		return
	}

	healthy := check.Healthy(ctx)
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	err = json.NewEncoder(w).Encode(map[string]bool{"healthy": healthy})
	if err != nil {
		s.log.Error("Failed to encode health check response", zap.Error(err))
	}
}

// Run starts the health check server.
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return s.server.Shutdown(context.Background())
	})
	group.Go(func() error {
		defer cancel()
		err := s.server.Serve(s.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
	})

	return group.Wait()
}

// Close stops the server.
func (s *Server) Close() error {
	return s.server.Close()
}

// TestGetAddress returns the address of this server for tests.
func (s *Server) TestGetAddress() string {
	return s.listener.Addr().String()
}
