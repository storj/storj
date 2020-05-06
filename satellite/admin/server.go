// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package admin implements administrative endpoints for satellite.
package admin

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

// Config defines configuration for debug server.
type Config struct {
	Address string `help:"admin peer http listening address" releaseDefault:"" devDefault:""`

	AuthorizationToken string `internal:"true"`
}

// DB is databases needed for the admin server.
type DB interface {
	// ProjectAccounting returns database for storing information about project data use
	ProjectAccounting() accounting.ProjectAccounting
	// Console returns database for satellite console
	Console() console.DB
}

// Server provides endpoints for debugging.
type Server struct {
	log *zap.Logger

	listener net.Listener
	server   http.Server
	mux      *mux.Router

	db DB
}

// NewServer returns a new debug.Server.
func NewServer(log *zap.Logger, listener net.Listener, db DB, config Config) *Server {
	server := &Server{
		log: log,
	}

	server.db = db
	server.listener = listener
	server.mux = mux.NewRouter()
	server.server.Handler = &protectedServer{
		allowedAuthorization: config.AuthorizationToken,
		next:                 server.mux,
	}

	// When adding new options, also update README.md
	server.mux.HandleFunc("/api/user/{useremail}", server.userInfo).Methods("GET")
	server.mux.HandleFunc("/api/project/{project}/limit", server.getProjectLimit).Methods("GET")
	server.mux.HandleFunc("/api/project/{project}/limit", server.putProjectLimit).Methods("PUT", "POST")

	return server
}

type protectedServer struct {
	allowedAuthorization string

	next http.Handler
}

func (server *protectedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if server.allowedAuthorization == "" {
		http.Error(w, "Authorization not enabled.", http.StatusForbidden)
		return
	}

	equality := subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("Authorization")),
		[]byte(server.allowedAuthorization),
	)
	if equality != 1 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	r.Header.Set("Cache-Control", "must-revalidate")

	server.next.ServeHTTP(w, r)
}

// Run starts the debug endpoint.
func (server *Server) Run(ctx context.Context) error {
	if server.listener == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(server.server.Shutdown(context.Background()))
	})
	group.Go(func() error {
		defer cancel()
		err := server.server.Serve(server.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return Error.Wrap(err)
	})
	return group.Wait()
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return Error.Wrap(server.server.Close())
}
