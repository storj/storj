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
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
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
	// StripeCoinPayments returns database for satellite stripe coin payments
	StripeCoinPayments() stripecoinpayments.DB
	// Buckets returns database for satellite buckets
	Buckets() metainfo.BucketsDB
}

// Server provides endpoints for administrative tasks.
type Server struct {
	log *zap.Logger

	listener net.Listener
	server   http.Server
	mux      *mux.Router

	db       DB
	payments payments.Accounts

	nowFn func() time.Time
}

// NewServer returns a new administration Server.
func NewServer(log *zap.Logger, listener net.Listener, db DB, accounts payments.Accounts, config Config) *Server {
	server := &Server{
		log: log,

		listener: listener,
		mux:      mux.NewRouter(),

		db:       db,
		payments: accounts,

		nowFn: time.Now,
	}

	server.server.Handler = &protectedServer{
		allowedAuthorization: config.AuthorizationToken,
		next:                 server.mux,
	}

	// When adding new options, also update README.md
	server.mux.HandleFunc("/api/user", server.addUser).Methods("POST")
	server.mux.HandleFunc("/api/user/{useremail}", server.updateUser).Methods("PUT")
	server.mux.HandleFunc("/api/user/{useremail}", server.userInfo).Methods("GET")
	server.mux.HandleFunc("/api/user/{useremail}", server.deleteUser).Methods("DELETE")
	server.mux.HandleFunc("/api/coupon", server.addCoupon).Methods("POST")
	server.mux.HandleFunc("/api/coupon/{couponid}", server.couponInfo).Methods("GET")
	server.mux.HandleFunc("/api/coupon/{couponid}", server.deleteCoupon).Methods("DELETE")
	server.mux.HandleFunc("/api/project/{project}/usage", server.checkProjectUsage).Methods("GET")
	server.mux.HandleFunc("/api/project/{project}/limit", server.getProjectLimit).Methods("GET")
	server.mux.HandleFunc("/api/project/{project}/limit", server.putProjectLimit).Methods("PUT", "POST")
	server.mux.HandleFunc("/api/project/{project}", server.getProject).Methods("GET")
	server.mux.HandleFunc("/api/project/{project}", server.renameProject).Methods("PUT")
	server.mux.HandleFunc("/api/project/{project}", server.deleteProject).Methods("DELETE")
	server.mux.HandleFunc("/api/project", server.addProject).Methods("POST")

	return server
}

type protectedServer struct {
	allowedAuthorization string

	next http.Handler
}

func (server *protectedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if server.allowedAuthorization == "" {
		httpJSONError(w, "Authorization not enabled.",
			"", http.StatusForbidden)
		return
	}

	equality := subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("Authorization")),
		[]byte(server.allowedAuthorization),
	)
	if equality != 1 {
		httpJSONError(w, "Forbidden",
			"", http.StatusForbidden)
		return
	}

	r.Header.Set("Cache-Control", "must-revalidate")

	server.next.ServeHTTP(w, r)
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
