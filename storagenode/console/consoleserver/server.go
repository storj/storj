// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/console"
)

const (
	contentType = "Content-Type"

	applicationJSON = "application/json"
)

// Error is storagenode console web error type
var (
	mon   = monkit.Package()
	Error = errs.Class("storagenode console web error")
)

// Config contains configuration for storagenode console web server
type Config struct {
	Address   string `help:"server address of the api gateway and frontend app" default:"127.0.0.1:14002"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents storagenode console web server
type Server struct {
	log *zap.Logger

	config   Config
	service  *console.Service
	listener net.Listener

	server http.Server
}

// NewServer creates new instance of storagenode console web server
func NewServer(logger *zap.Logger, config Config, service *console.Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		service:  service,
		config:   config,
		listener: listener,
	}

	var fs http.Handler
	mux := http.NewServeMux()

	// handle static pages
	if config.StaticDir != "" {
		fs = http.FileServer(http.Dir(server.config.StaticDir))

		mux.Handle("/static/", http.StripPrefix("/static", fs))
		mux.Handle("/", http.HandlerFunc(server.appHandler))
		mux.Handle("/api/dashboard/", http.HandlerFunc(server.dashboardHandler))
	}

	server.server = http.Server{
		Handler: mux,
	}

	return &server
}

// Run starts the server that host webapp and api endpoints
func (s *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return s.server.Shutdown(nil)
	})
	group.Go(func() error {
		defer cancel()
		return s.server.Serve(s.listener)
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (s *Server) Close() error {
	return s.server.Close()
}

// appHandler is web app http handler function
func (s *Server) appHandler(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, filepath.Join(s.config.StaticDir, "dist", "index.html"))
}

// appHandler is web app http handler function
func (s *Server) dashboardHandler(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	defer mon.Task()(&ctx)(nil)
	w.Header().Set(contentType, applicationJSON)

	var response struct {
		Data struct {
			Bandwidth     console.BandwidthInfo `json:"bandwidth"`
			DiskSpace     console.DiskSpaceInfo `json:"diskSpace"`
			WalletAddress string                `json:"walletAddress"`
			VersionInfo   version.Info          `json:"versionInfo"`
			IsLastVersion bool                  `json:"isLastVersion"`
			Uptime        time.Duration         `json:"uptime"`
			NodeID        string                `json:"nodeId"`
			Satellites    storj.NodeIDList      `json:"satellites"`
		} `json:"data"`
		Error         string                `json:"error,omitempty"`
	}

	defer func() {
		err := json.NewEncoder(w).Encode(&response)
		if err != nil {
			s.log.Error(err.Error())
		}
	}()

	space, err := s.service.GetUsedStorageTotal(ctx)
	if err != nil {
		s.log.Error("can not get disk space usage", zap.Error(err))
		response.Error = "can not get disk space usage"
		return
	}

	usage, err := s.service.GetUsedBandwidthTotal(ctx)
	if err != nil {
		s.log.Error("can not get bandwidth usage", zap.Error(err))
		response.Error = "can not get bandwidth usage"
		return
	}

	walletAddress := s.service.GetWalletAddress(ctx)

	versionInfo := s.service.GetVersion(ctx)

	err = s.service.CheckVersion(ctx)
	if err != nil {
		s.log.Error("can not check latest storagenode version", zap.Error(err))
		response.Error = "can not check latest storagenode version"
		return
	}

	uptime := s.service.GetUptime(ctx)

	nodeID := s.service.GetNodeID(ctx)

	satellites, err := s.service.GetSatellites(ctx)
	if err != nil {
		s.log.Error("can not get satellites list", zap.Error(err))
		response.Error = "can not get satellites list"
		return
	}

	response.Data.DiskSpace = *space
	response.Data.Bandwidth = *usage
	response.Data.WalletAddress = walletAddress
	response.Data.VersionInfo = versionInfo
	response.Data.IsLastVersion = true
	response.Data.Uptime = uptime
	response.Data.NodeID = nodeID.String()
	response.Data.Satellites = satellites
}
