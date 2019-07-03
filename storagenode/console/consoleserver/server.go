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

	"storj.io/storj/storagenode/nodestats"

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

// DashboardResponse stores data and error message
type DashboardResponse struct {
	Data  DashboardData `json:"data"`
	Error string        `json:"error,omitempty"`
}

// DashboardData stores all needed information about storagenode
type DashboardData struct {
	Bandwidth          console.BandwidthInfo       `json:"bandwidth"`
	DiskSpace          console.DiskSpaceInfo       `json:"diskSpace"`
	WalletAddress      string                      `json:"walletAddress"`
	VersionInfo        version.Info                `json:"versionInfo"`
	IsLastVersion      bool                        `json:"isLastVersion"`
	Uptime             time.Duration               `json:"uptime"`
	NodeID             string                      `json:"nodeId"`
	Satellites         storj.NodeIDList            `json:"satellites"`
	UptimeCheck        *nodestats.UptimeCheck      `json:"uptimeCheck"`
	BandwidthChartData []console.BandwidthUsed     `json:"bandwidthChartData"`
	DiskSpaceChartData []nodestats.SpaceUsageStamp `json:"diskSpaceChartData"`
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
		mux.Handle("/api/update/", http.HandlerFunc(server.liveReloadHandler))
	}

	server.server = http.Server{
		Handler: mux,
	}

	server.upgrader = websocket.Upgrader{}

	return &server
}

// Run starts the server that host webapp and api endpoints
func (server *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return server.server.Shutdown(nil)
	})
	group.Go(func() error {
		defer cancel()
		return server.server.Serve(server.listener)
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (server *Server) Close() error {
	return server.server.Close()
}

// appHandler is web app http handler function
func (server *Server) appHandler(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, filepath.Join(server.config.StaticDir, "dist", "index.html"))
}

// appHandler is web app http handler function
func (server *Server) dashboardHandler(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	defer mon.Task()(&ctx)(nil)
	writer.Header().Set(contentType, applicationJSON)

	var response = DashboardResponse{}

	defer func() {
		err := json.NewEncoder(writer).Encode(&response)
		if err != nil {
			server.log.Error(err.Error())
		}
	}()

	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	satelliteIDParam := request.URL.Query().Get("satelliteId")
	satelliteID, err := server.parseSatelliteIDParam(satelliteIDParam)
	if err != nil {
		server.log.Error("satellite id is not valid", zap.Error(err))
		response.Error = "satellite id is not valid"
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	data, err := server.getDashboardData(ctx, satelliteID)
	if err != nil {
		server.log.Error("can not get dashboard data", zap.Error(err))
		response.Error = err.Error()
	}

	response.Data = data

	writer.WriteHeader(http.StatusOK)
}

// appHandler is web app http handler function
func (server *Server) liveReloadHandler(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	defer mon.Task()(&ctx)(nil)

	var response = &DashboardResponse{}

	conn, err := server.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		server.log.Error("\nwebsocket error: ", zap.Error(err))
	}

	var satelliteID *storj.NodeID

	go func(conn *websocket.Conn) {
		for {
			_, satelliteIDParam, err := conn.ReadMessage()
			if err != nil {
				if err = conn.Close(); err != nil {
					server.log.Error("can not close websocket connection", zap.Error(err))
				}
			}

			satelliteID, err = server.parseSatelliteIDParam(string(satelliteIDParam))
			if err != nil {
				server.log.Error("\nsatellite id is not valid", zap.Error(err))
				response.Error = "satellite id is not valid"
			}
		}
	}(conn)

	go func(conn *websocket.Conn) {
		ch := time.Tick(5 * time.Second)

		for range ch {
			data, err := server.getDashboardData(context.Background(), satelliteID)
			if err != nil {
				server.log.Error("can not get dashboard data", zap.Error(err))
				response.Error = err.Error()
			}

			response.Data = data

			if err = conn.WriteJSON(response); err != nil {
				server.log.Error("can not send data", zap.Error(err))
			}
		}
	}(conn)
}

func (server *Server) getDashboardData(ctx context.Context, satelliteID *storj.NodeID) (DashboardData, error) {
	var response = DashboardData{}

	satellites, err := server.service.GetSatellites(ctx)
	if err != nil {
		return response, err
	}

	// checks if current satellite id is related to current storage node
	if satelliteID != nil {
		if err = server.checkSatelliteID(satellites, *satelliteID); err != nil {
			return response, err
		}
	}

	space, err := server.getStorage(ctx, satelliteID)
	if err != nil {
		return response, err
	}

	usage, err := server.getBandwidth(ctx, satelliteID)
	if err != nil {
		return response, err
	}

	walletAddress := server.service.GetWalletAddress(ctx)

	versionInfo := server.service.GetVersion(ctx)

	err = server.service.CheckVersion(ctx)
	if err != nil {
		return response, err
	}

	bandwidthChartData, err := server.getBandwidthChartData(ctx, satelliteID)
	if err != nil {
		return response, err
	}

	diskSpaceChartData, err := server.getDiskSpaceChartData(ctx, satelliteID, satellites)
	if err != nil {
		return response, err
	}

	uptime := server.service.GetUptime(ctx)
	nodeID := server.service.GetNodeID(ctx)

	if satelliteID != nil {
		response.UptimeCheck, err = server.service.GetUptimeCheckForSatellite(ctx, *satelliteID)
		if err != nil {
			return response, err
		}
	}

	response.DiskSpace = *space
	response.Bandwidth = *usage
	response.WalletAddress = walletAddress
	response.VersionInfo = versionInfo
	response.IsLastVersion = true
	response.Uptime = uptime
	response.NodeID = nodeID.String()
	response.Satellites = satellites
	response.BandwidthChartData = bandwidthChartData
	response.DiskSpaceChartData = diskSpaceChartData

	return response, nil
}

func (server *Server) getBandwidth(ctx context.Context, satelliteID *storj.NodeID) (_ *console.BandwidthInfo, err error) {
	if satelliteID != nil {
		return server.service.GetBandwidthBySatellite(ctx, *satelliteID)
	}

	return server.service.GetUsedBandwidthTotal(ctx)
}

func (server *Server) getBandwidthChartData(ctx context.Context, satelliteID *storj.NodeID) (_ []console.BandwidthUsed, err error) {
	from, to := getMonthRange()

	if satelliteID != nil {
		return server.service.GetDailyBandwidthUsed(ctx, *satelliteID, from, to)
	}

	return server.service.GetDailyTotalBandwidthUsed(ctx, from, to)
}

func (server *Server) getDiskSpaceChartData(ctx context.Context, satelliteID *storj.NodeID, satellitesID storj.NodeIDList) (result []nodestats.SpaceUsageStamp, err error) {
	from, to := getMonthRange()

	if satelliteID != nil {
		return server.service.GetDailyStorageUsedForSatellite(ctx, *satelliteID, from, to)
	}

	if satellitesID == nil || satellitesID.Len() == 0 {
		return nil, nil
	}

	for _, id := range satellitesID {
		usage, err := server.service.GetDailyStorageUsedForSatellite(ctx, id, from, to)
		if err != nil {
			return nil, err
		}

		if len(result) == 0 {
			result = usage
		} else {
			for i := 0; i < len(result); i++ {
				for j := 0; j < len(usage); j++ {
					if result[i].TimeStamp == usage[j].TimeStamp {
						result[i].AtRestTotal += usage[j].AtRestTotal
					}
				}
			}
		}
	}

	return result, nil
}

func (server *Server) getStorage(ctx context.Context, satelliteID *storj.NodeID) (_ *console.DiskSpaceInfo, err error) {
	if satelliteID != nil {
		return server.service.GetUsedStorageBySatellite(ctx, *satelliteID)
	}

	return server.service.GetUsedStorageTotal(ctx)
}

func (server *Server) checkSatelliteID(satelliteIDs storj.NodeIDList, satelliteID storj.NodeID) error {
	for _, id := range satelliteIDs {
		if satelliteID == id {
			return nil
		}
	}

	return errs.New("satellite id is not found in the available satellite list")
}

func (server *Server) parseSatelliteIDParam(satelliteID string) (*storj.NodeID, error) {
	if satelliteID != "" {
		id, err := storj.NodeIDFromString(satelliteID)
		return &id, err
	}

	return nil, nil
}

// getMonthRange is used to get first and last dates of month
func getMonthRange() (firstDay, lastDay time.Time) {
	now := time.Now()
	currentYear, currentMonth, _ := now.Date()
	currentLocation := now.Location()

	firstDay = time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastDay = firstDay.AddDate(0, 1, -1)

	return
}
