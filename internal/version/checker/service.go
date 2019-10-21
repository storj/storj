// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/version"
)

// Config contains the necessary Information to check the Software Version
type Config struct {
	ClientConfig

	CheckInterval time.Duration `help:"Interval to check the version" default:"0h15m0s"`
}

// Service contains the information and variables to ensure the Software is up to date
//
// architecture: Service
type Service struct {
	log     *zap.Logger
	config  Config
	client  *Client
	info    version.Info
	service string

	Loop *sync2.Cycle

	checked sync2.Fence
	mu      sync.Mutex
	allowed bool
}

// NewService creates a Version Check Client with default configuration
func NewService(log *zap.Logger, config Config, info version.Info, service string) (client *Service) {
	return &Service{
		log:     log,
		config:  config,
		client:  New(config.ClientConfig),
		info:    info,
		service: service,
		Loop:    sync2.NewCycle(config.CheckInterval),
		allowed: true,
	}
}

// CheckVersion checks to make sure the version is still okay, returning an error if not
func (srv *Service) CheckVersion(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !srv.checkVersion(ctx) {
		return fmt.Errorf("outdated software version (%v), please update", srv.info.Version.String())
	}
	return nil
}

// CheckProcessVersion is not meant to be used for peers but is meant to be
// used for other utilities
func CheckProcessVersion(ctx context.Context, log *zap.Logger, config Config, info version.Info, service string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return NewService(log, config, info, service).CheckVersion(ctx)
}

// Run logs the current version information
func (srv *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !srv.checked.Released() {
		err := srv.CheckVersion(ctx)
		if err != nil {
			return err
		}
	}
	return srv.Loop.Run(ctx, func(ctx context.Context) error {
		srv.checkVersion(ctx)
		return nil
	})
}

// IsAllowed returns whether if the Service is allowed to operate or not
func (srv *Service) IsAllowed() bool {
	srv.checked.Wait()
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.allowed
}

// CheckVersion checks if the client is running latest/allowed code
func (srv *Service) checkVersion(ctx context.Context) (allowed bool) {
	defer mon.Task()(&ctx)(nil)

	defer func() {
		srv.mu.Lock()
		srv.allowed = allowed
		srv.mu.Unlock()
		srv.checked.Release()
	}()

	if !srv.info.Release {
		return true
	}

	minimum, err := srv.client.OldMinimum(ctx, srv.service)
	if err != nil {
		// Log about the error, but dont crash the service and allow further operation
		srv.log.Sugar().Errorf("Failed to do periodic version check: %s", err.Error())
		return true
	}

	srv.log.Sugar().Debugf("allowed minimum version from control server is: %s", minimum.String())

	if minimum.String() == "" {
		srv.log.Sugar().Errorf("no version from control server, accepting to run")
		return true
	}
	if isAcceptedVersion(srv.info.Version, minimum) {
		srv.log.Sugar().Infof("running on version %s", srv.info.Version.String())
		return true
	}
	srv.log.Sugar().Errorf("running on not allowed/outdated version %s", srv.info.Version.String())
	return false
}

// DebugHandler implements version info endpoint.
type DebugHandler struct {
	log *zap.Logger
}

// NewDebugHandler returns new debug handler.
func NewDebugHandler(log *zap.Logger) *DebugHandler {
	return &DebugHandler{log}
}

// ServeHTTP returns a json representation of the current version information for the binary.
func (server *DebugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	j, err := version.Build.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(append(j, '\n'))
	if err != nil {
		server.log.Sugar().Errorf("error writing data to client %v", err)
	}
}

// isAcceptedVersion compares and checks if the passed version is greater/equal than the minimum required version
func isAcceptedVersion(test version.SemVer, target version.SemVer) bool {
	return test.Major > target.Major || (test.Major == target.Major && (test.Minor > target.Minor || (test.Minor == target.Minor && test.Patch >= target.Patch)))
}
