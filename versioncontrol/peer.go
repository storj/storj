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
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/sync2"
	"storj.io/common/version"
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
	Address       string        `user:"true" help:"public address to listen on" default:":8080" testDefault:"$HOST:0"`
	SafeRate      float64       `user:"true" help:"the safe daily fractional increase for a rollout (a value of .5 means 0 to 50% in 24 hours). 0 means immediate rollout." default:".2"`
	RegenInterval time.Duration `user:"true" help:"how long to go between recalculating the current cursors. 0 means on demand." default:"5m"`

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
	Seed           string `user:"true" help:"random 32 byte, hex-encoded string" default:"" testDefault:"000102030405060708090a0b0c0d0e0ff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff"`
	PreviousCursor int    `user:"true" help:"prior configuration's cursor value. if 100%, will be capped at the current cursor." default:"100"`
	Cursor         int    `user:"true" help:"percentage of nodes which should roll-out to the suggested version" default:"0"`
}

// response invariant: the struct or its data is never modified after creation.
type response struct {
	versions version.AllowedVersions
	// serialized contains the byte version of current allowed versions.
	serialized []byte
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

	config   Config
	initTime time.Time

	regenLoop *sync2.Cycle

	mu       sync.Mutex
	response *response
}

// New creates a new VersionControl Server.
func New(log *zap.Logger, config *Config) (peer *Peer, err error) {
	if err := config.Binary.ValidateRollouts(log); err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	peer = &Peer{
		Log:       log,
		config:    *config,
		initTime:  time.Now(),
		regenLoop: sync2.NewCycle(config.RegenInterval),
	}

	err = peer.updateResponse()
	if err != nil {
		return nil, err
	}

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

func (peer *Peer) getResponse() *response {
	if peer.config.RegenInterval <= 0 && peer.config.SafeRate > 0 {
		// generate on demand.
		if err := peer.updateResponse(); err != nil {
			peer.Log.Error("Error updating config.", zap.Error(err))
		}
	}

	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.response
}

func (peer *Peer) updateResponse() (err error) {
	response, err := peer.config.generateResponse(peer.initTime)
	if err != nil {
		peer.Log.Error("Error updating response.", zap.Error(err))
		return err
	}

	peer.Log.Debug("Setting version info.", zap.ByteString("Value", response.serialized))
	peer.mu.Lock()
	defer peer.mu.Unlock()
	peer.response = response
	return nil
}

func (config *Config) generateResponse(initTime time.Time) (rv *response, err error) {
	rv = &response{}

	rv.versions.Processes.Satellite, err = config.configToProcess(initTime, config.Binary.Satellite)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	rv.versions.Processes.Storagenode, err = config.configToProcess(initTime, config.Binary.Storagenode)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	rv.versions.Processes.StoragenodeUpdater, err = config.configToProcess(initTime, config.Binary.StoragenodeUpdater)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	rv.versions.Processes.Uplink, err = config.configToProcess(initTime, config.Binary.Uplink)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	rv.versions.Processes.Gateway, err = config.configToProcess(initTime, config.Binary.Gateway)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	rv.versions.Processes.Identity, err = config.configToProcess(initTime, config.Binary.Identity)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	rv.serialized, err = json.Marshal(rv.versions)
	if err != nil {
		return nil, RolloutErr.Wrap(err)
	}

	return rv, nil
}

// versionHandle handles all process versions request.
func (peer *Peer) versionHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, err := w.Write(peer.getResponse().serialized)
	if err != nil {
		peer.Log.Error("Error writing response to client.", zap.Error(err))
	}
}

// processURLHandle handles process binary url resolving.
func (peer *Peer) processURLHandle(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	service := params["service"]
	versionType := params["version"]

	response := peer.getResponse()

	var process version.Process
	switch service {
	case "satellite":
		process = response.versions.Processes.Satellite
	case "storagenode":
		process = response.versions.Processes.Storagenode
	case "storagenode-updater":
		process = response.versions.Processes.StoragenodeUpdater
	case "uplink":
		process = response.versions.Processes.Uplink
	case "gateway":
		process = response.versions.Processes.Gateway
	case "identity":
		process = response.versions.Processes.Identity
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
	if peer.config.RegenInterval > 0 {
		group.Go(func() error {
			defer cancel()
			return peer.regenLoop.Run(ctx, func(ctx context.Context) error {
				return peer.updateResponse()
			})
		})
	}
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
	if rollout.PreviousCursor < 0 || rollout.PreviousCursor > 100 {
		return RolloutErr.New("invalid previous cursor percentage: %d", rollout.PreviousCursor)
	}

	if _, err := hex.DecodeString(rollout.Seed); err != nil {
		return RolloutErr.New("invalid seed: %q", rollout.Seed)
	}
	return nil
}

func (config *Config) configToProcess(initTime time.Time, binary ProcessConfig) (version.Process, error) {
	currentPercent := calculateRolloutCursor(initTime, binary, config.SafeRate)

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
			Cursor: version.PercentageToCursorF(currentPercent),
		},
	}

	seedBytes, err := hex.DecodeString(binary.Rollout.Seed)
	if err != nil {
		return version.Process{}, err
	}
	copy(process.Rollout.Seed[:], seedBytes)
	return process, nil
}

func calculateRolloutCursor(initTime time.Time, binary ProcessConfig, safeRate float64) float64 {
	targetPercent := float64(binary.Rollout.Cursor)
	previousPercent := float64(binary.Rollout.PreviousCursor)
	if previousPercent > targetPercent {
		previousPercent = targetPercent
	}
	elapsed := time.Since(initTime)
	currentPercent := targetPercent

	safePercentPerDay := safeRate * 100
	if safePercentPerDay > 0 {
		// first calculate targetTime:
		targetTimeInDaysFromNow := (targetPercent - previousPercent) / safePercentPerDay
		targetTime := time.Duration(targetTimeInDaysFromNow * 24 * float64(time.Hour))

		if targetTime > 0 {
			// now calculate the current percent based on how close targetTime is.
			currentPercent = clampedLinearInterp(float64(elapsed)/float64(targetTime), previousPercent, targetPercent)
		}
	}

	return currentPercent
}

func clampedLinearInterp(frac, low, high float64) float64 {
	v := (high-low)*frac + low
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
