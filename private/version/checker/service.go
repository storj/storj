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

	"storj.io/common/sync2"
	"storj.io/storj/private/version"
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
	Info    version.Info
	service string

	checked         sync2.Fence
	mu              sync.Mutex
	allowed         bool
	acceptedVersion version.SemVer
}

// NewService creates a Version Check Client with default configuration
func NewService(log *zap.Logger, config Config, info version.Info, service string) (client *Service) {
	return &Service{
		log:     log,
		config:  config,
		client:  New(config.ClientConfig),
		Info:    info,
		service: service,
		allowed: true,
	}
}

// CheckProcessVersion is not meant to be used for peers but is meant to be
// used for other utilities
func CheckProcessVersion(ctx context.Context, log *zap.Logger, config Config, info version.Info, service string) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = NewService(log, config, info, service).CheckVersion(ctx)

	return err
}

// IsAllowed returns whether if the Service is allowed to operate or not.
func (service *Service) IsAllowed(ctx context.Context) (version.SemVer, bool) {
	if !service.checked.Wait(ctx) {
		return version.SemVer{}, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.acceptedVersion, service.allowed
}

// CheckVersion checks to make sure the version is still relevant and returns suggested version, returning an error if not.
func (service *Service) CheckVersion(ctx context.Context) (latest version.SemVer, err error) {
	defer mon.Task()(&ctx)(&err)
	latest, allowed := service.checkVersion(ctx)
	if !allowed {
		return latest, fmt.Errorf("outdated software version (%s), please update", service.Info.Version.String())
	}
	return latest, nil
}

// checkVersion checks if the client is running latest/allowed version.
func (service *Service) checkVersion(ctx context.Context) (latestVersion version.SemVer, allowed bool) {
	var err error
	defer mon.Task()(&ctx)(&err)

	var minimum version.SemVer

	defer func() {
		service.mu.Lock()
		service.allowed = allowed
		if err == nil {
			service.acceptedVersion = minimum
		}
		service.mu.Unlock()
		service.checked.Release()
	}()

	allowedVersions, err := service.client.All(ctx)
	if err != nil {
		return service.acceptedVersion, true
	}
	suggestedVersion, err := allowedVersions.Processes.Storagenode.Suggested.SemVer()
	if err != nil {
		return service.acceptedVersion, true
	}

	if !service.Info.Release {
		minimum = service.Info.Version
		return suggestedVersion, true
	}

	minimumOld, err := service.client.OldMinimum(ctx, service.service)
	if err != nil {
		// Log about the error, but dont crash the Service and allow further operation
		service.log.Sugar().Errorf("Failed to do periodic version check: %s", err.Error())
		return suggestedVersion, true
	}

	minimum, err = version.NewSemVer(minimumOld.String())
	if err != nil {
		service.log.Sugar().Errorf("failed to convert old sem version to sem version")
		return suggestedVersion, true
	}

	service.log.Sugar().Debugf("allowed minimum version from control server is: %s", minimum.String())

	if isAcceptedVersion(service.Info.Version, minimumOld) {
		service.log.Sugar().Infof("running on version %s", service.Info.Version.String())
		return suggestedVersion, true
	}

	service.log.Sugar().Errorf("running on not allowed/outdated version %s", service.Info.Version.String())

	return suggestedVersion, false
}

// Checked returns whether the version has been updated.
func (service *Service) Checked() bool {
	return service.checked.Released()
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
		server.log.Sugar().Errorf("error writing data to client: %w", err)
	}
}

// isAcceptedVersion compares and checks if the passed version is greater/equal than the minimum required version
func isAcceptedVersion(test version.SemVer, target version.OldSemVer) bool {
	return test.Major > uint64(target.Major) || (test.Major == uint64(target.Major) && (test.Minor > uint64(target.Minor) || (test.Minor == uint64(target.Minor) && test.Patch >= uint64(target.Patch))))
}
