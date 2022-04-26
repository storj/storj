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
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/satellite/accounting"
	adminui "storj.io/storj/satellite/admin/ui"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Config defines configuration for debug server.
type Config struct {
	Address   string `help:"admin peer http listening address" releaseDefault:"" devDefault:""`
	StaticDir string `help:"an alternate directory path which contains the static assets to serve. When empty, it uses the embedded assets" releaseDefault:"" devDefault:""`

	AuthorizationToken string `internal:"true"`

	ConsoleConfig console.Config
}

// DB is databases needed for the admin server.
type DB interface {
	// ProjectAccounting returns database for storing information about project data use
	ProjectAccounting() accounting.ProjectAccounting
	// Console returns database for satellite console
	Console() console.DB
	// OIDC returns the database for OIDC and OAuth information.
	OIDC() oidc.DB
	// StripeCoinPayments returns database for satellite stripe coin payments
	StripeCoinPayments() stripecoinpayments.DB
}

// Server provides endpoints for administrative tasks.
type Server struct {
	log *zap.Logger

	listener net.Listener
	server   http.Server

	db       DB
	payments payments.Accounts
	buckets  *buckets.Service
	restKeys *restkeys.Service

	nowFn func() time.Time

	config Config
}

// NewServer returns a new administration Server.
func NewServer(log *zap.Logger, listener net.Listener, db DB, buckets *buckets.Service, restKeys *restkeys.Service, accounts payments.Accounts, config Config) *Server {
	server := &Server{
		log: log,

		listener: listener,

		db:       db,
		payments: accounts,
		buckets:  buckets,
		restKeys: restKeys,

		nowFn: time.Now,

		config: config,
	}

	root := mux.NewRouter()

	api := root.PathPrefix("/api/").Subrouter()
	api.Use(allowedAuthorization(config.AuthorizationToken))

	// When adding new options, also update README.md
	api.HandleFunc("/users", server.addUser).Methods("POST")
	api.HandleFunc("/users/{useremail}", server.updateUser).Methods("PUT")
	api.HandleFunc("/users/{useremail}", server.userInfo).Methods("GET")
	api.HandleFunc("/users/{useremail}", server.deleteUser).Methods("DELETE")
	api.HandleFunc("/oauth/clients", server.createOAuthClient).Methods("POST")
	api.HandleFunc("/oauth/clients/{id}", server.updateOAuthClient).Methods("PUT")
	api.HandleFunc("/oauth/clients/{id}", server.deleteOAuthClient).Methods("DELETE")
	api.HandleFunc("/projects", server.addProject).Methods("POST")
	api.HandleFunc("/projects/{project}/usage", server.checkProjectUsage).Methods("GET")
	api.HandleFunc("/projects/{project}/limit", server.getProjectLimit).Methods("GET")
	api.HandleFunc("/projects/{project}/limit", server.putProjectLimit).Methods("PUT", "POST")
	api.HandleFunc("/projects/{project}", server.getProject).Methods("GET")
	api.HandleFunc("/projects/{project}", server.renameProject).Methods("PUT")
	api.HandleFunc("/projects/{project}", server.deleteProject).Methods("DELETE")
	api.HandleFunc("/projects/{project}/apikeys", server.listAPIKeys).Methods("GET")
	api.HandleFunc("/projects/{project}/apikeys", server.addAPIKey).Methods("POST")
	api.HandleFunc("/projects/{project}/apikeys/{name}", server.deleteAPIKeyByName).Methods("DELETE")
	api.HandleFunc("/projects/{project}/buckets/{bucket}", server.getBucketInfo).Methods("GET")
	api.HandleFunc("/projects/{project}/buckets/{bucket}/geofence", server.createGeofenceForBucket).Methods("POST")
	api.HandleFunc("/projects/{project}/buckets/{bucket}/geofence", server.deleteGeofenceForBucket).Methods("DELETE")
	api.HandleFunc("/apikeys/{apikey}", server.deleteAPIKey).Methods("DELETE")
	api.HandleFunc("/restkeys/{useremail}", server.addRESTKey).Methods("POST")
	api.HandleFunc("/restkeys/{apikey}/revoke", server.revokeRESTKey).Methods("PUT")

	// This handler must be the last one because it uses the root as prefix,
	// otherwise will try to serve all the handlers set after this one.
	if config.StaticDir == "" {
		root.PathPrefix("/").Handler(http.FileServer(http.FS(adminui.Assets))).Methods("GET")
	} else {
		root.PathPrefix("/").Handler(http.FileServer(http.Dir(config.StaticDir))).Methods("GET")
	}

	server.server.Handler = root
	return server
}

// Run starts the admin endpoint.
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

// SetNow allows tests to have the server act as if the current time is whatever they want.
func (server *Server) SetNow(nowFn func() time.Time) {
	server.nowFn = nowFn
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return Error.Wrap(server.server.Close())
}

func allowedAuthorization(token string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				sendJSONError(w, "Authorization not enabled.",
					"", http.StatusForbidden)
				return
			}

			equality := subtle.ConstantTimeCompare(
				[]byte(r.Header.Get("Authorization")),
				[]byte(token),
			)
			if equality != 1 {
				sendJSONError(w, "Forbidden",
					"", http.StatusForbidden)
				return
			}

			r.Header.Set("Cache-Control", "must-revalidate")
			next.ServeHTTP(w, r)
		})
	}
}
