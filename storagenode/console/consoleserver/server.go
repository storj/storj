// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleapi"
	"storj.io/storj/storagenode/heldamount"
	"storj.io/storj/storagenode/notifications"
)

var (
	mon = monkit.Package()
	// Error is storagenode console web error type.
	Error = errs.Class("storagenode console web error")
)

// Config contains configuration for storagenode console web server.
type Config struct {
	Address   string `help:"server address of the api gateway and frontend app" default:"127.0.0.1:14002"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents storagenode console web server.
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	service       *console.Service
	notifications *notifications.Service
	heldAmount    *heldamount.Service
	listener      net.Listener

	server http.Server
}

// NewServer creates new instance of storagenode console web server.
func NewServer(logger *zap.Logger, assets http.FileSystem, notifications *notifications.Service, service *console.Service, heldAmount *heldamount.Service, listener net.Listener) *Server {
	server := Server{
		log:           logger,
		service:       service,
		listener:      listener,
		notifications: notifications,
		heldAmount:    heldAmount,
	}

	router := mux.NewRouter()

	// handle api endpoints
	storageNodeController := consoleapi.NewStorageNode(server.log, server.service)
	storageNodeRouter := router.PathPrefix("/api/sno").Subrouter()
	storageNodeRouter.StrictSlash(true)
	storageNodeRouter.HandleFunc("/", storageNodeController.StorageNode).Methods(http.MethodGet)
	storageNodeRouter.HandleFunc("/satellites", storageNodeController.Satellites).Methods(http.MethodGet)
	storageNodeRouter.HandleFunc("/satellite/{id}", storageNodeController.Satellite).Methods(http.MethodGet)

	notificationController := consoleapi.NewNotifications(server.log, server.notifications)
	notificationRouter := router.PathPrefix("/api/notifications").Subrouter()
	notificationRouter.StrictSlash(true)
	notificationRouter.HandleFunc("/list", notificationController.ListNotifications).Methods(http.MethodGet)
	notificationRouter.HandleFunc("/{id}/read", notificationController.ReadNotification).Methods(http.MethodPost)
	notificationRouter.HandleFunc("/readall", notificationController.ReadAllNotifications).Methods(http.MethodPost)

	heldAmountController := consoleapi.NewHeldAmount(server.log, server.heldAmount)
	heldAmountRouter := router.PathPrefix("/api/heldamount").Subrouter()
	heldAmountRouter.StrictSlash(true)
	heldAmountRouter.HandleFunc("/paystubs/{period}", heldAmountController.PayStubMonthly).Methods(http.MethodGet)
	heldAmountRouter.HandleFunc("/paystubs/{start}/{end}", heldAmountController.PayStubPeriod).Methods(http.MethodGet)
	heldAmountRouter.HandleFunc("/heldback/{id}", heldAmountController.HeldbackHistory).Methods(http.MethodGet)

	if assets != nil {
		fs := http.FileServer(assets)
		router.PathPrefix("/static/").Handler(server.cacheMiddleware(http.StripPrefix("/static", fs)))
		router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := r.Clone(r.Context())
			req.URL.Path = "/dist/"
			fs.ServeHTTP(w, req)
		}))
	}

	server.server = http.Server{
		Handler: router,
	}

	return &server
}

// Run starts the server that host webapp and api endpoints.
func (server *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return server.server.Shutdown(context.Background())
	})
	group.Go(func() error {
		defer cancel()
		err := server.server.Serve(server.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
	})

	return group.Wait()
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return server.server.Close()
}

// cacheMiddleware is a middleware for caching static files.
func (server *Server) cacheMiddleware(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "mime" package, which http.FileServer uses, depends on Operating System
		// configuration for mime-types. When a system has hardcoded mime-types to
		// something else, they might serve ".js" as a "plain/text".
		//
		// Override any of that default behavior to ensure we get the correct types for
		// common files.
		if contentType, ok := CommonContentType(filepath.Ext(r.URL.Path)); ok {
			w.Header().Set("Content-Type", contentType)
		}

		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		fn.ServeHTTP(w, r)
	})
}
