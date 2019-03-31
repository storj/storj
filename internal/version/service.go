// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/transport"
)

// Config contains the necessary Information to check the Software Version
type Config struct {
	ServerAddress  string
	RequestTimeout time.Duration
	CheckInterval  time.Duration
}

// Service contains the information and variables to ensure the Software is up to date
type Service struct {
	config *Config
	info   *Info

	Loop *sync2.Cycle

	mu      sync.Mutex
	allowed bool
}

// NewService creates a Version Check Client with default configuration
func NewService(config *Config, info *Info) (client *Service) {
	return &Service{
		config: config,
		info:   info,
		Loop:   sync2.NewCycle(config.CheckInterval),
	}
}

// NewInfo creates a default information configuration
// ToDo: temporary for testing
func NewInfo() (info Info) {
	return Info{
		Version: SemVer{
			Major: 0,
			Minor: 1,
			Patch: 0,
		},
	}
}

// NewVersionedClient returns a transport client which ensures, that the software is up to date
func NewVersionedClient(transport transport.Client, service Service) transport.Client {
	/*if !service.IsUpToDate() {
		zap.S().Fatal("Software Version outdated, please update")
	}*/
	return transport
}

// Run logs the current version information
func (srv *Service) Run(ctx context.Context) error {
	return srv.Loop.Run(ctx, func(ctx context.Context) error {
		srv.mu.Lock()
		defer srv.mu.Unlock()
		var err error
		srv.allowed, err = srv.checkVersion(&ctx)
		if err != nil {
			zap.S().Errorf("Failed to do periodic version check: ", err)
		}
		return err
	})
}

// IsUpToDate returns whether if the Service is allowed to operate or not
func (srv *Service) IsUpToDate() bool {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.allowed
}

// CheckVersion checks if the client is running latest/allowed code
func (srv *Service) checkVersion(ctx *context.Context) (allowed bool, err error) {
	defer mon.Task()(ctx)(&err)
	accepted, err := srv.queryVersionFromControlServer()
	if err != nil {
		return false, err
	}

	zap.S().Debugf("allowed versions from Control Server: %v", accepted)

	// ToDo: Fetch own Service Tag to compare correctly!
	list := accepted.Storagenode
	if list == nil {
		return true, errs.New("Empty List from Versioning Server")
	}
	if containsVersion(list, Build.Version) {
		zap.S().Infof("running on version %s", Build.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("running on not allowed/outdated version %s", Build.Version.String())
		allowed = false
	}
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func (srv *Service) queryVersionFromControlServer() (ver AllowedVersions, err error) {
	client := http.Client{
		Timeout: srv.config.RequestTimeout,
	}
	resp, err := client.Get(srv.config.ServerAddress)
	if err != nil {
		// ToDo: Make sure Control Server is always reachable and refuse startup
		srv.allowed = true
		return AllowedVersions{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return AllowedVersions{}, err
	}
	err = json.Unmarshal(body, &ver)
	return
}

// DebugHandler returns a json representation of the current version information for the binary
func (srv *Service) DebugHandler(w http.ResponseWriter, r *http.Request) {
	j, err := Build.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(j)
	if err != nil {
		zap.S().Errorf("error writing data to client %v", err)
	}
}
