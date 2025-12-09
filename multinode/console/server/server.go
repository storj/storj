// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/multinode/bandwidth"
	"storj.io/storj/multinode/console/controllers"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/multinode/operators"
	"storj.io/storj/multinode/payouts"
	"storj.io/storj/multinode/reputation"
	"storj.io/storj/multinode/storage"
	"storj.io/storj/private/web"
)

var (
	// Error is an error class for internal Multinode Dashboard http server error.
	Error = errs.Class("multinode console server")
)

// Config contains configuration for Multinode Dashboard http server.
type Config struct {
	Address   string `json:"address" help:"server address of the api gateway and frontend app" default:"127.0.0.1:15002" testDefault:"$HOST:0"`
	StaticDir string `help:"path to static resources" default:""`
}

// Services contains services utilized by multinode dashboard.
type Services struct {
	Nodes      *nodes.Service
	Payouts    *payouts.Service
	Operators  *operators.Service
	Storage    *storage.Service
	Bandwidth  *bandwidth.Service
	Reputation *reputation.Service
}

// Server represents Multinode Dashboard http server.
//
// architecture: Endpoint
type Server struct {
	log      *zap.Logger
	listener net.Listener
	http     http.Server
	assets   fs.FS

	nodes      *nodes.Service
	payouts    *payouts.Service
	operators  *operators.Service
	bandwidth  *bandwidth.Service
	storage    *storage.Service
	reputation *reputation.Service
}

// NewServer returns new instance of Multinode Dashboard http server.
func NewServer(log *zap.Logger, listener net.Listener, assets fs.FS, services Services) (*Server, error) {
	server := Server{
		log:        log,
		listener:   listener,
		assets:     assets,
		nodes:      services.Nodes,
		operators:  services.Operators,
		payouts:    services.Payouts,
		storage:    services.Storage,
		bandwidth:  services.Bandwidth,
		reputation: services.Reputation,
	}

	router := mux.NewRouter()

	apiRouter := router.PathPrefix("/api/v0").Subrouter()
	apiRouter.NotFoundHandler = controllers.NewNotFound(server.log)

	nodesController := controllers.NewNodes(server.log, server.nodes)
	nodesRouter := apiRouter.PathPrefix("/nodes").Subrouter()
	nodesRouter.HandleFunc("", nodesController.Add).Methods(http.MethodPost)
	nodesRouter.HandleFunc("/infos", nodesController.ListInfos).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/infos/{satelliteID}", nodesController.ListInfosSatellite).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/trusted-satellites", nodesController.TrustedSatellites).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/{id}", nodesController.Get).Methods(http.MethodGet)
	nodesRouter.HandleFunc("/{id}", nodesController.UpdateName).Methods(http.MethodPatch)
	nodesRouter.HandleFunc("/{id}", nodesController.Delete).Methods(http.MethodDelete)

	operatorsController := controllers.NewOperators(server.log, server.operators)
	operatorsRouter := apiRouter.PathPrefix("/operators").Subrouter()
	operatorsRouter.HandleFunc("", operatorsController.ListPaginated).Methods(http.MethodGet)

	bandwidthController := controllers.NewBandwidth(server.log, server.bandwidth)
	bandwidthRouter := apiRouter.PathPrefix("/bandwidth").Subrouter()
	bandwidthRouter.HandleFunc("/", bandwidthController.Monthly).Methods(http.MethodGet)
	bandwidthRouter.HandleFunc("/{nodeID}", bandwidthController.MonthlyNode).Methods(http.MethodGet)
	bandwidthRouter.HandleFunc("/satellites/{id}", bandwidthController.MonthlySatellite).Methods(http.MethodGet)
	bandwidthRouter.HandleFunc("/satellites/{id}/{nodeID}", bandwidthController.MonthlySatelliteNode).Methods(http.MethodGet)

	payoutsController := controllers.NewPayouts(server.log, server.payouts)
	payoutsRouter := apiRouter.PathPrefix("/payouts").Subrouter()
	payoutsRouter.HandleFunc("/summaries", payoutsController.Summary).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/summaries/{period}", payoutsController.SummaryPeriod).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/expectations", payoutsController.Expectations).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/expectations/{nodeID}", payoutsController.NodeExpectations).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/paystubs/{nodeID}", payoutsController.Paystub).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/paystubs/{period}/{nodeID}", payoutsController.PaystubPeriod).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/total-earned", payoutsController.Earned).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/held-amounts/{nodeID}", payoutsController.HeldAmountSummary).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/satellites/{id}/summaries", payoutsController.SummarySatellite).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/satellites/{id}/summaries/{period}", payoutsController.SummarySatellitePeriod).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/satellites/{id}/paystubs/{nodeID}", payoutsController.PaystubSatellite).Methods(http.MethodGet)
	payoutsRouter.HandleFunc("/satellites/{id}/paystubs/{period}/{nodeID}", payoutsController.PaystubSatellitePeriod).Methods(http.MethodGet)

	storageController := controllers.NewStorage(server.log, server.storage)
	storageRouter := apiRouter.PathPrefix("/storage").Subrouter()
	storageRouter.HandleFunc("/usage", storageController.TotalUsage).Methods(http.MethodGet)
	storageRouter.HandleFunc("/usage/{nodeID}", storageController.Usage).Methods(http.MethodGet)
	storageRouter.HandleFunc("/satellites/{satelliteID}/usage", storageController.TotalUsageSatellite).Methods(http.MethodGet)
	storageRouter.HandleFunc("/satellites/{satelliteID}/usage/{nodeID}", storageController.UsageSatellite).Methods(http.MethodGet)
	storageRouter.HandleFunc("/disk-space", storageController.TotalDiskSpace).Methods(http.MethodGet)
	storageRouter.HandleFunc("/disk-space/{nodeID}", storageController.DiskSpace).Methods(http.MethodGet)

	reputationController := controllers.NewReputation(server.log, server.reputation)
	reputationRouter := apiRouter.PathPrefix("/reputation").Subrouter()
	reputationRouter.HandleFunc("/satellites/{satelliteID}", reputationController.Stats)

	staticServer := http.FileServer(http.FS(server.assets))
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", web.CacheHandler(staticServer)))
	router.PathPrefix("/").HandlerFunc(server.appHandler)

	server.http = http.Server{
		Handler: router,
	}

	return &server, nil
}

// appHandler is web app http handler function.
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	header.Set("Content-Type", "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")

	f, err := server.assets.Open("index.html")
	if err != nil {
		http.Error(w, `web/multinode unbuilt, run "npm install && npm run build" in web/multinode.`, http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	_, _ = io.Copy(w, f)
}

// Run starts the server that host webapp and api endpoints.
func (server *Server) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group

	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(server.http.Shutdown(context.Background()))
	})
	group.Go(func() error {
		defer cancel()
		err := Error.Wrap(server.http.Serve(server.listener))
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
	})

	return Error.Wrap(group.Wait())
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return Error.Wrap(server.http.Close())
}
