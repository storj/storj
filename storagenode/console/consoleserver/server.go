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

	"github.com/gorilla/websocket"
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

	server   http.Server
	upgrader websocket.Upgrader
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

	server.upgrader = websocket.Upgrader{}

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
func (s *Server) appHandler(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, filepath.Join(s.config.StaticDir, "dist", "index.html"))
}

// appHandler is web app http handler function
func (s *Server) dashboardHandler(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	defer mon.Task()(&ctx)(nil)
	writer.Header().Set(contentType, applicationJSON)

	var response struct {
		Data struct {
			Bandwidth          console.BandwidthInfo   `json:"bandwidth"`
			DiskSpace          console.DiskSpaceInfo   `json:"diskSpace"`
			WalletAddress      string                  `json:"walletAddress"`
			VersionInfo        version.Info            `json:"versionInfo"`
			IsLastVersion      bool                    `json:"isLastVersion"`
			Uptime             time.Duration           `json:"uptime"`
			NodeID             string                  `json:"nodeId"`
			Satellites         storj.NodeIDList        `json:"satellites"`
			BandwidthChartData []console.BandwidthUsed `json:"bandwidthChartData"`
		} `json:"data"`
		Error string `json:"error,omitempty"`
	}

	defer func() {
		err := json.NewEncoder(writer).Encode(&response)
		if err != nil {
			s.log.Error(err.Error())
		}
	}()

	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	satelliteIDParam := request.URL.Query().Get("satelliteId")
	satelliteID, err := s.parseSatelliteIDParam(satelliteIDParam)
	if err != nil {
		s.log.Error("satellite id is not valid", zap.Error(err))
		response.Error = "satellite id is not valid"
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	satellites, err := s.service.GetSatellites(ctx)
	if err != nil {
		s.log.Error("can not get satellites list", zap.Error(err))
		response.Error = "can not get satellites list"
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	// checks if current satellite id is related to current storage node
	if satelliteID != nil {
		if err = s.checkSatelliteID(satellites, *satelliteID); err != nil {
			s.log.Error(err.Error())
			response.Error = err.Error()
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	space, err := s.getStorage(ctx, satelliteID)
	if err != nil {
		s.log.Error("can not get disk space usage", zap.Error(err))
		response.Error = "can not get disk space usage"
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	usage, err := s.getBandwidth(ctx, satelliteID)
	if err != nil {
		s.log.Error("can not get bandwidth usage", zap.Error(err))
		response.Error = "can not get bandwidth usage"
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	walletAddress := s.service.GetWalletAddress(ctx)

	versionInfo := s.service.GetVersion(ctx)

	err = s.service.CheckVersion(ctx)
	if err != nil {
		s.log.Error("can not check latest storage node version", zap.Error(err))
		response.Error = "can not check latest storage node version"
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	bandwidthChartData, err := s.getBandwidthChartData(ctx, satelliteID)
	if err != nil {
		s.log.Error("can not get bandwidth usage for chart", zap.Error(err))
		response.Error = "can not get bandwidth usage for chart"
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	uptime := s.service.GetUptime(ctx)
	nodeID := s.service.GetNodeID(ctx)

	response.Data.DiskSpace = *space
	response.Data.Bandwidth = *usage
	response.Data.WalletAddress = walletAddress
	response.Data.VersionInfo = versionInfo
	response.Data.IsLastVersion = true
	response.Data.Uptime = uptime
	response.Data.NodeID = nodeID.String()
	response.Data.Satellites = satellites
	response.Data.BandwidthChartData = bandwidthChartData

	writer.WriteHeader(http.StatusOK)
}

// appHandler is web app http handler function
func (s *Server) liveReloadHandler(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	defer mon.Task()(&ctx)(nil)
	writer.Header().Set(contentType, applicationJSON)

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
		Error string `json:"error,omitempty"`
	}

	defer func() {
		err := json.NewEncoder(writer).Encode(&response)
		if err != nil {
			s.log.Error(err.Error())
		}
	}()

	var conn, _ = s.upgrader.Upgrade(writer, request, nil)

	go func(conn *websocket.Conn) {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				conn.Close()
			}
		}
	}(conn)

	//go func(conn *websocket.Conn) {
	//	ch := time.Tick(5 * time.Second)
	//
	//	for range ch {
	//		conn.WriteJSON(myStruct{
	//			Username:  "mvansickle",
	//			FirstName: "Michael",
	//			LastName:  "Van Sickle",
	//		})
	//	}
	//}(conn)

	writer.WriteHeader(http.StatusOK)
}

func (s *Server) getBandwidth(ctx context.Context, satelliteID *storj.NodeID) (_ *console.BandwidthInfo, err error) {
	if satelliteID != nil {
		return s.service.GetBandwidthBySatellite(ctx, *satelliteID)
	}

	return s.service.GetUsedBandwidthTotal(ctx)
}

func (s *Server) getBandwidthChartData(ctx context.Context, satelliteID *storj.NodeID) (_ []console.BandwidthUsed, err error) {
	if satelliteID != nil {
		return s.service.GetDailyBandwidthUsed(ctx, *satelliteID, time.Now(), time.Now())
	}

	return s.service.GetDailyTotalBandwidthUsed(ctx, time.Now(), time.Now())
}

func (s *Server) getStorage(ctx context.Context, satelliteID *storj.NodeID) (_ *console.DiskSpaceInfo, err error) {
	if satelliteID != nil {
		return s.service.GetUsedStorageBySatellite(ctx, *satelliteID)
	}

	return s.service.GetUsedStorageTotal(ctx)
}

func (s *Server) checkSatelliteID(satelliteIDs storj.NodeIDList, satelliteID storj.NodeID) error {
	for _, id := range satelliteIDs {
		if satelliteID == id {
			return nil
		}
	}

	return errs.New("satellite id in not found in the available satellite list")
}

func (s *Server) parseSatelliteIDParam(satelliteID string) (*storj.NodeID, error) {
	if satelliteID != "" {
		id, err := storj.NodeIDFromString(satelliteID)
		return &id, err
	}

	return nil, nil
}
