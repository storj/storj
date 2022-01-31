// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/private/version"
)

// seedLength is the number of bytes in a rollout seed.
const seedLength = 32

var (
	// RolloutErr defines the rollout config error class.
	RolloutErr = errs.Class("rollout config")
	// EmptySeedErr is used when the rollout contains an empty seed value.
	EmptySeedErr = RolloutErr.New("empty seed")
)

// Config is all the configuration parameters for a Version Control Server.
type Config struct {
	Address  string `user:"true" help:"public address to listen on" default:":8080"`
	Versions OldVersionConfig

	Binary ProcessesConfig
}

// OldVersionConfig provides a list of allowed Versions per process.
//
// NB: use `ProcessesConfig` for newer code instead.
type OldVersionConfig struct {
	Satellite   string `user:"true" help:"Allowed Satellite Versions" default:"v0.0.1"`
	Storagenode string `user:"true" help:"Allowed Storagenode Versions" default:"v0.0.1"`
	Uplink      string `user:"true" help:"Allowed Uplink Versions" default:"v0.0.1"`
	Gateway     string `user:"true" help:"Allowed Gateway Versions" default:"v0.0.1"`
	Identity    string `user:"true" help:"Allowed Identity Versions" default:"v0.0.1"`
}

// ProcessesConfig represents versions configuration for all processes.
type ProcessesConfig struct {
	Satellite          ProcessConfig
	Storagenode        ProcessConfig
	StoragenodeUpdater ProcessConfig
	Uplink             ProcessConfig
	Gateway            ProcessConfig
	Identity           ProcessConfig
}

// ProcessConfig represents versions configuration for a single process.
type ProcessConfig struct {
	Minimum   VersionConfig
	Suggested VersionConfig
	Rollout   RolloutConfig
}

// VersionConfig single version configuration.
type VersionConfig struct {
	Version string `user:"true" help:"peer version" default:"v0.0.1"`
	URL     string `user:"true" help:"URL for specific binary" default:""`
}

// RolloutConfig represents the state of a version rollout configuration of a process.
type RolloutConfig struct {
	Seed   string `user:"true" help:"random 32 byte, hex-encoded string"`
	Cursor int    `user:"true" help:"percentage of nodes which should roll-out to the suggested version" default:"0"`
}

// Peer is the representation of a VersionControl Server.
//
// architecture: Peer
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

// New creates a new VersionControl Server.
func New(log *zap.Logger, config *Config) (peer *Peer, err error) {
	if err := config.Binary.ValidateRollouts(log); err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer = &Peer{
		Log: log,
	}

	// Convert each Service's VersionConfig String to SemVer
	peer.Versions.Satellite, err = version.NewOldSemVer(config.Versions.Satellite)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Storagenode, err = version.NewOldSemVer(config.Versions.Storagenode)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Uplink, err = version.NewOldSemVer(config.Versions.Uplink)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Gateway, err = version.NewOldSemVer(config.Versions.Gateway)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Identity, err = version.NewOldSemVer(config.Versions.Identity)
	if err != nil {
		return &Peer{}, err
	}

	peer.Versions.Processes.Satellite, err = configToProcess(config.Binary.Satellite)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.Versions.Processes.Storagenode, err = configToProcess(config.Binary.Storagenode)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.Versions.Processes.StoragenodeUpdater, err = configToProcess(config.Binary.StoragenodeUpdater)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.Versions.Processes.Uplink, err = configToProcess(config.Binary.Uplink)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.Versions.Processes.Gateway, err = configToProcess(config.Binary.Gateway)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.Versions.Processes.Identity, err = configToProcess(config.Binary.Identity)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.response, err = json.Marshal(peer.Versions)
	if err != nil {
		peer.Log.Error("Error marshalling version info.", zap.Error(err))
		return nil, RolloutErr.Wrap(err)
	}

	peer.Log.Debug("Setting version info.", zap.ByteString("Value", peer.response))

	{
		router := mux.NewRouter()
		router.HandleFunc("/", peer.versionHandle).Methods(http.MethodGet)
		router.HandleFunc("/processes/{service}/{version}/url", peer.processURLHandle).Methods(http.MethodGet)

		peer.Server.Endpoint = http.Server{
			Handler: router,
		}

		peer.Server.Listener, err = net.Listen("tcp", config.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	return peer, nil
}

// versionHandle handles all process versions request.
func (peer *Peer) versionHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, err := w.Write(peer.response)
	if err != nil {
		peer.Log.Error("Error writing response to client.", zap.Error(err))
	}
}

// processURLHandle handles process binary url resolving.
func (peer *Peer) processURLHandle(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	service := params["service"]
	versionType := params["version"]

	var process version.Process
	switch service {
	case "satellite":
		process = peer.Versions.Processes.Satellite
	case "storagenode":
		process = peer.Versions.Processes.Storagenode
	case "storagenode-updater":
		process = peer.Versions.Processes.StoragenodeUpdater
	case "uplink":
		process = peer.Versions.Processes.Uplink
	case "gateway":
		process = peer.Versions.Processes.Gateway
	case "identity":
		process = peer.Versions.Processes.Identity
	default:
		http.Error(w, "service does not exists", http.StatusNotFound)
		return
	}

	var url string
	switch versionType {
	case "minimum":
		url = process.Minimum.URL
	case "suggested":
		url = process.Suggested.URL
	default:
		http.Error(w, "invalid version, should be minimum or suggested", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()

	os := query.Get("os")
	if os == "" {
		http.Error(w, "goos is not specified", http.StatusBadRequest)
		return
	}

	arch := query.Get("arch")
	if arch == "" {
		http.Error(w, "goarch is not specified", http.StatusBadRequest)
		return
	}

	if scheme, ok := isBinarySupported(service, os, arch); !ok {
		http.Error(w, fmt.Sprintf("binary scheme %s is not supported", scheme), http.StatusNotFound)
		return
	}

	url = strings.Replace(url, "{os}", os, 1)
	url = strings.Replace(url, "{arch}", arch, 1)

	w.Header().Set("Content-Type", "text/plain")
	_, err := w.Write([]byte(url))
	if err != nil {
		peer.Log.Error("Error writing response to client.", zap.Error(err))
	}
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
		peer.Log.Info("Versioning server started.", zap.String("Address", peer.Addr()))
		err := peer.Server.Endpoint.Serve(peer.Server.Listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
	})
	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() (err error) {
	return peer.Server.Endpoint.Close()
}

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Listener.Addr().String() }

// ValidateRollouts validates the rollout field of each field in the Versions struct.
func (versions ProcessesConfig) ValidateRollouts(log *zap.Logger) error {
	value := reflect.ValueOf(versions)
	fieldCount := value.NumField()
	validationErrs := errs.Group{}
	for i := 0; i < fieldCount; i++ {
		binary, ok := value.Field(i).Interface().(ProcessConfig)
		if !ok {
			log.Warn("non-binary field in versions config struct", zap.String("field name", value.Type().Field(i).Name))
			continue
		}
		if err := binary.Rollout.Validate(); err != nil {
			if errors.Is(err, EmptySeedErr) {
				log.Warn(err.Error(), zap.String("binary", value.Type().Field(i).Name))
				continue
			}
			validationErrs.Add(err)
		}
	}
	return validationErrs.Err()
}

// Validate validates the rollout seed and cursor config values.
func (rollout RolloutConfig) Validate() error {
	seedLen := len(rollout.Seed)
	if seedLen == 0 {
		return EmptySeedErr
	}

	if seedLen != hex.EncodedLen(seedLength) {
		return RolloutErr.New("invalid seed length: %d", seedLen)
	}

	if rollout.Cursor < 0 || rollout.Cursor > 100 {
		return RolloutErr.New("invalid cursor percentage: %d", rollout.Cursor)
	}

	if _, err := hex.DecodeString(rollout.Seed); err != nil {
		return RolloutErr.New("invalid seed: %s", rollout.Seed)
	}
	return nil
}

func configToProcess(binary ProcessConfig) (version.Process, error) {
	process := version.Process{
		Minimum: version.Version{
			Version: binary.Minimum.Version,
			URL:     binary.Minimum.URL,
		},
		Suggested: version.Version{
			Version: binary.Suggested.Version,
			URL:     binary.Suggested.URL,
		},
		Rollout: version.Rollout{
			Cursor: version.PercentageToCursor(binary.Rollout.Cursor),
		},
	}

	seedBytes, err := hex.DecodeString(binary.Rollout.Seed)
	if err != nil {
		return version.Process{}, err
	}
	copy(process.Rollout.Seed[:], seedBytes)
	return process, nil
}
