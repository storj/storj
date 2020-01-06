// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"reflect"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/private/version"
)

// seedLength is the number of bytes in a rollout seed.
const seedLength = 32

var (
	// RolloutErr defines the rollout config error class.
	RolloutErr = errs.Class("rollout config error")
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
// NB: this will be deprecated in favor of `ProcessesConfig`.
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

// HandleGet contains the request handler for the version control web server.
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
	peer.Log.Sugar().Debugf("Request from: %s for %s", r.RemoteAddr, xfor)

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(peer.response)
	if err != nil {
		peer.Log.Sugar().Errorf("error writing response to client: %v", err)
	}
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

	peer.Versions.Processes = version.Processes{}
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
			if err == EmptySeedErr {
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
