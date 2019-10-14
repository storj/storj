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

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/version"
)

// SeedLength is the number of bytes in a rollout seed.
const SeedLength = 32

// RolloutErr defines the rollout config error class.
var RolloutErr = errs.Class("rollout config error")

// Config is all the configuration parameters for a Version Control Server.
type Config struct {
	Address  string `user:"true" help:"public address to listen on" default:":8080"`
	Versions ServiceVersions

	Binary Versions
}

// ServiceVersions provides a list of allowed Versions per Service.
type ServiceVersions struct {
	Satellite   string `user:"true" help:"Allowed Satellite Versions" default:"v0.0.1"`
	Storagenode string `user:"true" help:"Allowed Storagenode Versions" default:"v0.0.1"`
	Uplink      string `user:"true" help:"Allowed Uplink Versions" default:"v0.0.1"`
	Gateway     string `user:"true" help:"Allowed Gateway Versions" default:"v0.0.1"`
	Identity    string `user:"true" help:"Allowed Identity Versions" default:"v0.0.1"`
}

// Versions represents versions for all binaries.
// TODO: this name is inconsistent with the internal/version pkg's analogue, `Processes`.
type Versions struct {
	Satellite   Binary
	Storagenode Binary
	Uplink      Binary
	Gateway     Binary
	Identity    Binary
}

// Binary represents versions for single binary.
// TODO: This name is inconsistent with the internal/version pkg's analogue, `Process`.
type Binary struct {
	Minimum   Version
	Suggested Version
	Rollout   Rollout
}

// Version single version.
type Version struct {
	Version string `user:"true" help:"peer version" default:"v0.0.1"`
	URL     string `user:"true" help:"URL for specific binary" default:""`
}

// Rollout represents the state of a version rollout of a binary to the suggested version.
type Rollout struct {
	Seed   string `user:"true" help:"random 32 byte, hex-encoded string"`
	Cursor int    `user:"true" help:"percentage of nodes which should roll-out to the target version" default:"0"`
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
		return nil, err
	}

	peer = &Peer{
		Log: log,
	}

	// Convert each Service's Version String to SemVer
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

	peer.Versions.Processes = version.Processes{}
	peer.Versions.Processes.Satellite, err = configToProcess(config.Binary.Satellite)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer.Versions.Processes.Storagenode, err = configToProcess(config.Binary.Storagenode)
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

func configToProcess(binary Binary) (version.Process, error) {
	process := version.Process{
		Minimum: version.Version{
			Version: binary.Minimum.Version,
			URL:     binary.Minimum.URL,
		},
		Suggested: version.Version{
			Version: binary.Suggested.Version,
			URL:     binary.Suggested.URL,
		},
		Rollout: version.Rollout{},
	}

	seedJSONBytes := []byte("\""+binary.Rollout.Seed+"\"")
	if err := json.Unmarshal(seedJSONBytes, &process.Rollout.Seed); err != nil {
		return version.Process{}, err
	}
	return process, nil
}

// ValidateRollouts validates the rollout field of each field in the Versions struct.
func (versions Versions) ValidateRollouts(log *zap.Logger) error {
	value := reflect.ValueOf(versions)
	fieldCount := value.NumField()
	validationErrs := errs.Group{}
	for i := 1; i < fieldCount; i++ {
		binary, ok := value.Field(i).Interface().(Binary)
		if !ok {
			log.Warn("non-binary field in versions config struct", zap.String("field name", value.Type().Field(i).Name))
			continue
		}
		if err := binary.Rollout.Validate(value.Type().Field(i).Name, log); err != nil {
			validationErrs.Add(err)
		}
	}
	return validationErrs.Err()
}

// Validate validates the rollout seed and cursor config values.
func (rollout Rollout) Validate(binary string, log *zap.Logger) error {
	seedLen := len(rollout.Seed)
	if seedLen == 0 {
		log.Warn("empty seed", zap.String("binary", binary))
		return nil
	}

	if seedLen != hex.EncodedLen(SeedLength) {
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
