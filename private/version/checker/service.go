// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/version"
)

// Config contains the necessary Information to check the Software Version.
type Config struct {
	ClientConfig

	CheckInterval time.Duration `help:"Interval to check the version" default:"0h15m0s"`
}

// ErrOutdatedVersion is returned when the software is below the minimum allowed version.
var ErrOutdatedVersion = errs.Class("software outdated")

// Service contains the information and variables to ensure the Software is up to date.
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

// NewService creates a Version Check Client with default configuration.
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
// used for other utilities.
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
		return latest, ErrOutdatedVersion.New("outdated software version (%s), please update", service.Info.Version.String())
	}
	return latest, nil
}

// checkVersion checks if the client is running latest/allowed version.
func (service *Service) checkVersion(ctx context.Context) (_ version.SemVer, allowed bool) {
	var err error
	defer mon.Task()(&ctx)(&err)

	var minimum version.SemVer

	defer func() {
		service.mu.Lock()
		service.allowed = allowed
		if err == nil {
			if minimum.Compare(service.acceptedVersion) >= 0 {
				service.acceptedVersion = minimum
			}
		}
		service.mu.Unlock()
		service.checked.Release()
	}()

	process, err := service.client.Process(ctx, service.service)
	if err != nil {
		service.log.Error("failed to get process version info", zap.Error(err))
		return service.acceptedVersion, true
	}

	suggestedVersion, err := process.Suggested.SemVer()
	if err != nil {
		return service.acceptedVersion, true
	}

	service.mu.Lock()
	isVersionActual := suggestedVersion.Compare(service.acceptedVersion)
	service.mu.Unlock()

	if isVersionActual < 0 {
		minimum = service.Info.Version
		return service.acceptedVersion, true
	}

	if !service.Info.Release {
		minimum = service.Info.Version
		return suggestedVersion, true
	}

	minimum, err = process.Minimum.SemVer()
	if err != nil {
		return suggestedVersion, true
	}

	service.log.Debug("Allowed minimum version from control server.", zap.Stringer("Minimum Version", minimum.Version))

	if service.Info.Version.Compare(minimum) >= 0 {
		service.log.Debug("Running on allowed version.", zap.Stringer("Version", service.Info.Version.Version))
		return suggestedVersion, true
	}
	service.log.Warn("version not allowed/outdated",
		zap.Stringer("current version", service.Info.Version.Version),
		zap.String("minimum allowed version", minimum.String()),
	)
	return suggestedVersion, false
}

// GetCursor returns storagenode rollout cursor value.
func (service *Service) GetCursor(ctx context.Context) (_ version.RolloutBytes, err error) {
	allowedVersions, err := service.client.All(ctx)
	if err != nil {
		return version.RolloutBytes{}, err
	}
	return allowedVersions.Processes.Storagenode.Rollout.Cursor, nil
}

// SetAcceptedVersion changes accepted version to specific for tests.
func (service *Service) SetAcceptedVersion(version version.SemVer) {
	service.mu.Lock()
	defer service.mu.Unlock()

	service.acceptedVersion = version
}

// Checked returns whether the version has been updated.
func (service *Service) Checked() bool {
	return service.checked.Released()
}
