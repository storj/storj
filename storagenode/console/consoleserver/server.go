// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/private/web"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleapi"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/payouts"
)

var (
	mon = monkit.Package()
	// Error is storagenode console web error type.
	Error = errs.Class("consoleserver")
)

// Config contains configuration for storagenode console web server.
type Config struct {
	Address   string `help:"server address of the api gateway and frontend app" default:"127.0.0.1:14002" testDefault:"$HOST:0"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents storagenode console web server.
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	service       *console.Service
	notifications *notifications.Service
	payout        *payouts.Service
	listener      net.Listener
	assets        fs.FS

	server http.Server
}

// NewServer creates new instance of storagenode console web server.
func NewServer(logger *zap.Logger, assets fs.FS, notifications *notifications.Service, service *console.Service, payout *payouts.Service, listener net.Listener) *Server {
	server := Server{
		log:           logger,
		service:       service,
		listener:      listener,
		assets:        assets,
		notifications: notifications,
		payout:        payout,
	}

	router := mux.NewRouter()

	// handle api endpoints
	storageNodeController := consoleapi.NewStorageNode(server.log, server.service)
	storageNodeRouter := router.PathPrefix("/api/sno").Subrouter()
	storageNodeRouter.StrictSlash(true)
	storageNodeRouter.HandleFunc("/", storageNodeController.StorageNode).Methods(http.MethodGet)
	storageNodeRouter.HandleFunc("/satellites", storageNodeController.Satellites).Methods(http.MethodGet)
	storageNodeRouter.HandleFunc("/satellite/{id}", storageNodeController.Satellite).Methods(http.MethodGet)
	storageNodeRouter.HandleFunc("/satellites/{id}/pricing", storageNodeController.Pricing).Methods(http.MethodGet)
	storageNodeRouter.HandleFunc("/estimated-payout", storageNodeController.EstimatedPayout).Methods(http.MethodGet)

	notificationController := consoleapi.NewNotifications(server.log, server.notifications)
	notificationRouter := router.PathPrefix("/api/notifications").Subrouter()
	notificationRouter.StrictSlash(true)
	notificationRouter.HandleFunc("/list", notificationController.ListNotifications).Methods(http.MethodGet)
	notificationRouter.HandleFunc("/{id}/read", notificationController.ReadNotification).Methods(http.MethodPost)
	notificationRouter.HandleFunc("/readall", notificationController.ReadAllNotifications).Methods(http.MethodPost)

	payoutController := consoleapi.NewPayout(server.log, server.payout)
	payoutRouter := router.PathPrefix("/api/heldamount").Subrouter()
	payoutRouter.StrictSlash(true)
	payoutRouter.HandleFunc("/paystubs/{period}", payoutController.PayStubMonthly).Methods(http.MethodGet)
	payoutRouter.HandleFunc("/paystubs/{start}/{end}", payoutController.PayStubPeriod).Methods(http.MethodGet)
	payoutRouter.HandleFunc("/held-history", payoutController.HeldHistory).Methods(http.MethodGet)
	payoutRouter.HandleFunc("/periods", payoutController.HeldAmountPeriods).Methods(http.MethodGet)
	payoutRouter.HandleFunc("/payout-history/{period}", payoutController.PayoutHistory).Methods(http.MethodGet)

	staticServer := http.FileServer(http.FS(server.assets))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", web.CacheHandler(staticServer)))
	router.PathPrefix("/").HandlerFunc(server.appHandler)

	server.server = http.Server{
		Handler: router,
	}

	return &server
}

// appHandler is web app http handler function.
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	header.Set("Content-Type", "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")

	f, err := server.assets.Open("index.html")
	if err != nil {
		http.Error(w, `web/storagenode unbuilt, run "npm install && npm run build" in web/storagenode.`, http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	_, _ = io.Copy(w, f)
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
