// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/version"
)

// Config is all the configuration parameters for a Version Control Server
type Config struct {
	Address  string `user:"true" help:"public address to listen on" default:":8080"`
	Versions ServiceVersions
}

// ServiceVersions provides a list of allowed Versions per Service
type ServiceVersions struct {
	Bootstrap   string `user:"true" help:"Allowed Bootstrap Versions" default:"v0.1.0,v0.1.1"`
	Satellite   string `user:"true" help:"Allowed Satellite Versions" default:"v0.1.0,v0.1.1"`
	Storagenode string `user:"true" help:"Allowed Storagenode Versions" default:"v0.1.0,v0.1.1"`
	Uplink      string `user:"true" help:"Allowed Uplink Versions" default:"v0.1.0,v0.1.1"`
	Gateway     string `user:"true" help:"Allowed Gateway Versions" default:"v0.1.0,v0.1.1"`
}

// Peer is the representation of a VersionControl Server.
type Peer struct {
	// core dependencies
	Log *zap.Logger

	// Web server
	Server struct {
		Listener net.Listener
	}
	Versions version.AllowedVersions
}

var (
	// response contains the byte version of current allowed versions
	response []byte
)

func handleGet(w http.ResponseWriter, r *http.Request) {
	var xfor string

	// Only handle GET Requests
	if r.Method == "GET" {
		if xfor = r.Header.Get("X-Forwarded-For"); xfor == "" {
			xfor = r.RemoteAddr
		}
		zap.S().Debugf("Request from: %s for %s", r.RemoteAddr, xfor)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, err := w.Write(response)
		if err != nil {
			zap.S().Errorf("error writing response to client: %v", err)
		}
	}
}

// New creates a new VersionControl Server.
func New(log *zap.Logger, config *Config) (peer *Peer, err error) {
	peer = &Peer{
		Log: log,
	}

	// Convert each Service's Version String to List of SemVer
	bootstrapVersions := strings.Split(config.Versions.Bootstrap, ",")
	peer.Versions.Bootstrap, err = version.StrToSemVerList(bootstrapVersions)

	satelliteVersions := strings.Split(config.Versions.Satellite, ",")
	peer.Versions.Satellite, err = version.StrToSemVerList(satelliteVersions)

	storagenodeVersions := strings.Split(config.Versions.Storagenode, ",")
	peer.Versions.Storagenode, err = version.StrToSemVerList(storagenodeVersions)

	uplinkVersions := strings.Split(config.Versions.Uplink, ",")
	peer.Versions.Uplink, err = version.StrToSemVerList(uplinkVersions)

	gatewayVersions := strings.Split(config.Versions.Gateway, ",")
	peer.Versions.Gateway, err = version.StrToSemVerList(gatewayVersions)

	response, err = json.Marshal(peer.Versions)

	if err != nil {
		peer.Log.Sugar().Fatalf("Error marshalling version info: %v", err)
	}

	peer.Log.Sugar().Debugf("setting version info to: %v", string(response))

	peer.Server.Listener, err = net.Listen("tcp", config.Address)
	if err != nil {
		return nil, errs.Combine(err, peer.Close())
	}
	return peer, nil
}

// Run runs versioncontrol server until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		peer.Log.Sugar().Infof("Versioning server started on %s", peer.Addr())
		http.HandleFunc("/", handleGet)
		err = http.Serve(peer.Server.Listener, nil)
		if err != nil {
			peer.Log.Sugar().Error("error occurred starting web server")
		}
		return err
	})
	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() (err error) {
	if peer.Server.Listener != nil {
		err = peer.Server.Listener.Close()
	}
	return
}

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Listener.Addr().String() }
