// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/version"
)

// Config is all the configuration parameters for a Version Control Server
type Config struct {
	Address  string `user:"true" help:"public address to listen on" default:":8080"`
	Versions ServiceVersions
}

// ServiceVersions provides a list of allowed Versions per Service
type ServiceVersions struct {
	Bootstrap   string `user:"true" help:"Allowed Bootstrap Versions" default:"v0.0.1"`
	Satellite   string `user:"true" help:"Allowed Satellite Versions" default:"v0.0.1"`
	Storagenode string `user:"true" help:"Allowed Storagenode Versions" default:"v0.0.1"`
	Uplink      string `user:"true" help:"Allowed Uplink Versions" default:"v0.0.1"`
	Gateway     string `user:"true" help:"Allowed Gateway Versions" default:"v0.0.1"`
	Identity    string `user:"true" help:"Allowed Identity Versions" default:"v0.0.1"`
}

// Peer is the representation of a VersionControl Server.
type Peer struct {
	// core dependencies
	Log *zap.Logger

	// Web server
	Server struct {
		Endpoint http.Server
		Listener net.Listener
	}
	Versions version.AllowedVersions

	// response contains the byte version of current allowed versions
	response []byte
}

// HandleGet contains the request handler for the version control web server
func (peer *Peer) HandleGet(w http.ResponseWriter, r *http.Request) {
	// Only handle GET Requests
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var xfor string
	if xfor = r.Header.Get("X-Forwarded-For"); xfor == "" {
		xfor = r.RemoteAddr
	}
	zap.S().Debugf("Request from: %s for %s", r.RemoteAddr, xfor)

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(peer.response)
	if err != nil {
		zap.S().Errorf("error writing response to client: %v", err)
	}
}

// New creates a new VersionControl Server.
func New(log *zap.Logger, config *Config) (peer *Peer, err error) {
	peer = &Peer{
		Log: log,
	}

	// Convert each Service's Version String to SemVer
	peer.Versions.Bootstrap, err = version.NewSemVer(config.Versions.Bootstrap)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Satellite, err = version.NewSemVer(config.Versions.Satellite)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Storagenode, err = version.NewSemVer(config.Versions.Storagenode)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Uplink, err = version.NewSemVer(config.Versions.Uplink)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Gateway, err = version.NewSemVer(config.Versions.Gateway)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Identity, err = version.NewSemVer(config.Versions.Identity)
	if err != nil {
		return &Peer{}, err
	}

	peer.response, err = json.Marshal(peer.Versions)

	if err != nil {
		peer.Log.Sugar().Fatalf("Error marshalling version info: %v", err)
	}

	peer.Log.Sugar().Debugf("setting version info to: %v", string(peer.response))

	mux := http.NewServeMux()
	mux.HandleFunc("/", peer.HandleGet)
	peer.Server.Endpoint = http.Server{
		Handler: mux,
	}

	peer.Server.Listener, err = net.Listen("tcp", config.Address)
	if err != nil {
		return nil, errs.Combine(err, peer.Close())
	}
	return peer, nil
}

// Run runs versioncontrol server until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group

	group.Go(func() error {
		<-ctx.Done()
		return errs2.IgnoreCanceled(peer.Server.Endpoint.Shutdown(ctx))
	})
	group.Go(func() error {
		defer cancel()
		peer.Log.Sugar().Infof("Versioning server started on %s", peer.Addr())
		return errs2.IgnoreCanceled(peer.Server.Endpoint.Serve(peer.Server.Listener))
	})
	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() (err error) {
	return peer.Server.Endpoint.Close()
}

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Listener.Addr().String() }
