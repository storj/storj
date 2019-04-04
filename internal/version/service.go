// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
)

// Config contains the necessary Information to check the Software Version
type Config struct {
	ServerAddress  string        `help:"server address to check its version against" default:"https://version.alpha.storj.io"`
	RequestTimeout time.Duration `help:"Request timeout for version checks" default:"0h1m0s"`
	CheckInterval  time.Duration `help:"Interval to check the version" default:"0h15m0s"`
}

// Service contains the information and variables to ensure the Software is up to date
type Service struct {
	log     *zap.Logger
	config  Config
	service string

	Info Info

	Loop *sync2.Cycle

	mu      sync.Mutex
	allowed bool
}

// NewService creates a Version Check Client with provided configuration and runs one check, returning an error if the version is out of date.
func NewService(ctx context.Context, log *zap.Logger, config Config, info Info, service string) (*Service, error) {
	log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
		info.Version.String(), info.CommitHash, info.Timestamp.String(), info.Release)
	srv := &Service{
		log:     log,
		config:  config,
		service: service,
		Info:    info,
		Loop:    sync2.NewCycle(config.CheckInterval),
		allowed: true,
	}
	err := srv.runOnce(ctx)
	if err != nil {
		return nil, err
	}
	if !srv.IsAllowed() {
		return nil, fmt.Errorf("Outdated software version (%v), please update!", info.Version.String())
	}
	return srv, nil
}

// Run logs the current version information
func (srv *Service) Run(ctx context.Context) error {
	return srv.Loop.Run(ctx, srv.runOnce)
}

func (srv *Service) runOnce(ctx context.Context) (err error) {
	allowed, err := srv.checkVersion(ctx)
	if err != nil {
		// Log about the error, but dont crash the service and allow further operation
		zap.S().Errorf("Failed to do periodic version check: ", err)
		allowed = true
	}

	srv.mu.Lock()
	srv.allowed = allowed
	srv.mu.Unlock()

	return nil
}

// IsAllowed returns whether if the Service is allowed to operate or not
func (srv *Service) IsAllowed() bool {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	return srv.allowed
}

// CheckVersion checks if the client is running latest/allowed code
func (srv *Service) checkVersion(ctx context.Context) (allowed bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if !srv.Info.Release {
		return true, nil
	}

	accepted, err := srv.queryVersionFromControlServer(ctx)
	if err != nil {
		return false, err
	}

	list := getFieldString(&accepted, srv.service)
	zap.S().Debugf("allowed versions from Control Server: %v", list)

	if list == nil {
		return true, errs.New("Empty List from Versioning Server")
	}
	if containsVersion(list, srv.Info.Version) {
		zap.S().Infof("running on version %s", srv.Info.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("running on not allowed/outdated version %s", srv.Info.Version.String())
		allowed = false
	}
	return allowed, err
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func (srv *Service) queryVersionFromControlServer(ctx context.Context) (ver AllowedVersions, err error) {
	// Tune Client to have a custom Timeout (reduces hanging software)
	client := http.Client{
		Timeout: srv.config.RequestTimeout,
	}

	// New Request that used the passed in context
	req, err := http.NewRequest("GET", srv.config.ServerAddress, nil)
	if err != nil {
		return AllowedVersions{}, err
	}
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return AllowedVersions{}, err
	}

	defer func() { _ = resp.Body.Close() }()

	err = json.NewDecoder(resp.Body).Decode(&ver)
	return ver, err
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

func getFieldString(array *AllowedVersions, field string) []SemVer {
	r := reflect.ValueOf(array)
	f := reflect.Indirect(r).FieldByName(field).Interface()
	result, ok := f.([]SemVer)
	if ok {
		return result
	}
	return nil
}
