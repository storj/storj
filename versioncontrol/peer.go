// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"regexp"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/version"
)

// Config is all the configuration parameters for a Version Control Server
type Config struct {
	Address        string `user:"true" help:"public address to listen on" default:":7777"`
	AllowedVersion []string
}

// Peer is the representation of a VersionControl Server.
type Peer struct {
	// core dependencies
	Log *zap.Logger

	// Web server
	Server struct {
		Listener net.Listener
		Handler  http.HandlerFunc
	}
}

var (
	logfile  = "/var/log/storj/version.log"
	ver      []version.Info
	response []byte
)

func handleGet(w http.ResponseWriter, r *http.Request) {
	var xfor string

	// Only handle GET Requests
	if r.Method == "GET" {
		if xfor = r.Header.Get("X-Forwarded-For"); xfor == "" {
			xfor = r.RemoteAddr
		}
		log.Printf("Request from: %s for %s", r.RemoteAddr, xfor)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, err := w.Write(response)
		if err != nil {
			log.Printf("error writing response to client: %v", err)
		}
	}
}

// New creates a new VersionControl Server.
func New(log *zap.Logger, config Config) (peer *Peer, err error) {
	peer = &Peer{
		Log: log,
	}

	versionRegex := regexp.MustCompile("^" + version.SemVerRegex + "$")

	for _, subVersion := range config.AllowedVersion {
		sVer, err := version.NewSemVer(versionRegex, subVersion)
		if err != nil {
			log.Sugar().Fatalf("Error parsing version %s", subVersion)
		}
		instance := version.Info{
			Version: *sVer,
		}
		ver = append(ver, instance)
	}

	response, err = json.Marshal(ver)
	if err != nil {
		log.Sugar().Fatalf("Error marshalling version info: %v", err)
	}

	log.Sugar().Debugf("setting version info to: %v", ver)

	peer.Server.Listener, err = net.Listen("tcp", config.Address)
	if err != nil {
		return nil, errs.Combine(err, peer.Close())
	}
	return
}

// Run runs bootstrap node until it's either closed or it errors.
func (peer *Peer) Run() (err error) {

	peer.Log.Sugar().Infof("Public server started on %s", peer.Addr())

	http.HandleFunc("/", handleGet)
	err = http.Serve(peer.Server.Listener, nil)
	if err != nil {
		peer.Log.Sugar().Error("error occurred starting web server")
	}
	return
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
